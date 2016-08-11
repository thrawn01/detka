package detka

import (
	"net/http"
	"time"

	"fmt"

	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/dancannon/gorethink"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/rethink"
	"golang.org/x/net/context"
)

func NewHandler(producerManager *kafka.ProducerManager, rethinkManager *rethink.Manager) http.Handler {
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
	// Pass the rethink context into every request
	router.Use(rethink.Middleware(rethinkManager))

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

	msgId := chi.URLParam(ctx, "messageId")

	if err := ValidMessageId(msgId); err != nil {
		BadRequest(resp, err.Error(), logrus.Fields{"method": "GetMessage", "type": "validate"})
	}

	session := rethink.GetSession(ctx)
	if session == nil {
		InternalError(resp, "rethink session is nil",
			logrus.Fields{"method": "GetMessage", "type": "rethink"})
		return
	}

	var message Message
	cursor, err := gorethink.Table("messages").Get(msgId).Run(session, rethink.RunOpts)
	if err != nil {
		InternalError(resp, err.Error(), logrus.Fields{"method": "GetMessage", "type": "rethink"})
	} else if err := cursor.One(&message); err != nil {
		if cursor.IsNil() {
			NotFound(resp, fmt.Sprintf("message id - %s not found", msgId),
				logrus.Fields{"method": "GetMessage", "type": "rethink"})
			return
		}
		InternalError(resp, err.Error(), logrus.Fields{"method": "GetMessage.One()", "type": "rethink"})
	} else {
		// Scrub the type from the message
		message.Type = ""
		ToJson(resp, message)
	}
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

	logrus.Debugf("-> %+v\n", msg)

	// Validate the request
	if err := msg.Validate(); err != nil {
		BadRequest(resp, err.Error(), logrus.Fields{"method": "NewMessages", "type": "validate"})
		return
	}

	// Generate a new id
	msg.Id = NewId()

	// Marshall the message back to json
	payload, err := json.Marshal(msg)
	if err != nil {
		InternalError(resp, err.Error(), logrus.Fields{"method": "NewMessages", "type": "json"})
		return
	}

	// Send the email request to the queue to be processed
	producer := kafka.GetProducer(ctx)
	if err := producer.Send(payload); err != nil {
		InternalError(resp, err.Error(), logrus.Fields{"method": "NewMessages", "type": "kafta"})
		return
	}

	// Return the message id
	ToJson(resp, NewMessageResponse{Id: msg.Id, Message: "Queued, Thank you."})
}

func Healthz(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
	// Validate the health of kafka producer
	producer := kafka.GetProducer(ctx)

	notReady := func() {
		resp.WriteHeader(500)
		resp.Write([]byte(`{"ready" : false}`))
	}

	payload, err := json.Marshal(Message{Type: "ping"})
	if err != nil {
		notReady()
		return
	}

	if err := producer.Send(payload); err != nil {
		notReady()
		return
	}

	// Validate the health of rethink
	rethinkMgr := rethink.GetManager(ctx)
	session := rethinkMgr.GetSession()
	if session == nil || !session.IsConnected() {
		notReady()
		return
	}

	resp.WriteHeader(200)
	resp.Write([]byte(`{"ready" : true}`))
}
