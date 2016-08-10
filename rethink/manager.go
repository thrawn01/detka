package rethink

import (
	"sync"

	"time"

	"net/http"

	"strings"

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

var RunOpts = gorethink.RunOpts{
	Durability: "hard",
}

var ExecOpts = gorethink.ExecOpts{
	Durability: "hard",
}

func SetManager(ctx context.Context, manager *Manager) context.Context {
	return context.WithValue(ctx, rethinkManagerKey, manager)
}

func GetSession(ctx context.Context) *gorethink.Session {
	obj := GetManager(ctx)
	return obj.GetSession()
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
	logrus.Info("Connecting to Rethink Cluster ", opts.StringSlice("rethink-endpoints"))
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

	if opts.Bool("rethink-auto-create") {
		if !self.createDbIfNotExists(session, opts) {
			return false
		}

		if !self.createTablesIfNotExists(session) {
			return false
		}
	}

	self.new <- session
	return true
}

func (self *Manager) createDbIfNotExists(session *gorethink.Session, opts *args.Options) bool {
	err := gorethink.DBCreate(opts.String("rethink-db")).Exec(session, ExecOpts)
	return handleCreateError("createDbIfNotExists", err)
}
func (self *Manager) createTablesIfNotExists(session *gorethink.Session) bool {
	err := gorethink.TableCreate("messages", gorethink.TableCreateOpts{PrimaryKey: "Id"}).Exec(session, ExecOpts)
	return handleCreateError("createTablesIfNotExists", err)
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

func handleCreateError(context string, err error) bool {
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			logrus.WithFields(logrus.Fields{
				"method": context,
				"type":   "rethink",
			}).Error(err)
			return false
		}
		logrus.Debug(err)
	}
	return true
}
