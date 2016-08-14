package detka

import (
	"fmt"

	"encoding/json"

	"time"

	"github.com/Shopify/sarama"
	"github.com/Sirupsen/logrus"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/models"
	"github.com/thrawn01/detka/store"
)

type Worker struct {
	mailer   Mailer
	consumer *kafka.ConsumerManager
	store    store.Store
	done     chan struct{}
}

func NewWorker(cm *kafka.ConsumerManager, store store.Store, mailer Mailer) *Worker {
	worker := &Worker{
		mailer:   mailer,
		store:    store,
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
		err := self.store.UpdateMessage(id, map[string]interface{}{
			"Status": status,
		})
		if err == nil {
			return
		}

		// Special logging, this needs investigation
		if store.IsNotFound(err) {
			logrus.WithFields(logrus.Fields{
				"method": "Worker.updateStatus",
				"type":   "store",
				"result": "not-found",
			}).Error(err.Error())
			return
		}

		logrus.WithFields(logrus.Fields{
			"method": "Worker.updateStatus",
			"type":   "store",
			"result": "retry",
		}).Error(err.Error())

		if store.IsConnectError(err) {
			self.store.SignalReconnect()
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

func (self *Worker) handleMessage(payload *sarama.ConsumerMessage) {
	logrus.Debugf("Got new message -> %s", payload.Value)

	var msg models.QueueMessage
	if err := json.Unmarshal(payload.Value, &msg); err != nil {
		logrus.WithFields(logrus.Fields{
			"method": "Worker.handleMessage()",
			"type":   "json",
			"result": "discarded",
		}).Error(fmt.Sprintf("Unmarshal failed on payload - %s", string(payload.Value)))
		return
	}

	// API is just testing the connection
	if msg.Type == "ping" {
		return
	}

	// Get the message from the database
	email, err := self.store.GetMessage(msg.Id)
	if err != nil {
		if store.IsNotFound(err) {
			logrus.WithFields(logrus.Fields{
				"method": "Worker.handleMessage()",
				"type":   "store",
				"result": "discarded",
			}).Error(fmt.Sprintf("Queue Message Id not found - %s", msg.Id))
		}
	}

	if err := self.mailer.Send(email); err != nil {
		self.updateStatus(msg.Id, "UN-DELIVERABLE")
		return
	}
	self.updateStatus(msg.Id, "DELIVERED")
}
