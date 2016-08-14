package store

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	internalErr   int = 1
	notFoundErr   int = 2
	connectionErr int = 3
)

type StoreError struct {
	Err  error
	Kind int
}

func (self *StoreError) Error() string {
	return self.Err.Error()
}

// Create a new store error from an error object
func FromError(kind int, err error, msg string, stuff ...interface{}) *StoreError {
	return &StoreError{
		Err:  errors.Wrap(err, fmt.Sprintf(msg, stuff...)),
		Kind: kind,
	}
}

// Create a new store error
func NewError(kind int, msg string, stuff ...interface{}) *StoreError {
	return &StoreError{
		Err:  errors.New(fmt.Sprintf(msg, stuff...)),
		Kind: kind,
	}
}

func GetStoreError(err error) *StoreError {
	obj, ok := err.(*StoreError)
	if !ok {
		return &StoreError{}
	}
	return obj
}

// Return true if the store error is a not found error
func IsNotFound(err error) bool {
	return GetStoreError(err).Kind == notFoundErr
}

// Return true if the store error a connection error
func IsConnectError(err error) bool {
	return GetStoreError(err).Kind == connectionErr
}
