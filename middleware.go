package detka

import (
	"net/http"
	"time"

	"github.com/howler-chat/api-service/metrics"
	"github.com/pressly/chi"
	"golang.org/x/net/context"
)

// Sets the 'Content-Type' to 'application/json'
func MimeJson(next chi.Handler) chi.Handler {
	return chi.HandlerFunc(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTPC(ctx, resp, req)
	})
}

// Records request count, and latency
func RecordMetrics(next chi.Handler) chi.Handler {
	return chi.HandlerFunc(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
		metrics.HTTPRequestCount.WithLabelValues(req.Method, req.URL.Path).Inc()
		startTime := time.Now()
		next.ServeHTTPC(ctx, resp, req)
		stopTime := time.Now()

		elapsed := stopTime.Sub(startTime)
		metrics.HTTPRequestLatency.WithLabelValues(req.Method, req.URL.Path).
			Observe(float64(elapsed) / float64(time.Millisecond))
	})
}
