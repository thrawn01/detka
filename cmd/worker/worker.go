package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"time"

	"github.com/Sirupsen/logrus"
	"github.com/braintree/manners"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/store"
	"golang.org/x/net/context"
)

func main() {
	parser := args.NewParser(args.Desc("Mail Workers for baby mailgun"),
		args.EnvPrefix("WORKER_"))
	parser.AddOption("--bind").Alias("-b").Env("BIND").
		Default("0.0.0.0:4141").Help("The interface to bind too")
	parser.AddOption("--debug").Alias("-d").IsTrue().Env("DEBUG").
		Help("Output debug messages")
	parser.AddOption("--config").Alias("-c").Help("Read options from a config file")

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

	// Decide which mail transport to use
	parser.AddOption("--mail-transport").Alias("-M").Default("smtp").Env("MAIL_TRANSPORT").
		Help("Choose what transport to use. choices('mailgun', 'smtp')")
	parser.AddOption("--transport-retry").Alias("-R").Default("3").Env("TRANSPORT_RETRY").
		Help("How many retries before giving up")

	// Mailgun Options
	parser.AddOption("--mailgun-domain").Env("MAILGUN_DOMAIN").Help("Mailgun Domain")
	parser.AddOption("--mailgun-api-key").Env("MAILGUN_API_KEY").Help("Mailgun api-key")
	parser.AddOption("--mailgun-public-key").Env("MAILGUN_PUBLIC_KEY").Help("Mailgun public-key")

	// SMTP Options
	parser.AddOption("--smtp-server").Env("SMTP_SERVER").Help("SMTP Server (mail.example.com:25)")
	parser.AddOption("--smtp-user").Env("SMTP_USER").Help("SMTP User")
	parser.AddOption("--smtp-password").Env("SMTP_PASSWORD").Help("SMTP Password")

	opt := parser.ParseArgsSimple(nil)
	if opt.Bool("debug") {
		logrus.Info("Debug Enabled")
		logrus.SetLevel(logrus.DebugLevel)
	}

	if opt.IsSet("config") {
		content, err := detka.LoadFile(opt.String("config"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config - %s\n", err.Error())
			os.Exit(1)
		}
		opt, err = parser.FromIni(content)
	}

	mailer, err := detka.NewMailer(parser)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init Mailer - %s\n", err.Error())
		os.Exit(1)
	}

	dbStore := store.NewRethinkStore(parser, nil)
	consumerManager := kafka.NewConsumerManager(parser)

	// Worker to handle messages from the event loop
	worker := detka.NewWorker(consumerManager, dbStore, mailer)

	if opt.IsSet("config") {
		configFile := opt.String("config")
		// Watch our config file for changes
		cancelWatch, err := args.WatchFile(configFile, time.Second, func(err error) {
			if err != nil {
				logrus.Errorf("Error Watching %s - %s", configFile, err.Error())
				return
			}

			logrus.Info("Config file changed, Reloading...")
			content, err := detka.LoadFile(configFile)
			if err != nil {
				logrus.Error("Failed to load config - ", err.Error())
				return
			}
			_, err = parser.FromIni(content)
			if err != nil {
				logrus.Info("Failed to update config - ", err.Error())
				return
			}
			// Perhaps our endpoints changed, we should reconnect
			dbStore.SignalReconnect()
			consumerManager.Start()

			// Perhaps our mailer config changed
			mailer, err := detka.NewMailer(parser)
			if err != nil {
				logrus.Error("Failed to init Mailer - ", err.Error())
				return
			}
			// Stop the current worker and create a new one
			worker.Stop()
			worker = detka.NewWorker(consumerManager, dbStore, mailer)
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to watch '%s' -  %s", configFile, err.Error())
		}
		// Shut down the watcher when done
		defer cancelWatch()
	}

	// Serve up /healthz to indicate the health of our service
	router := chi.NewRouter()
	router.Get("/healthz", func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json; charset=utf-8")
		notReady := func() {
			resp.WriteHeader(500)
			resp.Write([]byte(`{"ready" : false}`))
		}

		// Validate the health of kafka consumer
		if !consumerManager.IsConnected() {
			notReady()
			return
		}

		// Is rethink connected?
		if !dbStore.IsConnected() {
			notReady()
			return
		}

		resp.WriteHeader(200)
		resp.Write([]byte(`{"ready" : true}`))
	})
	// TODO: Add metrics handler

	server := manners.NewWithServer(&http.Server{
		Addr:    opt.String("bind"),
		Handler: router,
	})

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, os.Kill)
		sig := <-signalChan
		logrus.Info(fmt.Sprintf("Captured %v. Exiting...", sig))
		server.Close()
		worker.Stop()
	}()

	logrus.Infof("Listening on %s...\n", opt.String("bind"))
	err = server.ListenAndServe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server Error - %s\n", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
