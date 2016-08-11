.PHONY: test all dist get-deps start-containers stop-containers create-topic
.DEFAULT_GOAL := all

# GO
GOPATH := $(shell go env | grep GOPATH | sed 's/GOPATH="\(.*\)"/\1/')
GLIDE := $(GOPATH)/bin/glide
PATH := $(GOPATH)/bin:$(PATH)
export $(PATH)

# Docker
export DETKA_DOCKER_HOST=$(shell hostname -I 2> /dev/null | awk '{ print $$1; exit }')
DOCKER_MACHINE_IP=$(shell docker-machine ip default 2> /dev/null)
ifneq ($(DOCKER_MACHINE_IP),)
	DETKA_DOCKER_HOST=$(DOCKER_MACHINE_IP)
endif

export KAFKA_ENDPOINTS=$(DETKA_DOCKER_HOST):9092
export RETHINK_ENDPOINTS=$(DETKA_DOCKER_HOST):28015

export WORKER_KAFKA_ENDPOINTS=$(KAFKA_ENDPOINTS)
export WORKER_RETHINK_ENDPOINTS=$(RETHINK_ENDPOINTS)

export API_KAFKA_ENDPOINTS=$(KAFKA_ENDPOINTS)
export API_RETHINK_ENDPOINTS=$(RETHINK_ENDPOINTS)

ETCD_DOCKER_IMAGE=quay.io/coreos/etcd:latest

start-containers:
	@echo Checking Docker Containers
	@if [ $(shell docker ps -a | grep -ci detka-zookeeper) -eq 0 ]; then \
		echo Starting Docker Container detka-zookeeper; \
		docker run -d --name detka-zookeeper --publish ${DETKA_DOCKER_HOST}:2181:2181 jplock/zookeeper:3.4.6; \
	elif [ $(shell docker ps | grep -ci detka-zookeeper) -eq 0 ]; then \
		echo restarting detka-zookeeper; \
		docker start detka-zookeeper > /dev/null; \
	fi
	@if [ $(shell docker ps -a | grep -ci detka-kafka) -eq 0 ]; then \
		echo Starting Docker Container detka-kafka; \
		docker run -d --hostname localhost \
		--name detka-kafka --publish ${DETKA_DOCKER_HOST}:9092:9092 --publish ${DETKA_DOCKER_HOST}:7203:7203 \
		--env KAFKA_ADVERTISED_HOST_NAME=${DETKA_DOCKER_HOST} --env ZOOKEEPER_IP=${DETKA_DOCKER_HOST} \
		ches/kafka; \
	elif [ $(shell docker ps | grep -ci detka-kafka) -eq 0 ]; then \
		echo restarting detka-kafka; \
		docker start detka-kafka > /dev/null; \
	fi
	@if [ $(shell docker ps -a | grep -ci detka-rethink) -eq 0 ]; then \
		echo Starting Docker Container detka-rethinkdb; \
		docker run --name detka-rethinkdb -p ${DETKA_DOCKER_HOST}:8080:8080 -p ${DETKA_DOCKER_HOST}:28015:28015 -d rethinkdb:latest; \
	elif [ $(shell docker ps | grep -ci detka-rethink) -eq 0 ]; then \
		echo restarting detka-rethinkdb; \
		docker start detka-rethinkdb > /dev/null; \
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
	@if [ $(shell docker ps -a | grep -ci detka-rethinkdb) -eq 1 ]; then \
		echo Stopping Container detka-rethindbk; \
		docker stop detka-rethinkdb > /dev/null; \
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

bin/worker: cmd/worker.go
	go build -o bin/worker cmd/worker.go

all: bin/api bin/worker

run-worker: bin/worker
	@echo "Running bin/worker with the following endpoints..."
	@echo WORKER_KAFKA_ENDPOINTS=$(WORKER_KAFKA_ENDPOINTS)
	@echo WORKER_RETHINK_ENDPOINTS=$(WORKER_RETHINK_ENDPOINTS)
	bin/worker -c etc/worker.ini -d

run-api: bin/api
	@echo "Running bin/worker with the following endpoints..."
	@echo API_KAFKA_ENDPOINTS=$(API_KAFKA_ENDPOINTS)
	@echo API_RETHINK_ENDPOINTS=$(API_RETHINK_ENDPOINTS)
	bin/api -d

clean:
	rm bin/*

travis-ci: get-deps start-containers
	go get -u github.com/mattn/goveralls
	go get -u golang.org/x/tools/cmd/cover
	goveralls -service=travis-ci

test: get-deps start-containers
	go test $(go list ./... | grep -v /vendor/)
