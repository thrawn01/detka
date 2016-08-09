package rethink

import (
	"sync"

	"time"

	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/dancannon/gorethink"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
	"golang.org/x/net/context"
)

type contextKey int

const (
	rethinkContextKey contextKey = 1
)

func SetContext(ctx context.Context, kafkaCtx *Context) context.Context {
	return context.WithValue(ctx, rethinkContextKey, kafkaCtx)
}

func GetContext(ctx context.Context) *Context {
	obj, ok := ctx.Value(rethinkContextKey).(*Context)
	if !ok {
		panic("No rethink.Context found in context")
	}
	return obj
}

type Context struct {
	done      chan struct{}
	parser    *args.ArgParser
	current   chan *gorethink.Session
	new       chan *gorethink.Session
	reconnect chan bool
}

func NewContext(parser *args.ArgParser) *Context {
	ctx := &Context{
		parser: parser,
	}
	ctx.Start()
	return ctx
}

func (self *Context) Get() *gorethink.Session {
	return <-self.current
}

// Tell the context goroutine to start reconnecting
func (self *Context) SignalReconnect() {
	self.reconnect <- true
}

// Start 2 goroutines, the first one provides the current rethink session
// the second connects or reconnects to the rethink cluster
func (self *Context) Start() {
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
			log.Info("Connecting to Rethink Cluster ", endpoints)
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

func (self *Context) connect() bool {
	opts := self.parser.GetOpts()

	// Attempt to connect to rethinkdb
	session, err := gorethink.Connect(gorethink.ConnectOpts{
		Addresses: opts.StringSlice("rethink-endpoints"),
		Database:  opts.String("rethink-db"),
		Username:  opts.String("rethink-user"),
		Password:  opts.String("rethink-password"),
	})
	if err != nil {
		log.WithFields(log.Fields{
			"type":   "rethink",
			"method": "Context.connect()",
		}).Errorf("Rethinkdb Connect Failed - %s", err.Error())
		return false
	}
	self.new <- session
	return true
}

func (self *Context) Stop() {
	session := self.Get()
	if session != nil {
		session.Close()
	}
	close(self.done)
}

// Injects rethink.Context into the context.Context for each request
func Middleware(rethinkCtx *Context) func(chi.Handler) chi.Handler {
	return func(next chi.Handler) chi.Handler {
		return chi.HandlerFunc(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			ctx = SetContext(ctx, rethinkCtx)
			next.ServeHTTPC(ctx, resp, req)
		})
	}
}
