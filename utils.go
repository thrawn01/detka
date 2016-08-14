package detka

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

func LoadFile(fileName string) ([]byte, error) {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read '%s'", fileName)
	}
	return content, nil
}

func ToJson(resp http.ResponseWriter, payload interface{}) {
	if err := json.NewEncoder(resp).Encode(payload); err != nil {
		InternalError(resp, err.Error(), logrus.Fields{"method": "ToJson", "type": "json"})
	}
}

func FromJson(payload []byte) {
}
