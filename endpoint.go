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
	"github.com/thrawn01/detka/rethink"
	"golang.org/x/net/context"
)

func NewHandler(kafkaCtx *kafka.ProducerManager, rethinkCtx *rethink.Manager) http.Handler {
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
	// Pass the kafka context into every request
	router.Use(kafka.Middleware(kafkaCtx))
	// Pass the rethink context into every request
	router.Use(rethink.Middleware(rethinkCtx))

	// TODO: Add API Throttling

	// Expose the metrics we have collected
	router.Get("/metrics", prometheus.Handler())
	router.Get("/healthz", Healthz)

	// Handle not found message in json
	router.NotFound(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(404)
		resp.Write([]byte(fmt.Sprintf(`{"error" : "Path '%s' Not Found"}`, req.URL.RequestURI())))
	})

	router.Post("/messages", NewMessages)

	return router
}

func NewMessages(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
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
		BadRequest(resp, err, log.Fields{"method": "NewMessages", "type": "validate"})
	}

	// Generate a new id
	msg.Id = NewId()

	// Marshall the message back to json
	payload, err := json.Marshal(msg)
	if err != nil {
		InternalError(resp, err, log.Fields{"method": "NewMessages", "type": "json"})
		return
	}

	// Send the email request to the queue to be processed
	producer := kafka.GetProducer(ctx)
	if err := producer.Send(payload); err != nil {
		InternalError(resp, err, log.Fields{"method": "NewMessages", "type": "kafta"})
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
