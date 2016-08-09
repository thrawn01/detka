package kafka

import (
	"sync"

	"time"

	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
	"github.com/thrawn01/args"
	"golang.org/x/net/context"
)

func SetContext(ctx context.Context, kafkaCtx *Context) context.Context {
	return context.WithValue(ctx, kafkaContextKey, kafkaCtx)
}

func GetContext(ctx context.Context) *Context {
	obj, ok := ctx.Value(kafkaContextKey).(*Context)
	if !ok {
		panic("No kafka.Context found in context")
	}
	return obj
}

type Context struct {
	done      chan struct{}
	parser    *args.ArgParser
	current   chan Kafka
	new       chan Kafka
	reconnect chan []string
}

func NewContext(parser *args.ArgParser) *Context {
	return &Context{
		parser: parser,
	}
}

func (self *Context) Get() Kafka {
	return <-self.current
}

// Tell the context goroutine to start reconnecting
func (self *Context) SignalReconnect() {
	// Always get the latest list of endpoints from our config
	self.reconnect <- self.parser.GetOpts().StringSlice("kafka-endpoints")
}

// Start 2 goroutines, the first one provides the current kafka interface
// the second connects or reconnects to the kakfa cluster
func (self *Context) Start() {
	self.current = make(chan Kafka)
	self.new = make(chan Kafka)
	self.reconnect = make(chan []string)
	self.done = make(chan struct{})

	// Always feed clients the latest kafka interface object
	go func() {
		defer close(self.current)
		var current Kafka
		current = &KafkaNil{}

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
				goto connect
			case <-timer:
				goto connect
			case <-self.done:
				return
			}
		connect:
			log.Info("Connecting to Kafka Cluster ", brokerList)
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

func (self *Context) connect(brokerList []string) bool {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3

	// During our reconnect give new requests a nil impl
	self.new <- &KafkaNil{}

	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		log.Error("NewSyncProducer() failed - ", err)
		return false
	}
	self.new <- NewKafa(self, producer)
	return true
}

func (self *Context) Stop() {
	close(self.done)
}
