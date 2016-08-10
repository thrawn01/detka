package detka

import (
	"fmt"
	"net/mail"

	"github.com/pkg/errors"
)

func ValidEmail(address string) error {
	_, err := mail.ParseAddressList(address)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("'%s'", address))
	}
	return nil
}

func ValidMessageId(id string) error {
	if len(id) == 26 {
		return nil
	}
	return errors.New("Invalid Message ID - must be 26 characters")
}
