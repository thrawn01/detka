package detka

import (
	"github.com/Sirupsen/logrus"
	"github.com/mailgun/mailgun-go"
	"github.com/thrawn01/args"
)

type Mailer interface {
	Send(Message) error
}

type Mailgun struct {
	parser  *args.ArgParser
	mailgun mailgun.Mailgun
}

func NewMailer(parser *args.ArgParser) Mailer {
	// TODO: We could have an option to return an SMTP implementation instead of mailgun
	return &Mailgun{
		parser: parser,
	}
}

func (self *Mailgun) Send(msg Message) error {
	opts := self.parser.GetOpts()

	mail := mailgun.NewMailgun(
		opts.String("mailgun-domain"),
		opts.String("mailgun-api-key"),
		opts.String("mailgun-public-api-key"))

	newMessage := mail.NewMessage(
		msg.From,
		msg.Subject,
		msg.Text,
		msg.To,
	)

	var err error
	for i := 0; i < opts.Int("mailgun-retry"); i++ {
		var id string
		_, id, err = self.mailgun.Send(newMessage)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"method": "Send()",
				"type":   "mailgun",
			}).Error("sendEmail - ", err.Error())
			continue
		}
		logrus.Debug("Message id=%s", id)
		return nil
	}
	// TODO: This only returns the final error, should probably return all the errors?
	return err
}
