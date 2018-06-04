CLI_NAME:='dama'
SERVER_NAME:='dama-proxy'
VERSION:=$(shell date "+%Y%m%d")
LDFLAGS:='-X "main.version=$(VERSION)" -s -w'

default: build

run:
	sudo -E ./dama-proxy

cli:
	go build -ldflags $(LDFLAGS) -o $(CLI_NAME) ./cmd/cli

server:
	go build -ldflags $(LDFLAGS) -tags=jsoniter -o $(SERVER_NAME) .

build:
	go build -ldflags $(LDFLAGS) -tags=jsoniter -o $(SERVER_NAME) .
	go build -ldflags $(LDFLAGS) -o $(CLI_NAME) ./cmd/cli

buildall:
	GOOS=linux go build -ldflags $(LDFLAGS) -tags=jsoniter -o $(SERVER_NAME) .
	GOOS=linux go build -ldflags $(LDFLAGS) -o $(CLI_NAME) ./cmd/cli
	GOOS=darwin go build -ldflags $(LDFLAGS) -o $(CLI_NAME)-darwin ./cmd/cli
	GOOS=windows go build -ldflags $(LDFLAGS) -o $(CLI_NAME)-windows ./cmd/cli
