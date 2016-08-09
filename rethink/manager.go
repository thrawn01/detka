package rethink

import (
	"sync"

	"time"

	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/dancannon/gorethink"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
	"golang.org/x/net/context"
)

type contextKey int

const (
	rethinkManagerKey contextKey = 1
)

func SetManager(ctx context.Context, manager *Manager) context.Context {
	return context.WithValue(ctx, rethinkManagerKey, manager)
}

func GetManager(ctx context.Context) *Manager {
	obj, ok := ctx.Value(rethinkManagerKey).(*Manager)
	if !ok {
		panic("No rethink.Manager found in context")
	}
	return obj
}

type Manager struct {
	done      chan struct{}
	parser    *args.ArgParser
	current   chan *gorethink.Session
	new       chan *gorethink.Session
	reconnect chan bool
}

func NewManager(parser *args.ArgParser) *Manager {
	manager := &Manager{
		parser: parser,
	}
	manager.Start()
	return manager
}

func (self *Manager) GetSession() *gorethink.Session {
	return <-self.current
}

func (self *Manager) SignalReconnect() {
	self.reconnect <- true
}

func (self *Manager) Start() {
	self.current = make(chan *gorethink.Session)
	self.new = make(chan *gorethink.Session)
	self.reconnect = make(chan bool)
	self.done = make(chan struct{})

	// Always feed clients the latest rethink session
	go func() {
		defer close(self.current)
		var current *gorethink.Session

		for {
			select {
			case self.current <- current:
			case current = <-self.new:
			case <-self.done:
				return
			}
		}
	}()

	var attemptedConnect sync.WaitGroup

	// Waits until it receives a list of endpoints, then attempts to connect
	// if it fails will sleep for 1 second and try again
	go func() {
		defer close(self.new)
		var once sync.Once
		attemptedConnect.Add(1)

		for {
			var endpoints []string
			var timer <-chan time.Time

			select {
			case <-self.reconnect:
				goto connect
			case <-timer:
				goto connect
			case <-self.done:
				return
			}
		connect:
			logrus.Info("Connecting to Rethink Cluster ", endpoints)
			// Attempt to connect, if we fail to connect, set a timer to try again
			if !self.connect() {
				timer = time.NewTimer(time.Second).C
			}
			once.Do(func() { attemptedConnect.Done() })
		}
	}()

	// Send the first connect signal
	self.SignalReconnect()
	// Attempt to connect at least once before we leave Start()
	attemptedConnect.Wait()
}

func (self *Manager) connect() bool {
	opts := self.parser.GetOpts()

	// Attempt to connect to rethinkdb
	session, err := gorethink.Connect(gorethink.ConnectOpts{
		Addresses: opts.StringSlice("rethink-endpoints"),
		Database:  opts.String("rethink-db"),
		Username:  opts.String("rethink-user"),
		Password:  opts.String("rethink-password"),
	})
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"type":   "rethink",
			"method": "Manager.connect()",
		}).Errorf("Rethinkdb Connect Failed - %s", err.Error())
		return false
	}
	self.new <- session
	return true
}

func (self *Manager) Stop() {
	session := self.GetSession()
	if session != nil {
		session.Close()
	}
	close(self.done)
}

// Injects rethink.Manager into the context.Context for each request
func Middleware(manager *Manager) func(chi.Handler) chi.Handler {
	return func(next chi.Handler) chi.Handler {
		return chi.HandlerFunc(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			ctx = SetManager(ctx, manager)
			next.ServeHTTPC(ctx, resp, req)
		})
	}
}
