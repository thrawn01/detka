package detka

import (
	"net/http"
	"time"

	"fmt"

	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/dancannon/gorethink"
	"github.com/pkg/errors"
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
	messageId := chi.URLParam(ctx, "messageId")

	if err := ValidMessageId(messageId); err != nil {
		BadRequest(resp, err.Error(), logrus.Fields{"method": "GetMessage", "type": "validate"})
	}

	session := rethink.GetSession(ctx)
	if session == nil {
		InternalError(resp, "rethink session is nil",
			logrus.Fields{"method": "GetMessage", "type": "rethink"})
		return
	}

	var message Message
	cursor, err := gorethink.Table("messages").Get(messageId).Run(session, rethink.RunOpts)
	if err != nil {
		InternalError(resp, err.Error(), logrus.Fields{"method": "GetMessage", "type": "rethink"})
	} else if err := cursor.One(&message); err != nil {
		if cursor.IsNil() {
			NotFound(resp, fmt.Sprintf("message id - %s not found"),
				logrus.Fields{"method": "GetMessage", "type": "rethink"})
			return
		}
		InternalError(resp, err.Error(), logrus.Fields{"method": "GetMessage.One()", "type": "rethink"})
	} else {
		// Scrub the type from the messsage
		message.Type = ""
		ToJson(resp, message)
	}
}

func InsertMessage(session *gorethink.Session, msg *Message) error {
	msg.Id = NewId()
	changed, err := gorethink.Table("messages").Insert(msg).RunWrite(session, rethink.RunOpts)
	if err != nil {
		return errors.Wrap(err, "Insert")
	} else if changed.Errors != 0 {
		return errors.New(fmt.Sprintf("Errors on Insert - %s", changed.FirstError))
	}
	if len(changed.GeneratedKeys) == 0 {
		return errors.New(fmt.Sprintf("No Generated keys after Insert - %s", changed.FirstError))
	}
	return nil
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
		BadRequest(resp, err.Error(), logrus.Fields{"method": "NewMessages", "type": "validate"})
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
	/*kafkaCtx := kafka.GetContext(ctx)
	if kafkaCtx.IsConnected() {
		resp.WriteHeader(200)
		resp.Write([]byte(`{"ready" : true}`))
		return
	}*/
	resp.WriteHeader(500)
	resp.Write([]byte(`{"ready" : false}`))
}
