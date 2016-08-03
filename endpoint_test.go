package detka_test

import (
	"testing"

	"net/http"
	"net/http/httptest"

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

	BeforeEach(func() {
		// Create a new handler instance
		server = detka.NewHandler(nil)
		// Record HTTP responses.
		resp = httptest.NewRecorder()
	})

	Describe("Error Conditions", func() {
		Context("When requested path doesn't exist", func() {
			It("should return 404", func() {
				req, _ = http.NewRequest("GET", "/path-not-found", nil)
				server.ServeHTTP(resp, req)
				Expect(resp.Code).To(Equal(404))
			})
		})
	})
})
