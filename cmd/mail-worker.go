package main

import (
	"fmt"

	"os"
	"os/signal"

	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/braintree/manners"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka"
	"github.com/thrawn01/detka/kafka"
	"golang.org/x/net/context"
)

func updateStatus(msgId, status string) error {
	return nil
}

func sendEmail(opts *args.Options, msg detka.Message) error {
	return nil
}

func processMessage(parser *args.ArgParser, msg detka.Message) {
	opts := parser.GetOpts()

	for i := 0; i < opts.Int("retry"); i++ {
		// Send message
		if err := sendEmail(parser, msg); err != nil {
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
	}
}

func main() {
	parser := args.NewParser(args.Desc("Mail Workers for baby mailgun"),
		args.EnvPrefix("WORKER"))
	parser.AddOption("--bind").Alias("-b").Env("BIND").
		Default("0.0.0.0:1234").Help("The interface to bind too")
	parser.AddOption("--debug").Alias("-d").IsTrue().Env("DEBUG").
		Help("Output debug messages")

	parser.AddOption("--kafka-endpoints").Alias("-e").Env("KAFKA_ENDPOINTS").
		Default("localhost:9092").Help("A comma separated list of kafka endpoints")

	parser.AddOption("--rethink-endpoints").Alias("-e").Env("RETHINK_ENDPOINTS").
		Default("localhost:28015").Help("A comma separated list of rethink endpoints")
	parser.AddOption("--rethink-user").Alias("-u").Env("RETHINK_USER").
		Help("RethinkDB Username")
	parser.AddOption("--rethink-password").Alias("-p").Env("RETHINK_PASSWORD").
		Help("RethinkDB Password")
	parser.AddOption("--rethink-db").Alias("-d").Env("RETHINK_DATABASE").
		Help("RethinkDB Database name")

	opt := parser.ParseArgsSimple(nil)
	if opt.Bool("debug") {
		log.Info("Debug Enabled")
		log.SetLevel(log.DebugLevel)
	}

	// Serve up our healthz and metrics endpoints
	go func() {
		router := chi.NewRouter()
		router.Get("/healthz", Healthz)

		server := manners.NewWithServer(&http.Server{
			Addr:    opt.String("bind"),
			Handler: router,
		})

		fmt.Printf("Listening on %s...\n", opt.String("bind"))
		log.Fatal(server.ListenAndServe())
	}()

	// TODO: Connect to kafka and rethink

	// Main event loop
	os.Exit(run(parser))
}

func run(parser *args.ArgParser) int {
	var msg detka.Message
	var consumer kafka.Consumer
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, os.Kill)

	for {
		select {
		case msg = <-consumer.Get():
			fmt.Printf("Got new message -> %s", msg)
			// If the message is a ping, ignore it
			if msg.Type != "ping" {
				// TODO: goroutine this and use a waitgroup
				processMessage(parser, msg)
			}
		case sig := <-signalChan:
			log.Info(fmt.Sprintf("Captured %v. Exiting...", sig))
			manners.Close()
			return 1
		}
	}
	return 0
}

func Healthz(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	// TODO: Return 200 if we are connected to kafka and rethink
	resp.WriteHeader(500)
	resp.Write([]byte(`{"ready" : false}`))
}
