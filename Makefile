.PHONY: test all dist get-deps
.DEFAULT_GOAL := all

# GO
GOPATH := $(shell go env | grep GOPATH | sed 's/GOPATH="\(.*\)"/\1/')
GLIDE := $(GOPATH)/bin/glide
PATH := $(GOPATH)/bin:$(PATH)
export $(PATH)

# Docker
export DETKA_DOCKER_HOST=localhost
DOCKER_MACHINE_IP=$(shell docker-machine ip default)
ifneq ($(DOCKER_MACHINE_IP),)
	DETKA_DOCKER_HOST=$(DOCKER_MACHINE_IP)
endif

ETCD_DOCKER_IMAGE=quay.io/coreos/etcd:latest

start-containers:
	@echo Checking Docker Containers
	@if [ $(shell docker ps -a | grep -ci detka-zookeeper) -eq 0 ]; then \
		echo Starting Docker Container detka-zookeeper; \
		docker run -d --name detka-zookeeper --publish 2181:2181 jplock/zookeeper:3.4.6; \
	elif [ $(shell docker ps | grep -ci args-etcd) -eq 0 ]; then \
		echo restarting detka-zookeeper; \
		docker start detka-zookeeper > /dev/null; \
	fi
	@if [ $(shell docker ps -a | grep -ci detka-kafka) -eq 0 ]; then \
		echo Starting Docker Container detka-kafka; \
		docker run -d --hostname localhost \
		--name detka-kafka --publish 9092:9092 --publish 7203:7203 \
		--env KAFKA_ADVERTISED_HOST_NAME=${DETKA_DOCKER_HOST} --env ZOOKEEPER_IP=${DETKA_DOCKER_HOST} \
		ches/kafka; \
	elif [ $(shell docker ps | grep -ci args-etcd) -eq 0 ]; then \
		echo restarting detka-kafka; \
		docker start detka-kafka > /dev/null; \
	fi

stop-containers:
	@if [ $(shell docker ps -a | grep -ci detka-zookeeper) -eq 1 ]; then \
		echo Stopping Container detka-zookeeper; \
		docker stop detka-zookeeper > /dev/null; \
	fi
	@if [ $(shell docker ps -a | grep -ci detka-zookeeper) -eq 1 ]; then \
		echo Stopping Container detka-kafka; \
		docker stop detka-kafka > /dev/null; \
	fi

create-topic:
	@docker run --rm ches/kafka kafka-topics.sh \
	--create --topic detka-topic --replication-factor 1 \
	--partitions 1 --zookeeper ${DETKA_DOCKER_HOST}:2181

describe-topic:
	@docker run --rm ches/kafka kafka-topics.sh \
	--describe -topic detka-topic --zookeeper ${DETKA_DOCKER_HOST}:2181

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

