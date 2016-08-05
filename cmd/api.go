package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	log "github.com/Sirupsen/logrus"
	"github.com/braintree/manners"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka"
	"github.com/thrawn01/detka/kafka"
)

func main() {
	parser := args.NewParser(
		args.Desc("Public API endpoint for baby mailgun"),
		args.EnvPrefix("API_"))
	parser.AddOption("--bind").Alias("-b").Env("BIND").
		Default("0.0.0.0:8080").Help("The interface to bind too")
	parser.AddOption("--debug").Alias("-d").IsTrue().Env("DEBUG").
		Help("Output debug messages")

	opt := parser.ParseArgsSimple(nil)
	if opt.Bool("debug") {
		log.Info("Debug Enabled")
		log.SetLevel(log.DebugLevel)
	}

	// kafka.Context manages kafka connections
	ctx := kafka.NewContext(parser)
	ctx.Start()

	// TODO: Setup args-backend watchers for new kafka brokers when they come online

	server := manners.NewWithServer(&http.Server{
		Addr:    opt.String("bind"),
		Handler: detka.NewHandler(ctx),
	})

	fmt.Printf("Listening on %s...\n", opt.String("bind"))
	log.Fatal(server.ListenAndServe())

	// Catch SIGINT Gracefully so we don't drop any active http requests
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, os.Kill)
		sig := <-signalChan
		log.Info(fmt.Sprintf("Captured %v. Exiting...", sig))
		manners.Close()
		ctx.Stop()
	}()

}
