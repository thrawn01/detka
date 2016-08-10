package metrics

import "github.com/prometheus/client_golang/prometheus"

var HTTPRequestCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "api",
		Name:      "http_request_count",
		Help:      "The number of HTTP requests.",
	},
	[]string{"method", "endpoint"},
)

var HTTPRequestLatency = prometheus.NewSummaryVec(
	prometheus.SummaryOpts{
		Namespace: "api",
		Name:      "http_request_latency",
		Help:      "The latency of HTTP requests.",
	},
	[]string{"method", "endpoint"},
)

var InternalErrors = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "api",
		Name:      "internal_error_count",
		Help:      "The number of internal errors.",
	},
	[]string{"type", "method"},
)

var Non200Responses = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "api",
		Name:      "non_200_response_count",
		Help:      "The number of non 200 responses.",
	},
	[]string{"type", "method"},
)

// Must call before using the RecordMetrics() middleware
func Init() {
	prometheus.MustRegister(HTTPRequestCount)
	prometheus.MustRegister(HTTPRequestLatency)
	prometheus.MustRegister(InternalErrors)
}
