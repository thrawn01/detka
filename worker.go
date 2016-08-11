package detka

import (
	"fmt"

	"encoding/json"

	"time"

	"github.com/Shopify/sarama"
	"github.com/Sirupsen/logrus"
	"github.com/dancannon/gorethink"
	"github.com/pkg/errors"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/rethink"
)

type Worker struct {
	mailer   Mailer
	rethink  *rethink.Manager
	consumer *kafka.ConsumerManager
	done     chan struct{}
}

func NewWorker(cm *kafka.ConsumerManager, rm *rethink.Manager, mailer Mailer) *Worker {
	worker := &Worker{
		mailer:   mailer,
		rethink:  rm,
		consumer: cm,
	}
	worker.Start()
	return worker
}

func (self *Worker) Start() {
	consumerChan := self.consumer.GetConsumerChannel()
	self.done = make(chan struct{})

	go func() {
		var messages <-chan *sarama.ConsumerMessage
		var errors <-chan *sarama.ConsumerError

		for {
			select {
			case consumer := <-consumerChan:
				// Consumer might be closed before self.done, if this is the
				if consumer != nil {
					messages = consumer.Messages()
					errors = consumer.Errors()
				}
			case msg := <-messages:
				self.handleMessage(msg)
			case err := <-errors:
				logrus.WithFields(logrus.Fields{
					"type":   "kafka",
					"method": "Start()",
				}).Error("Received Error - ", err.Error())
				self.consumer.SignalReconnect()
			case <-self.done:
				return
			}
		}
	}()
}

func (self *Worker) Stop() {
	close(self.done)
}

func (self *Worker) updateStatus(id, status string) {
	for {
		// Update the database
		changed, err := gorethink.Table("messages").Get(id).Update(map[string]interface{}{
			"Status": status,
		}).RunWrite(self.rethink.GetSession(), rethink.RunOpts)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"method": "Worker.updateStatus",
				"type":   "rethink",
			}).Error(err.Error())
			// TODO: Verify the error was a result of a not connected error before signaling for reconnect
			self.rethink.SignalReconnect()
		} else if changed.Skipped != 0 {
			logrus.WithFields(logrus.Fields{
				"method": "Worker.updateStatus",
				"type":   "rethink",
				"result": "skipped",
			}).Error(fmt.Sprintf("Message ID: %s does not exist in database", id))
			return
		} else {
			return
		}

		// Sleep for 1 second before retrying
		timer := time.NewTimer(time.Second).C
		select {
		case <-timer:
		case <-self.done:
			return
		}
	}
}

func (self *Worker) saveMessage(msg Message) {
	for {
		// Return if the insert was a success
		err := InsertMessage(self.rethink.GetSession(), &msg)
		if err == nil {
			return
		}
		// If not a success continue to try saving
		logrus.WithFields(logrus.Fields{
			"method": "Worker.saveMessage",
			"type":   "rethink",
		}).Error(fmt.Sprintf("Failed to save message - '%s' - %+v", err.Error(), msg))

		// TODO: Is the error because we can't connect to the database or some other error?
		self.rethink.SignalReconnect()

		// Sleep for 1 second before retrying
		timer := time.NewTimer(time.Second).C
		select {
		case <-timer:
		case <-self.done:
			return
		}
	}
}

func (self *Worker) handleMessage(payload *sarama.ConsumerMessage) {
	logrus.Debugf("Got new message -> %s", payload.Value)

	var msg Message
	if err := json.Unmarshal(payload.Value, &msg); err != nil {
		logrus.WithFields(logrus.Fields{
			"method": "Worker.handleMessage",
			"type":   "json",
			"result": "discarded",
		}).Error(fmt.Sprintf("Unmarshal failed on payload - %s", string(payload.Value)))
		return
	}

	// API is just testing the connection
	if msg.Type == "ping" {
		return
	}

	// Save the message to the database before sending
	msg.Status = "SENDING"
	self.saveMessage(msg)

	if err := self.mailer.Send(msg); err != nil {
		self.updateStatus(msg.Id, "UN-DELIVERABLE")
		return
	}
	self.updateStatus(msg.Id, "DELIVERED")
}

// TODO: interaction with the db should really be through a repository
func InsertMessage(session *gorethink.Session, msg *Message) error {
	if len(msg.Id) == 0 {
		msg.Id = NewId()
	}
	changed, err := gorethink.Table("messages").Insert(msg).RunWrite(session, rethink.RunOpts)
	if err != nil {
		return errors.Wrap(err, "Insert")
	} else if changed.Errors != 0 {
		return errors.New(fmt.Sprintf("Errors on Insert - %s", changed.FirstError))
	}
	return nil
}
