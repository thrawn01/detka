package detka

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thrawn01/detka/metrics"
)

func InternalError(resp http.ResponseWriter, msg string, fields logrus.Fields) {
	logrus.WithFields(fields).Error(msg)
	metrics.InternalErrors.With(ToLabels(fields)).Inc()
	resp.WriteHeader(http.StatusInternalServerError)
	resp.Write([]byte(fmt.Sprintf(`{"error": "%s"}`, http.StatusText(500))))
}

func BadRequest(resp http.ResponseWriter, msg string, fields logrus.Fields) {
	metrics.Non200Responses.With(ToLabels(fields)).Inc()
	resp.WriteHeader(http.StatusBadRequest)
	resp.Write([]byte(fmt.Sprintf(`{"error": "%s"}`, msg)))
}

func NotFound(resp http.ResponseWriter, msg string, fields logrus.Fields) {
	metrics.Non200Responses.With(ToLabels(fields)).Inc()
	resp.WriteHeader(http.StatusNotFound)
	resp.Write([]byte(fmt.Sprintf(`{"error": "%s"}`, msg)))
}

func ToLabels(tags logrus.Fields) prometheus.Labels {
	result := prometheus.Labels{}
	for key, value := range tags {
		result[key] = value.(string)
	}
	return result
}
