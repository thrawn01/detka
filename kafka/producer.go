package kafka

import (
	"encoding/json"

	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	"github.com/thrawn01/detka/models"
	"golang.org/x/net/context"
)

type Producer interface {
	Send(models.QueueMessage) error
}

// Producer Implementation
type KafkaProducer struct {
	producer sarama.SyncProducer
	topic    string
	ctx      *ProducerManager
}

func NewProducer(ctx *ProducerManager, topic string, producer sarama.SyncProducer) Producer {
	return &KafkaProducer{
		producer: producer,
		topic:    topic,
		ctx:      ctx,
	}
}

func (self *KafkaProducer) Send(msg models.QueueMessage) error {

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, _, err = self.producer.SendMessage(&sarama.ProducerMessage{
		Topic: self.topic,
		Value: sarama.ByteEncoder(payload),
	})

	if err != nil {
		if err == sarama.ErrBrokerNotAvailable || err == sarama.ErrClosedClient {
			// Signal We should reconnect
			self.ctx.Signal()
		}
		return err
	}
	return nil
}

func (self *KafkaProducer) Get(payload []byte) error {
	_, _, err := self.producer.SendMessage(&sarama.ProducerMessage{
		Topic: self.topic,
		Value: sarama.ByteEncoder(payload),
	})

	if err != nil {
		if err == sarama.ErrBrokerNotAvailable || err == sarama.ErrClosedClient {
			// Signal We should reconnect
			self.ctx.Signal()
		}
		return err
	}
	return nil
}

// Nil Implementation only returns errors
type NilProducer struct{}

func (self *NilProducer) Send(msg models.QueueMessage) error {
	return errors.New("Not Connected")
}

// Returns the Kafka interface from our context
func GetProducer(ctx context.Context) Producer {
	return GetProducerManager(ctx).GetProducer()
}
