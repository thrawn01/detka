package kafka

import (
	"sync"

	"time"

	"net/http"

	"github.com/Shopify/sarama"
	"github.com/Sirupsen/logrus"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
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
	done      chan struct{}
	parser    *args.ArgParser
	current   chan Producer
	new       chan Producer
	reconnect chan []string
}

func NewProducerManager(parser *args.ArgParser) *ProducerManager {
	manager := &ProducerManager{
		parser: parser,
	}
	manager.Start()
	return manager
}

func (self *ProducerManager) Get() Producer {
	return <-self.current
}

func (self *ProducerManager) SignalReconnect() {
	// Always get the latest list of endpoints from our config
	self.reconnect <- self.parser.GetOpts().StringSlice("kafka-endpoints")
}

func (self *ProducerManager) Start() {
	self.current = make(chan Producer)
	self.new = make(chan Producer)
	self.reconnect = make(chan []string)
	self.done = make(chan struct{})

	// Always feed clients the latest kafka interface object
	go func() {
		defer close(self.current)
		var current Producer
		current = &NilProducer{}

		for {
			select {
			case self.current <- current:
			case current = <-self.new:
			case <-self.done:
				return
			}
		}
	}()

	var attemptedConnect sync.WaitGroup

	// Waits until it receives a list of brokers, then attempts to connect to the kafka cluster
	// if it fails will sleep for 1 second and try again
	go func() {
		defer close(self.new)
		var once sync.Once
		attemptedConnect.Add(1)

		for {
			var brokerList []string
			var timer <-chan time.Time

			select {
			case brokerList = <-self.reconnect:
			case <-timer:
			case <-self.done:
				return
			}

			logrus.Info("Connecting to Kafka Cluster ", brokerList)
			// Attempt to connect, if we fail to connect, set a timer to try again
			if !self.connect(brokerList) {
				timer = time.NewTimer(time.Second).C
			}
			once.Do(func() { attemptedConnect.Done() })
		}
	}()

	// Send the first connect signal
	self.SignalReconnect()
	// Attempt to connect at least once before we leave Start()
	attemptedConnect.Wait()
}

func (self *ProducerManager) connect(brokerList []string) bool {
	opts := self.parser.GetOpts()
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3

	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		logrus.Error("NewSyncProducer() failed - ", err)
		return false
	}
	self.new <- NewProducer(self, opts.String("kafka-topic"), producer)
	return true
}

func (self *ProducerManager) Stop() {
	close(self.done)
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
