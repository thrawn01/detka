package detka

import (
	"net/http"
	"time"

	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/models"
	"github.com/thrawn01/detka/store"
	"golang.org/x/net/context"
)

func NewHandler(producerManager *kafka.ProducerManager, dbStore store.Store) http.Handler {
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
	router.Use(kafka.Middleware(producerManager))
	// Pass the store context into every request
	router.Use(store.Middleware(dbStore))

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
	router.Get("/messages/:messageId", GetMessage)

	return router
}

func GetMessage(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	// TODO: Authenticate User has access to create emails?

	id := chi.URLParam(ctx, "messageId")

	if err := models.ValidMessageId(id); err != nil {
		BadRequest(resp, err.Error(), logrus.Fields{"method": "GetMessage", "type": "validate"})
	}

	db := store.GetStore(ctx)

	var message *models.Message
	message, err := db.GetMessage(id)
	if err != nil {
		StoreError(resp, err, logrus.Fields{"method": "GetMessage", "type": "store"})
		return
	}

	ToJson(resp, message)
}

func NewMessages(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	// TODO: Authenticate User has access to create emails?

	req.ParseForm()

	msg := models.Message{
		Subject: req.FormValue("subject"),
		Text:    req.FormValue("text"),
		From:    req.FormValue("from"),
		To:      req.FormValue("to"),
	}

	logrus.Debugf("-> %+v\n", msg)

	// Validate the request
	if err := msg.Validate(); err != nil {
		BadRequest(resp, err.Error(), logrus.Fields{"method": "NewMessages", "type": "validate"})
		return
	}

	// Generate a new id
	msg.Id = models.NewId()
	msg.Status = "NEW"

	// Persist the email to the database before queuing
	dbStore := store.GetStore(ctx)
	if err := dbStore.InsertMessage(&msg); err != nil {
		InternalError(resp, err.Error(), logrus.Fields{"method": "NewMessages", "type": "store"})
		return
	}

	// Send the email request to the queue to be processed
	producer := kafka.GetProducer(ctx)
	if err := producer.Send(models.QueueMessage{Id: msg.Id, Type: "email"}); err != nil {
		InternalError(resp, err.Error(), logrus.Fields{"method": "NewMessages", "type": "kafka"})
		return
	}

	// Return the message id
	ToJson(resp, models.NewMessageResponse{Id: msg.Id, Message: "Queued, Thank you."})
}

func Healthz(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	// Validate the health of kafka producer
	producer := kafka.GetProducer(ctx)

	notReady := func() {
		resp.WriteHeader(500)
		resp.Write([]byte(`{"ready" : false}`))
	}

	if err := producer.Send(models.QueueMessage{Type: "ping"}); err != nil {
		notReady()
		return
	}

	// Validate the health of rethink
	dbStore := store.GetStore(ctx)
	if !dbStore.IsConnected() {
		notReady()
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"ready" : true}`))
}
