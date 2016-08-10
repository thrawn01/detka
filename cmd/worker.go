package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/Shopify/sarama"
	"github.com/Sirupsen/logrus"
	"github.com/braintree/manners"
	"github.com/dancannon/gorethink"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/rethink"
	"golang.org/x/net/context"
)

type HandleFunc func(gorethink.Session, *args.ArgParser, sarama.ConsumerMessage) error

func main() {
	parser := args.NewParser(args.Desc("Mail Workers for baby mailgun"),
		args.EnvPrefix("WORKER"))
	parser.AddOption("--bind").Alias("-b").Env("BIND").
		Default("0.0.0.0:1234").Help("The interface to bind too")
	parser.AddOption("--debug").Alias("-d").IsTrue().Env("DEBUG").
		Help("Output debug messages")

	parser.AddOption("--kafka-endpoints").Alias("-e").Env("KAFKA_ENDPOINTS").
		Default("localhost:9092").Help("A comma separated list of kafka endpoints")
	parser.AddOption("--kafka-topic").Alias("-t").Default("detka-topic").
		Help("Topic used to produce and consumer mail messages")

	parser.AddOption("--rethink-endpoints").Alias("-e").Env("RETHINK_ENDPOINTS").
		Default("localhost:28015").Help("A comma separated list of rethink endpoints")
	parser.AddOption("--rethink-user").Alias("-u").Env("RETHINK_USER").
		Help("RethinkDB Username")
	parser.AddOption("--rethink-password").Alias("-p").Env("RETHINK_PASSWORD").
		Help("RethinkDB Password")
	parser.AddOption("--rethink-db").Alias("-d").Env("RETHINK_DATABASE").Default("detka").
		Help("RethinkDB Database name")
	parser.AddOption("--rethink-auto-create").IsBool().Default("true").Env("RETHINK_AUTO_CREATE").
		Help("Create db and tables if none exists")

	opt := parser.ParseArgsSimple(nil)
	if opt.Bool("debug") {
		logrus.Info("Debug Enabled")
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Main loop to handle messages from the event loop
	cancel := consumeMessages(parser, handleMessage)

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, os.Kill)
		sig := <-signalChan
		logrus.Info(fmt.Sprintf("Captured %v. Exiting...", sig))
		manners.Close()
		cancel()
	}()

	// Serve up our healthz and metrics endpoints
	router := chi.NewRouter()
	router.Get("/healthz", func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		// TODO: Return 200 if we are connected to kafka and rethink
		resp.WriteHeader(500)
		resp.Write([]byte(`{"ready" : false}`))
	})

	server := manners.NewWithServer(&http.Server{
		Addr:    opt.String("bind"),
		Handler: router,
	})

	fmt.Printf("Listening on %s...\n", opt.String("bind"))
	logrus.Fatal(server.ListenAndServe())

}

func consumeMessages(parser *args.ArgParser, handleMessage HandleFunc) func() {
	rethinkMgr := rethink.NewManager(parser)
	consumerMgr := kafka.NewConsumerManager(parser)
	consumerChan := consumerMgr.GetConsumerChannel()
	done := make(chan struct{})

	go func() {
		var messages <-chan *sarama.ConsumerMessage
		var errors <-chan *sarama.ConsumerError

		select {
		case consumer := <-consumerChan:
			messages = consumer.Messages()
			errors = consumer.Errors()
		case msg := <-messages:
			// TODO: If we can't get a session, or if we error on updating the database
			// TODO: Should we block until we get the database back again or put the
			// TODO: message back on the queue?
			session := rethinkMgr.GetSession()
			if err := handleMessage(session, parser, msg); err != nil {
				logrus.WithFields(logrus.Fields{
					"type":   "rethink",
					"method": "handleMessage()",
				}).Error("Received Error - ", err.Error())
				if !session.IsConnected() {
					// TODO: Since the database is disconnected we should also
					// TODO: not accept new messages to process.
					rethinkMgr.SignalReconnect()
				}
			}
		case err := <-errors:
			logrus.WithFields(logrus.Fields{
				"type":   "kafka",
				"method": "Start()",
			}).Error("Received Error - ", err.Error())
			consumerMgr.SignalReconnect()
		case <-done:
			return
		}
	}()
	return func() { close(done) }
}

func updateStatus(msgId, status string) error {
	return nil
}

func sendEmail(msg detka.Message) error {
	//opts := self.parser.GetOpts()
	return nil
}

func handleMessage(session gorethink.Session, parser *args.ArgParser, msg sarama.ConsumerMessage) error {
	fmt.Printf("Got new message -> %+v", msg)

	// Marshal Message from JSON
	//msg.Value

	// If the message is a ping, ignore it
	/*if msg.Type == "ping" {
		return
	}*/

	// Attempt to send the message
	/*opts := parser.GetOpts()

	for i := 0; i < opts.Int("retry"); i++ {
		// Send message
		if err := sendEmail(msg); err != nil {
			log.WithFields(log.Fields{
				"method": "processMessage",
				"type":   "mail",
			}).Error("sendEmail - ", err.Error())
		}
		break
	}

	// On Success update the database 'status' field
	if err := updateStatus(msg.Id, "delivered"); err != nil {
		log.WithFields(log.Fields{
			"method": "processMessage",
			"type":   "rethink",
		}).Error("updateStatus - ", err.Error())

		// TODO: Requeue the message so it can be saved once the database comes backup?
		// Or just continue to try and save until we get the DB back?
	}*/
}
