package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/Sirupsen/logrus"
	"github.com/braintree/manners"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/rethink"
)

func main() {
	parser := args.NewParser(
		args.Desc("Public API endpoint for baby mailgun"),
		args.EnvPrefix("API_"))
	parser.AddOption("--bind").Alias("-b").Env("BIND").
		Default("0.0.0.0:8080").Help("The interface to bind too")
	parser.AddOption("--debug").Alias("-d").IsTrue().Env("DEBUG").
		Help("Output debug messages")

	parser.AddOption("--kafka-endpoints").Alias("-e").Env("KAFKA_ENDPOINTS").
		Default("localhost:9092").Help("A comma separated list of kafka endpoints")
	parser.AddOption("--kafka-topic").Alias("-t").Default("detka-topic").
		Help("Topic used to produce and consumer mail messages")

	opt := parser.ParseArgsSimple(nil)
	if opt.Bool("debug") {
		logrus.Info("Debug Enabled")
		logrus.SetLevel(logrus.DebugLevel)
	}

	// kafka.Context manages kafka connections
	kafkaCtx := kafka.NewProducerManager(parser)
	// rethink.Context manages rethink connections
	rethinkCtx := rethink.NewManager(parser)

	// TODO: Setup args-backend watchers for rethink and kafka contexts

	server := manners.NewWithServer(&http.Server{
		Addr:    opt.String("bind"),
		Handler: detka.NewHandler(kafkaCtx, rethinkCtx),
	})

	fmt.Printf("Listening on %s...\n", opt.String("bind"))
	logrus.Fatal(server.ListenAndServe())

	// Catch SIGINT Gracefully so we don't drop any active http requests
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, os.Kill)
		sig := <-signalChan
		logrus.Info(fmt.Sprintf("Captured %v. Exiting...", sig))
		manners.Close()
		kafkaCtx.Stop()
		rethinkCtx.Stop()
	}()

}
