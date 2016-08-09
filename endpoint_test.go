package detka_test

import (
	"os"
	"testing"

	"net/http"
	"net/http/httptest"
	"net/url"

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

func okToTestKafka() {
	if os.Getenv("KAFKA_ENDPOINTS") == "" {
		Skip("KAFKA_ENDPOINTS not set, skipped....")
	}
}

func parseArgs(argv *[]string) *args.ArgParser {
	parser := args.NewParser()
	parser.AddOption("--kafka-endpoints").Alias("-e").Env("KAFKA_ENDPOINTS")
	parser.AddOption("--kafka-topic").Alias("-t").Default("detka-topic")
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
	var kafkaCtx *kafka.ProducerManager
	var rethinkCtx *rethink.Manager
	var parser *args.ArgParser

	BeforeEach(func() {
		// Avoid printing log entries to StdError
		//log.SetOutput(ioutil.Discard)
		// Allow us to inspect log messages
		hook = logTest.NewGlobal()
		// Get our kafka Config from our local Environment
		parser = parseArgs(nil)
		// Create a kafka context for our service
		kafkaCtx = kafka.NewProducerManager(parser)
		// Create a rethink context for our service
		rethinkCtx = rethink.NewManager(parser)
		// Create a new handler instance
		server = detka.NewHandler(kafkaCtx, rethinkCtx)
		// Record HTTP responses.
		resp = httptest.NewRecorder()
	})

	AfterEach(func() {
		kafkaCtx.Stop()
		rethinkCtx.Stop()
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
		Context("When proper request is made", func() {
			It("should return non 200", func() {
				okToTestKafka()
				req, _ = http.NewRequest("POST", "/messages", nil)
				req.Form = url.Values{
					"to":      {"derrick@rackspace.com"},
					"from":    {"derrick@rackspace.com"},
					"text":    {"this is a test"},
					"subject": {"this is a test subject"},
				}
				server.ServeHTTP(resp, req)
				//Expect(len(hook.Entries)).To(Equal(2))
				Expect(resp.Code).To(Equal(200))

				/*consumer = kafka.NewConsumer(parser)
				message := consumer.Get()
				Expect(message).To(Equal([]byte("")))*/
			})
		})
	})
})
