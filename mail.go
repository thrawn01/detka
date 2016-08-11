package detka

import (
	"fmt"

	"net/smtp"

	"time"

	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/mailgun/mailgun-go"
	"github.com/pkg/errors"
	"github.com/thrawn01/args"
)

type Mailer interface {
	Send(Message) error
}

func NewMailer(parser *args.ArgParser) (Mailer, error) {
	opts := parser.GetOpts()
	if opts.String("mail-transport") == "mailgun" {
		required := []string{"mailgun-domain", "mailgun-api-key", "mailgun-public-key"}
		if err := opts.Required(required); err != nil {
			return nil, errors.New(
				fmt.Sprintf("'%s' option is required when 'mailgun' is transport", err.Error()))
		}
		return &Mailgun{
			parser: parser,
		}, nil
	}
	required := []string{"smtp-user", "smtp-server", "smtp-password"}
	if err := opts.Required(required); err != nil {
		return nil, errors.New(
			fmt.Sprintf("'%s' option required when 'smtp' is transport", err.Error()))
	}
	return &Smtp{
		parser: parser,
	}, nil
}

type Mailgun struct {
	parser *args.ArgParser
}

func (self *Mailgun) Send(msg Message) error {
	opts := self.parser.GetOpts()

	mail := mailgun.NewMailgun(
		opts.String("mailgun-domain"),
		opts.String("mailgun-api-key"),
		opts.String("mailgun-public-key"))

	newMessage := mail.NewMessage(
		msg.From,
		msg.Subject,
		msg.Text,
		msg.To,
	)

	var err error
	for i := 0; i < opts.Int("transport-retry"); i++ {
		var id string
		_, id, err = mail.Send(newMessage)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"method": "Send()",
				"type":   "mailgun",
			}).Error("sendEmail - ", err.Error())
			time.Sleep(time.Second)
			continue
		}
		logrus.Debug("Message id=%s", id)
		return nil
	}
	// TODO: This only returns the final error, should probably return all the errors?
	return err
}

type Smtp struct {
	parser *args.ArgParser
}

func (self *Smtp) Send(msg Message) error {
	opts := self.parser.GetOpts()
	server := opts.String("smtp-server")

	// Trim the port number `smtp.example.com:25` for authentication
	authServer := server[0:strings.LastIndex(server, ":")]
	auth := smtp.PlainAuth("", opts.String("smtp-user"),
		opts.String("smtp-password"), authServer)

	// Construct a body
	body := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", msg.To, msg.Subject, msg.Text))

	var err error
	for i := 0; i < opts.Int("transport-retry"); i++ {

		err = smtp.SendMail(server, auth,
			msg.From, []string{msg.To}, body)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"method": "Send()",
				"type":   "smtp",
			}).Error("sendEmail - ", err.Error())
			time.Sleep(time.Second)
			continue
		}
	}
	// TODO: This only returns the final error, should probably return all the errors?
	return err
}
