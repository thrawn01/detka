package kafka

import (
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/Sirupsen/logrus"
	"github.com/thrawn01/args"
)

type ConsumerManager struct {
	parser    *args.ArgParser
	done      chan struct{}
	new       chan sarama.PartitionConsumer
	reconnect chan bool
}

func NewConsumerManager(parser *args.ArgParser) *ConsumerManager {
	manager := &ConsumerManager{
		parser: parser,
	}
	manager.Start()
	return manager
}

func (self *ConsumerManager) connect() (sarama.PartitionConsumer, error) {
	opts := self.parser.GetOpts()
	consumer, err := sarama.NewConsumer(opts.StringSlice("kafka-endpoints"), nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"type":   "kafka",
			"method": "NewConsumer()",
		}).Error("Failed with - ", err.Error())
		return nil, err
	}
	partition, err := consumer.ConsumePartition(opts.String("kafka-topic"), 0, sarama.OffsetNewest)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"type":   "kafka",
			"method": "ConsumePartition()",
		}).Error("Failed with - ", err.Error())
		return nil, err
	}
	return partition, nil
}

func (self *ConsumerManager) GetConsumerChannel() chan sarama.PartitionConsumer {
	return self.new
}

func (self *ConsumerManager) GetConsumer() sarama.PartitionConsumer {
	return <-self.new
}

func (self *ConsumerManager) Start() {
	self.new = make(chan sarama.PartitionConsumer)
	self.reconnect = make(chan bool)
	self.done = make(chan struct{})

	var attemptedConnect sync.WaitGroup

	go func() {
		defer close(self.new)
		var once sync.Once
		attemptedConnect.Add(1)

		for {
			var timer <-chan time.Time

			select {
			case <-self.reconnect:
			case <-timer:
			case <-self.done:
				return
			}

			consumer, err := self.connect()
			if err != nil {
				timer = time.NewTimer(time.Second).C
			}
			once.Do(func() { attemptedConnect.Done() })
			if consumer != nil {
				self.new <- consumer
			}
		}
	}()

	// Send the first connect signal
	self.SignalReconnect()
	// Attempt to connect at least once before we leave Start()
	attemptedConnect.Wait()
}

func (self *ConsumerManager) SignalReconnect() {
	self.reconnect <- true
}

func (self *ConsumerManager) Stop() {
	close(self.done)
}
