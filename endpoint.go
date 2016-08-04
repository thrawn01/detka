package detka

import (
	"net/http"
	"time"

	"fmt"

	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thrawn01/args"
	"golang.org/x/net/context"
)

func NewHandler(parser *args.ArgParser) http.Handler {
	router := chi.NewRouter()

	// Log Every Request
	router.Use(Logger)
	// Recover from panic's
	router.Use(middleware.Recoverer)
	// Timeout in 1 second
	router.Use(middleware.Timeout(1 * time.Second))
	// Set JSON headers for every request
	router.Use(MimeJson)
	// Record Metrics for every request
	router.Use(RecordMetrics)

	// TODO: Add queue pipeline to context

	// TODO: Add API Throttling

	// Expose the metrics we have collected
	router.Get("/metrics", prometheus.Handler())
	router.Get("/healthz", Healthz)

	// Handle not found message in json
	router.NotFound(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(404)
		resp.Write([]byte(fmt.Sprintf(`{"error" : "Path '%s' Not Found"}`, req.URL.RequestURI())))
	})

	router.Get("/message", NewEmail)

	return router
}

func NewEmail(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	// Validate the request

	// Authenticate User has access to create emails?

	// Generate a request id

	// Send the email request to the queue to be processed

	// Return the request id
	resp.Write([]byte(`{"requestId" : "fakeid"}`))
}

func Healthz(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	resp.WriteHeader(500)
	// TODO: Return 200 {'Ready': true} when connected to our email queue
	resp.Write([]byte(`{"ready" : false}`))
}
