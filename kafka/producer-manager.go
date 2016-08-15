package kafka

import (
	"net/http"
	"time"

	"github.com/Shopify/sarama"
	"github.com/Sirupsen/logrus"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka/connection"
	"golang.org/x/net/context"
)

type contextKey int

const (
	producerManagerKey contextKey = 1
)

func SetProducerManager(ctx context.Context, manager *ProducerManager) context.Context {
	return context.WithValue(ctx, producerManagerKey, manager)
}

func GetProducerManager(ctx context.Context) *ProducerManager {
	obj, ok := ctx.Value(producerManagerKey).(*ProducerManager)
	if !ok {
		panic("No kafka.ProducerManager found in context")
	}
	return obj
}

type ProducerManager struct {
	*connection.Manager
	parser   *args.ArgParser
	producer Producer
}

func NewProducerManager(parser *args.ArgParser) *ProducerManager {
	manager := &ProducerManager{
		&connection.Manager{},
		parser,
		nil,
	}
	manager.Start()
	return manager
}

func (self *ProducerManager) GetProducer() (result Producer) {
	self.WithLock(func() {
		result = self.producer
	})
	return
}

func (self *ProducerManager) Start() {
	// run connect(), if it fails try again every 2 seconds
	self.Begin(self.connect, time.Second*2)
}

func (self *ProducerManager) Stop() {
	self.End()
}

func (self *ProducerManager) connect() bool {
	opts := self.parser.GetOpts()

	brokerList := opts.StringSlice("kafka-endpoints")
	logrus.Info("Connecting to Kafka Cluster ", brokerList)
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3

	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		logrus.Error("NewSyncProducer() failed - ", err)
		return false
	}
	self.WithLock(func() {
		self.producer = NewProducer(self, opts.String("kafka-topic"), producer)
	})
	return true
}

// Injects kafka.ProducerManager into the context.Context for each request
func Middleware(producerManager *ProducerManager) func(chi.Handler) chi.Handler {
	return func(next chi.Handler) chi.Handler {
		return chi.HandlerFunc(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			ctx = SetProducerManager(ctx, producerManager)
			next.ServeHTTPC(ctx, resp, req)
		})
	}
}
