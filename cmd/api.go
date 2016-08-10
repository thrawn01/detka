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

	// manages kafka connections
	producerManager := kafka.NewProducerManager(parser)
	// manages rethink connections
	rethinkManager := rethink.NewManager(parser)

	// TODO: Setup args-backend watchers for rethink and kafka contexts

	server := manners.NewWithServer(&http.Server{
		Addr:    opt.String("bind"),
		Handler: detka.NewHandler(producerManager, rethinkManager),
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
		producerManager.Stop()
		rethinkManager.Stop()
	}()

}
