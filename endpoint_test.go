package detka_test

import (
	"io/ioutil"
	"testing"

	"net/http"
	"net/http/httptest"

	log "github.com/Sirupsen/logrus"
	logTest "github.com/Sirupsen/logrus/hooks/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thrawn01/detka"
)

func TestDetka(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Endpoint Suite")
}

var _ = Describe("Endpoint", func() {
	var server http.Handler
	var req *http.Request
	var resp *httptest.ResponseRecorder
	var hook *logTest.Hook

	BeforeEach(func() {
		// Avoid printing log entries to StdError
		log.SetOutput(ioutil.Discard)
		// Allow us to inspect log messages
		hook = logTest.NewGlobal()
		// Create a new handler instance
		server = detka.NewHandler(nil)
		// Record HTTP responses.
		resp = httptest.NewRecorder()
	})

	AfterEach(func() {
		hook.Reset()
	})

	Describe("Error Conditions", func() {
		Context("When requested path doesn't exist", func() {
			It("should return 404", func() {
				req, _ = http.NewRequest("GET", "/path-not-found", nil)
				server.ServeHTTP(resp, req)
				Expect(len(hook.Entries)).To(Equal(1))
				Expect(resp.Code).To(Equal(404))
			})
		})
		Context("When app is not ready /healthz", func() {
			It("should return non 200", func() {
				req, _ = http.NewRequest("GET", "/healthz", nil)
				server.ServeHTTP(resp, req)
				Expect(len(hook.Entries)).To(Equal(1))
				Expect(resp.Code).To(Not(Equal(200)))
			})
		})
	})
})
