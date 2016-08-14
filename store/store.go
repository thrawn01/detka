package store

import (
	"net/http"

	"github.com/dancannon/gorethink"
	"github.com/pkg/errors"
	"github.com/pressly/chi"
	"github.com/thrawn01/args"
	"github.com/thrawn01/detka/models"
	"github.com/thrawn01/detka/rethink"
	"golang.org/x/net/context"
)

type contextKey int

const (
	storeContextKey contextKey = 1
)

func SetStore(ctx context.Context, store Store) context.Context {
	return context.WithValue(ctx, storeContextKey, store)
}

func GetStore(ctx context.Context) Store {
	obj, ok := ctx.Value(storeContextKey).(Store)
	if !ok {
		panic("No store.Store found in context")
	}
	return obj
}

type Store interface {
	GetMessage(string) (*models.Message, error)
	InsertMessage(*models.Message) error
	UpdateMessage(string, map[string]interface{}) error
	SignalReconnect()
	Stop()
	IsConnected() bool
}

func NewRethinkStore(parser *args.ArgParser, manager *rethink.Manager) Store {
	if manager == nil {
		manager = rethink.NewManager(parser)
		manager.Start()
	}

	return &RethinkStore{
		manager: manager,
	}
}

type RethinkStore struct {
	manager *rethink.Manager
}

func (self *RethinkStore) GetMessage(id string) (*models.Message, error) {
	session := self.manager.GetSession()
	if session == nil {
		return nil, NewError(internalErr, "InsertMessage() Not Connected")
	}

	var message models.Message
	cursor, err := gorethink.Table("messages").Get(id).Run(session, rethink.RunOpts)
	if err != nil {
		return nil, errors.Wrap(err, "GetMessage() ")
	} else if err := cursor.One(&message); err != nil {
		if cursor.IsNil() {
			return nil, NewError(notFoundErr, "message id - %s not found", id)
		}
		return nil, FromError(internalErr, err, "Cursor.One() error")
	}
	return &message, nil
}

func (self *RethinkStore) InsertMessage(msg *models.Message) error {
	if len(msg.Id) == 0 {
		msg.Id = models.NewId()
	}
	session := self.manager.GetSession()
	if session == nil {
		return NewError(internalErr, "InsertMessage() Not Connected")
	}

	changed, err := gorethink.Table("messages").Insert(msg).RunWrite(session, rethink.RunOpts)
	if err != nil {
		return FromError(internalErr, err, "rethink.Insert() Error")
	} else if changed.Errors != 0 {
		return NewError(internalErr, "changed.Error != 0 - %s", changed.FirstError)
	}
	return nil
}

func (self *RethinkStore) UpdateMessage(id string, fields map[string]interface{}) error {
	session := self.manager.GetSession()
	if session == nil {
		return NewError(internalErr, "UpdateMessage() Not Connected")
	}

	changed, err := gorethink.Table("messages").Get(id).Update(fields).
		RunWrite(session, rethink.RunOpts)
	if err != nil {
		return FromError(internalErr, err, "rethink.Update()")
	} else if changed.Skipped != 0 {
		return NewError(notFoundErr, "Message Id - %s not found", id)
	}
	return nil
}

func (self *RethinkStore) SignalReconnect() {
	self.manager.SignalReconnect()
}

func (self *RethinkStore) IsConnected() bool {
	session := self.manager.GetSession()
	if session == nil || !session.IsConnected() {
		return false
	}
	return true
}

func (self *RethinkStore) Stop() {
	self.manager.Stop()
}

// Injects store.Store into the context.Context for each request
func Middleware(dbStore Store) func(chi.Handler) chi.Handler {
	return func(next chi.Handler) chi.Handler {
		return chi.HandlerFunc(func(ctx context.Context, resp http.ResponseWriter, req *http.Request) {
			ctx = SetStore(ctx, dbStore)
			next.ServeHTTPC(ctx, resp, req)
		})
	}
}
