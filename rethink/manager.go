package rethink

import (
	"time"

	"net/http"

	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/dancannon/gorethink"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka/connection"
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
	*connection.Manager
	parser  *args.ArgParser
	session *gorethink.Session
}

func NewManager(parser *args.ArgParser) *Manager {
	me := &Manager{
		&connection.Manager{},
		parser,
		nil,
	}
	me.Start()
	return me
}

func (self *Manager) GetSession() (result *gorethink.Session) {
	self.WithLock(func() {
		result = self.session
	})
	return
}

func (self *Manager) Start() {
	// run connect(), if it fails try again every 2 seconds
	self.Begin(self.connect, time.Second*2)
}

func (self *Manager) Stop() {
	session := self.GetSession()
	if session != nil {
		session.Close()
	}
	self.End()
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

	self.WithLock(func() {
		self.session = session
	})
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
