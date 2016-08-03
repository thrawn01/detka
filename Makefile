.PHONY: test all dist get-deps
.DEFAULT_GOAL := all

# GO Dependencies
GOPATH := $(shell go env | grep GOPATH | sed 's/GOPATH="\(.*\)"/\1/')
GLIDE := $(GOPATH)/bin/glide
PATH := $(GOPATH)/bin:$(PATH)
export $(PATH)

$(GLIDE):
	go get -u github.com/Masterminds/glide

get-deps: $(GLIDE)
	$(GLIDE) install

bin/api: cmd/api.go
	go build -o bin/api cmd/api.go

all: bin/api

clean:
	rm bin/*

travis-ci: get-deps
	go get -u github.com/mattn/goveralls
	go get -u golang.org/x/tools/cmd/cover
	goveralls -service=travis-ci

test:
	go test $(go list ./... | grep -v /vendor/)

