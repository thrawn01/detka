.PHONY: test all dist
.DEFAULT_GOAL := all

bin/api: cmd/api.go
	go build -o bin/api cmd/api.go

all: bin/api

clean:
	rm bin/*

travis-ci: get-deps
	go get -u github.com/mattn/goveralls
	go get -u golang.org/x/tools/cmd/cover
	goveralls -service=travis-ci

glide:
	@if [ ! -e $(GOPATH)/bin ] ; then mkdir $(GOPATH)/bin ; fi
	@which glide > /dev/null ; if [ $$? -eq 1 ] ; then curl https://glide.sh/get | sh ; fi

get-deps: glide
	glide install