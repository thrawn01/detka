package kafka

import (
	"time"

	"github.com/Shopify/sarama"
	"github.com/Sirupsen/logrus"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka/connection"
)

type ConsumerManager struct {
	*connection.Manager
	parser       *args.ArgParser
	consumerChan chan sarama.PartitionConsumer
	connected    bool
}

func NewConsumerManager(parser *args.ArgParser) *ConsumerManager {
	manager := &ConsumerManager{
		&connection.Manager{},
		parser,
		make(chan sarama.PartitionConsumer, 1),
		false,
	}
	manager.Start()
	return manager
}

func (self *ConsumerManager) connect() bool {
	opts := self.parser.GetOpts()
	logrus.Info("Connecting to Kafka Cluster ", opts.StringSlice("kafka-endpoints"))
	consumer, err := sarama.NewConsumer(opts.StringSlice("kafka-endpoints"), nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"type":   "kafka",
			"method": "NewConsumer()",
		}).Error("Failed with - ", err.Error())
		self.setConnected(false)
		return false
	}
	partition, err := consumer.ConsumePartition(opts.String("kafka-topic"), 0, sarama.OffsetNewest)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"type":   "kafka",
			"method": "ConsumePartition()",
		}).Error("Failed with - ", err.Error())
		self.setConnected(false)
		return false
	}
	if partition != nil {
		self.setConnected(true)
		self.consumerChan <- partition
	}
	return true
}

func (self *ConsumerManager) GetConsumerChannel() chan sarama.PartitionConsumer {
	return self.consumerChan
}

func (self *ConsumerManager) Start() {
	// run connect(), if it fails try again every 2 seconds
	self.Begin(self.connect, time.Second*2)
}

func (self *ConsumerManager) Stop() {
	self.End()
}

func (self *ConsumerManager) IsConnected() (result bool) {
	self.WithLock(func() {
		result = self.connected
	})
	return
}
func (self *ConsumerManager) setConnected(set bool) {
	self.WithLock(func() {
		self.connected = set
	})
}
