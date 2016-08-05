package kafka

import (
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

/*config := sarama.NewConfig()
config.Producer.RequiredAcks = sarama.WaitForAll
config.Producer.Retry.Max = 3

producer, err := sarama.NewSyncProducer(brokerList, config)
if err != nil {
	log.Fatalln("Failed to start Sarama producer:", err)
}*/

type Context struct {
	parser *args.ArgParser
}

func NewContext(parser *args.ArgParser) *Context {
	return &Context{
		parser: parser,
	}
}

func (self *Context) Get() Kafka {
	return nil
}

func (self *Context) SignalReconnect() {

}

func (self *Context) IsConnected() bool {
	return false
}

func (self *Context) Start() {

}

func (self *Context) Stop() {

}

func (self *Context) UpdateBrokers(endpoints []string) {

}
