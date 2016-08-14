package models

import (
	"bytes"
	"encoding/base32"

	"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

type NewMessageResponse struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

type Message struct {
	Id      string `json:"id"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	From    string `json:"from"`
	To      string `json:"recipients"`
	Status  string `json:"status"`
}

type QueueMessage struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}

// After marshaling from JSON, call this method to validate the object is intact
func (self *Message) Validate() error {

	if err := ValidEmail(self.From); err != nil {
		return errors.Wrap(err, "From")
	}

	if err := ValidEmail(self.To); err != nil {
		return errors.Wrap(err, "To")
	}

	return nil
}

func NewId() string {
	var buf bytes.Buffer
	encoder := base32.NewEncoder(base32.StdEncoding, &buf)
	encoder.Write(uuid.NewRandom())
	encoder.Close()
	buf.Truncate(26) // removes the '==' padding
	return buf.String()
}
