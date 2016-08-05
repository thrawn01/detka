package detka

import (
	"net/http"
	"time"

	"fmt"

	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thrawn01/detka/kafka"
	"golang.org/x/net/context"
)

func NewHandler(ctx *kafka.Context) http.Handler {
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
	// TODO: Pass our context into every request
	//router.Use(kafka.Middleware(ctx))

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

	router.Get("/message", NewMessage)

	return router
}

func NewMessage(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	// TODO: Authenticate User has access to create emails?

	req.ParseForm()

	msg := Message{
		Subject: req.FormValue("subject"),
		Text:    req.FormValue("text"),
		From:    req.FormValue("from"),
		To:      req.FormValue("to"),
	}

	// Validate the request
	if err := msg.Validate(); err != nil {
		BadRequest(resp, err, log.Fields{"method": "NewEmail", "type": "validate"})
	}

	// Generate a new id
	msg.Id = NewId()

	// Marshall the message back to json
	payload, err := json.Marshal(msg)
	if err != nil {
		InternalError(resp, err, log.Fields{"method": "NewEmail", "type": "json"})
		return
	}

	// Send the email request to the queue to be processed
	producer := kafka.GetKafka(ctx)
	if err := producer.Send(payload); err != nil {
		InternalError(resp, err, log.Fields{"method": "NewEmail", "type": "kafta"})
		return
	}

	// Return the message id
	ToJson(resp, NewMessageResponse{Id: msg.Id, Message: "Queued, Thank you."})
}

func Healthz(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	/*kafkaCtx := kafka.GetContext(ctx)
	if kafkaCtx.IsConnected() {
		resp.WriteHeader(200)
		resp.Write([]byte(`{"ready" : true}`))
		return
	}*/
	resp.WriteHeader(500)
	resp.Write([]byte(`{"ready" : false}`))
}
