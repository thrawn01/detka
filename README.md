[![Coverage Status](https://img.shields.io/coveralls/thrawn01/detka.svg)](https://coveralls.io/github/thrawn01/detka)
[![Build Status](https://img.shields.io/travis/thrawn01/detka/master.svg)](https://travis-ci.org/thrawn01/detka)

## Introduction
Detka provides an HTTP API for sending emails!

## Architecture
Detka architecture consits of the following

 - Detka API - This is a go based http api that provides a /messages endpoint for creating
 and querying the status of queued messages.
 - Detka Worker - This is a go based service that listens for new messages on a kafka queue and
 dispatches the message via the chosen backend.

 ## Supporting Infrastructure
 - Rethinkdb - This is the database backend that stores the messages
 - Kafka - This is the queue used to send API created messages to the workers

## Features
- Detka Worker supports 2 backends, one for sending mail via the [mailgun](http://www.mailgun.com/)
API and another for sending via SMTP.
- Provides a /healthz endpoint that allows operations to interrogate the service. If the service
is ready for operation (IE: it has a connection to the database and the queue) the /healthz
endpoint will return 200, else it returns 500
- Provides a /metrics endpoint that allows operations to build graphs based on statistics exposed
- Both the API and Worker support hot reload of the config in the event the config file is modified.
This can be further enhanced to use etcd and eventually zookeeper for hot reload (but I ran out of time)

## Setup
Requires a go 1.5 or 1.6 installation with $GOPATH setup
```
go get -d github.com/thrawn01/detka
cd $GOPATH/src/github.com/thrawn01/detka
git checkout --track remotes/origin/baby-mailgun
```
## Run Tests
The project consists of a few functional tests and requires docker or boot2docker to be
 installed. To download the required docker images, install build dependencies and run
 the tests use run the following.
```
make test
```

## Build the binaries
This will make the ```bin/api``` and ```bin/worker```
```
make get-deps
make
```

## Run the API
First modify the config file in etc/api.ini then run
```
bin/api -c etc/api.ini
```

## Run the Worker
The worker requires a bit more configuration, you need to choose which backend mailer and configure
that by editing the etc/worker.ini config file. Once that is done, you can run the following.
```
bin/worker -c etc/worker.ini
```

## Create a new message
```
$ curl -X POST http://localhost:4040/messages \
    -d from='Excited User <excited@samples.mailgun.org>' \
    -d to='devs@mailgun.net' \
    -d subject='Hello' \
    -d text='Testing some Mailgun awesomeness!'
{"id":"AL3UDCVPMJDAFFNIO2OP4IYQKE","message":"Queued, Thank you."}
```
Get the status of the message
```
$ curl http://localhost:4040/messages/AL3UDCVPMJDAFFNIO2OP4IYQKE
```

## Outstanding issues
- If the queue is down, with messages pending, messages can be lost
- What happens if the worker dies with a message queued in the consumer channel? How do we recover?
- No API Throttling
- No Authentication
- Should log send errors into the database so the user can retrieve them
- The Connection Managers reconnect on any sort of error, we should only reconnect on terminated errors
- Should use the repository pattern for db abstraction, but I got distracted playing with Connection Managers
