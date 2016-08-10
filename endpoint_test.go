package detka_test

import (
	"os"
	"testing"

	"net/http"
	"net/http/httptest"
	"net/url"

	"encoding/json"

	"fmt"

	logTest "github.com/Sirupsen/logrus/hooks/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/rethink"
)

func TestDetka(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Endpoint Suite")
}

func okToTestFunctional() {
	if os.Getenv("KAFKA_ENDPOINTS") == "" {
		Skip("KAFKA_ENDPOINTS not set, skipped....")
	}
	if os.Getenv("RETHINK_ENDPOINTS") == "" {
		Skip("RETHINK_ENDPOINTS not set, skipped....")
	}
}

func parseArgs(argv *[]string) *args.ArgParser {
	parser := args.NewParser()
	parser.AddOption("--kafka-endpoints").Env("KAFKA_ENDPOINTS")
	parser.AddOption("--kafka-topic").Default("detka-topic")
	parser.AddOption("--rethink-auto-create").IsBool().Default("true")
	parser.AddOption("--rethink-endpoints").Alias("-e").Env("RETHINK_ENDPOINTS")
	parser.AddOption("--rethink-user").Env("RETHINK_USER")
	parser.AddOption("--rethink-password").Env("RETHINK_PASSWORD")
	parser.AddOption("--rethink-db").Env("RETHINK_DATABASE").Default("detka")

	parser.ParseArgs(argv)
	// TODO: Topic should be different for each run
	//opts.Set("kafka-topic", "some-generated-value")
	//parser.Apply(opts)
	return parser
}

var _ = Describe("Endpoint", func() {
	var server http.Handler
	var req *http.Request
	var resp *httptest.ResponseRecorder
	var hook *logTest.Hook
	var producerManager *kafka.ProducerManager
	var rethinkManager *rethink.Manager
	var parser *args.ArgParser

	BeforeEach(func() {
		// Avoid printing log entries to StdError
		//log.SetOutput(ioutil.Discard)
		// Allow us to inspect log messages
		hook = logTest.NewGlobal()
		// Get our kafka Config from our local Environment
		parser = parseArgs(nil)
		// Create a kafka context for our service
		producerManager = kafka.NewProducerManager(parser)
		// Create a rethink context for our service
		rethinkManager = rethink.NewManager(parser)
		// Create a new handler instance
		server = detka.NewHandler(producerManager, rethinkManager)
		// Record HTTP responses.
		resp = httptest.NewRecorder()
	})

	AfterEach(func() {
		producerManager.Stop()
		rethinkManager.Stop()
		hook.Reset()
	})

	Describe("Error Conditions", func() {
		Context("When requested path doesn't exist", func() {
			It("should return 404", func() {
				server = detka.NewHandler(nil, nil)
				resp = httptest.NewRecorder()
				req, _ = http.NewRequest("GET", "/path-not-found", nil)
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Equal(404))
			})
		})
		Context("When app is not ready /healthz", func() {
			It("should return non 200", func() {
				server = detka.NewHandler(nil, nil)
				resp = httptest.NewRecorder()
				req, _ = http.NewRequest("GET", "/healthz", nil)
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Not(Equal(200)))
			})
		})
	})

	Describe("POST /messages", func() {
		var manager *kafka.ConsumerManager

		BeforeEach(func() {
			manager = kafka.NewConsumerManager(parser)
		})

		AfterEach(func() {
			manager.Stop()
		})

		Context("When proper request is made", func() {
			It("should return 200", func() {
				okToTestFunctional()
				req, _ = http.NewRequest("POST", "/messages", nil)
				req.Form = url.Values{
					"to":      {"derrick@rackspace.com"},
					"from":    {"derrick@rackspace.com"},
					"text":    {"this is a test"},
					"subject": {"this is a test subject"},
				}
				// Server should have submitted the request successfully
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Equal(200))

				// Get a consumer and verify the message is in the topic
				consumer := manager.GetConsumer()
				payload := <-consumer.Messages()
				var msg detka.Message
				if err := json.Unmarshal(payload.Value, &msg); err != nil {
					Fail(err.Error())
				}
				Expect(msg.From).To(Equal("derrick@rackspace.com"))
				Expect(msg.To).To(Equal("derrick@rackspace.com"))
				Expect(msg.Text).To(Equal("this is a test"))
				Expect(msg.Subject).To(Equal("this is a test subject"))
				Expect(len(msg.Id)).To(Equal(26))
			})
		})
	})

	Describe("GET /messages", func() {
		Context("When proper request is made", func() {
			It("should return 200", func() {
				okToTestFunctional()
				originalMsg := detka.Message{
					To:      "derrick@rackspace.com",
					From:    "derrick@rackspace.com",
					Text:    "this is a test",
					Subject: "this is a test subject",
					Status:  "NEW",
					Type:    "MESSAGE",
				}

				// Get the session
				session := rethinkManager.GetSession()
				Expect(session).To(Not(BeNil()))

				// Insert a message into the db
				detka.InsertMessage(session, &originalMsg)

				// Use the endpoint to query the message
				req, _ = http.NewRequest("GET", fmt.Sprintf("/messages/%s", originalMsg.Id), nil)
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Equal(200))

				var msg detka.Message
				if err := json.Unmarshal(resp.Body.Bytes(), &msg); err != nil {
					Fail(err.Error())
				}
				Expect(msg.From).To(Equal("derrick@rackspace.com"))
				Expect(msg.To).To(Equal("derrick@rackspace.com"))
				Expect(msg.Text).To(Equal("this is a test"))
				Expect(msg.Subject).To(Equal("this is a test subject"))
				Expect(msg.Status).To(Equal("NEW"))
				Expect(msg.Type).To(Equal(""))
				Expect(len(msg.Id)).To(Equal(26))
			})
		})
	})
})
