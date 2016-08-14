package detka_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

	//"io/ioutil"

	//"github.com/Sirupsen/logrus"
	logTest "github.com/Sirupsen/logrus/hooks/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka"
	"github.com/thrawn01/detka/kafka"
	"github.com/thrawn01/detka/models"
	"github.com/thrawn01/detka/rethink"
	"github.com/thrawn01/detka/store"
)

type TestMailer struct {
	Result *models.Message
	Done   sync.WaitGroup
}

func NewTestMailer(msg *models.Message) *TestMailer {
	test := &TestMailer{
		Result: msg,
	}
	test.Done.Add(1)
	return test
}

func (self *TestMailer) Send(msg *models.Message) error {
	self.Result = msg
	self.Done.Done()
	return nil
}

func TestDetka(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Endpoint Suite")
}

func okToTestFunctional() {
	if os.Getenv("DETKA_DOCKER_HOST") == "" {
		Skip("DETKA_DOCKER_HOST not set, skipped....")
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

	opts, _ := parser.ParseArgs(argv)

	host := os.Getenv("DETKA_DOCKER_HOST")
	opts.Set("kafka-endpoints", fmt.Sprintf("%s:9092", host))
	opts.Set("rethink-endpoints", fmt.Sprintf("%s:28015", host))

	// TODO: Topic should be different for each run?
	//opts.Set("kafka-topic", "some-generated-value")

	parser.Apply(opts)
	return parser
}

var _ = Describe("Functional Tests", func() {
	var server http.Handler
	var req *http.Request
	var resp *httptest.ResponseRecorder
	var hook *logTest.Hook
	var producerManager *kafka.ProducerManager
	var rethinkManager *rethink.Manager
	var parser *args.ArgParser
	var dbStore store.Store

	BeforeEach(func() {
		// Avoid printing log entries to StdError
		//logrus.SetOutput(ioutil.Discard)
		// Allow us to inspect log messages
		hook = logTest.NewGlobal()
		// Get our kafka Config from our local Environment
		parser = parseArgs(nil)
		// Create a kafka context for our service
		producerManager = kafka.NewProducerManager(parser)
		// Create a rethink context for our service
		rethinkManager = rethink.NewManager(parser)
		// Create the database store
		dbStore = store.NewRethinkStore(parser, rethinkManager)
		// Create a new handler instance
		server = detka.NewHandler(producerManager, dbStore)
		// Record HTTP responses.
		resp = httptest.NewRecorder()
	})

	AfterEach(func() {
		producerManager.Stop()
		rethinkManager.Stop()
		hook.Reset()
	})

	Describe("Service Conditions", func() {
		Context("When requested path doesn't exist", func() {
			It("should return 404", func() {
				server = detka.NewHandler(nil, nil)
				resp = httptest.NewRecorder()
				req, _ = http.NewRequest("GET", "/path-not-found", nil)
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Equal(404))
			})
		})
		Context("When app is ready /healthz", func() {
			It("should return 200", func() {
				okToTestFunctional()
				req, _ = http.NewRequest("GET", "/healthz", nil)
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Equal(200))
				Expect(resp.Body.String()).To(Equal(`{"ready" : true}`))
			})
		})
	})

	Describe("POST /messages", func() {
		var consumerManager *kafka.ConsumerManager
		var worker *detka.Worker
		var mailResult models.Message
		var mailer *TestMailer

		BeforeEach(func() {
			consumerManager = kafka.NewConsumerManager(parser)
			mailer = NewTestMailer(&mailResult)
			worker = detka.NewWorker(consumerManager, dbStore, mailer)
		})

		AfterEach(func() {
			consumerManager.Stop()
			worker.Stop()
		})

		Context("When an email message is posted", func() {
			It("should send an email", func() {
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

				var respMsg models.NewMessageResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &respMsg); err != nil {
					Fail(err.Error())
				}
				Expect(len(respMsg.Id)).To(Equal(26))
				Expect(respMsg.Message).To(Equal("Queued, Thank you."))

				// Wait until Send() is called on our mailer
				mailer.Done.Wait()

				// Get the message
				msg := mailer.Result
				Expect(msg.From).To(Equal("derrick@rackspace.com"))
				Expect(msg.To).To(Equal("derrick@rackspace.com"))
				Expect(msg.Text).To(Equal("this is a test"))
				Expect(msg.Subject).To(Equal("this is a test subject"))
				Expect(len(msg.Id)).To(Equal(26))

				resp = httptest.NewRecorder()
				var savedMsg models.Message

				// API should respond with message in a "DELIVERED" status
				req, _ = http.NewRequest("GET", fmt.Sprintf("/messages/%s", msg.Id), nil)
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Equal(200))

				if err := json.Unmarshal(resp.Body.Bytes(), &savedMsg); err != nil {
					Fail(err.Error())
				}
				Expect(savedMsg.From).To(Equal("derrick@rackspace.com"))
				Expect(savedMsg.To).To(Equal("derrick@rackspace.com"))
				Expect(savedMsg.Text).To(Equal("this is a test"))
				Expect(savedMsg.Subject).To(Equal("this is a test subject"))
				Expect(savedMsg.Status).To(Equal("DELIVERED"))
				Expect(len(savedMsg.Id)).To(Equal(26))
			})

			/*It("should update the message in the database", func() {

			})*/
		})
	})

	Describe("GET /messages", func() {
		Context("When proper request is made", func() {
			It("should return 200", func() {
				okToTestFunctional()
				originalMsg := models.Message{
					To:      "derrick@rackspace.com",
					From:    "derrick@rackspace.com",
					Text:    "this is a test",
					Subject: "this is a test subject",
					Status:  "NEW",
				}

				// Get the session
				session := rethinkManager.GetSession()
				Expect(session).To(Not(BeNil()))

				// Insert a message into the db
				err := dbStore.InsertMessage(&originalMsg)
				Expect(err).To(BeNil())

				// Use the endpoint to query the message
				req, _ = http.NewRequest("GET", fmt.Sprintf("/messages/%s", originalMsg.Id), nil)
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Equal(200))

				var msg models.Message
				if err := json.Unmarshal(resp.Body.Bytes(), &msg); err != nil {
					Fail(err.Error())
				}
				Expect(msg.From).To(Equal("derrick@rackspace.com"))
				Expect(msg.To).To(Equal("derrick@rackspace.com"))
				Expect(msg.Text).To(Equal("this is a test"))
				Expect(msg.Subject).To(Equal("this is a test subject"))
				Expect(msg.Status).To(Equal("NEW"))
				Expect(len(msg.Id)).To(Equal(26))
			})
		})
	})

})
