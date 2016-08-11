package detka

import (
	"io/ioutil"

	"github.com/pkg/errors"
)

func LoadFile(fileName string) ([]byte, error) {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read '%s'", fileName)
	}
	return content, nil
}
