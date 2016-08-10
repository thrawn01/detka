package detka

import "github.com/thrawn01/args"

type Mailer interface {
	Send(Message) error
}

type Mail struct {
	parser *args.ArgParser
}

func NewMailer(parser *args.ArgParser) *Mail {
	return &Mail{
		parser: parser,
	}
}

func (self *Mail) Send(msg Message) error {
	return nil
	// Attempt to send the message
	/*opts := parser.GetOpts()

	for i := 0; i < opts.Int("retry"); i++ {
		// Send message
		if err := sendEmail(msg); err != nil {
			log.WithFields(log.Fields{
				"method": "processMessage",
				"type":   "mail",
			}).Error("sendEmail - ", err.Error())
		}
		break
	}
	*/
}
