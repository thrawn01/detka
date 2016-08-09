package kafka

import (
	"net/http"

	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	"github.com/pressly/chi"
	"golang.org/x/net/context"
)

type contextKey int

const (
	kafkaTopic      string     = "message-topic"
	kafkaContextKey contextKey = 1
)

type Kafka interface {
	Send([]byte) error
}

// Implementation
type KafkaImpl struct {
	producer sarama.SyncProducer
	ctx      *Context
}

func NewKafa(ctx *Context, producer sarama.SyncProducer) Kafka {
	return &KafkaImpl{
		producer: producer,
		ctx:      ctx,
	}
}

func (self *KafkaImpl) Send(payload []byte) error {
	_, _, err := self.producer.SendMessage(&sarama.ProducerMessage{
		Topic: kafkaTopic,
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
type KafkaNil struct{}

func (self *KafkaNil) Send(payload []byte) error {
	return errors.New("Kafka: Not Connected")
}

// Returns the Kafka interface from our context
func GetKafka(ctx context.Context) Kafka {
	return GetContext(ctx).Get()
}

// Injects kafka.Context into the context.Context for each request
func Middleware(kafkaContext *Context) func(chi.Handler) chi.Handler {
	return func(next chi.Handler) chi.Handler {
		return chi.HandlerFunc(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			ctx = SetContext(ctx, kafkaContext)
			next.ServeHTTPC(ctx, resp, req)
		})
	}
}
