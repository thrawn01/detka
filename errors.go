package detka

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thrawn01/detka/metrics"
)

func InternalError(resp http.ResponseWriter, err error, fields logrus.Fields) {
	logrus.WithFields(fields).Error(err.Error())
	metrics.InternalErrors.With(ToLabels(fields)).Inc()
	resp.WriteHeader(http.StatusInternalServerError)
	resp.Write([]byte(fmt.Sprintf(`{"error": "%s"}`, http.StatusText(500))))
}

func BadRequest(resp http.ResponseWriter, err error, fields logrus.Fields) {
	logrus.WithFields(fields).Error(err.Error())
	metrics.InternalErrors.With(ToLabels(fields)).Inc()

	obj := map[string]string{
		"error": err.Error(),
	}
	payload, err := json.Marshal(obj)
	if err != nil {
		logrus.WithFields(logrus.Fields{"method": "BadRequest", "type": "internal"}).
			Error("json.Marshal() failed on '%+v' with '%s'", obj, err.Error())
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(`{"error": "Internal Server Error"}`))
		return
	}
	resp.WriteHeader(http.StatusBadRequest)
	resp.Write(payload)
}

func ToLabels(tags logrus.Fields) prometheus.Labels {
	result := prometheus.Labels{}
	for key, value := range tags {
		result[key] = value.(string)
	}
	return result
}
