package kafka

import (
	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type Producer interface {
	Send([]byte) error
}

// Producer Implementation
type KafkaProducer struct {
	producer sarama.SyncProducer
	topic    string
	ctx      *Context
}

func NewProducer(ctx *Context, topic string, producer sarama.SyncProducer) Producer {
	return &KafkaProducer{
		producer: producer,
		topic:    topic,
		ctx:      ctx,
	}
}

func (self *KafkaProducer) Send(payload []byte) error {
	_, _, err := self.producer.SendMessage(&sarama.ProducerMessage{
		Topic: self.topic,
		Value: sarama.ByteEncoder(payload),
	})

	if err != nil {
		if err == sarama.ErrBrokerNotAvailable || err == sarama.ErrClosedClient {
			// Signal We should reconnect
			self.ctx.SignalReconnect()
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
			self.ctx.SignalReconnect()
		}
		return err
	}
	return nil
}

// Nil Implementation only returns errors
type NilProducer struct{}

func (self *NilProducer) Send(payload []byte) error {
	return errors.New("Not Connected")
}

// Returns the Kafka interface from our context
func GetProducer(ctx context.Context) Producer {
	return GetContext(ctx).Get()
}
