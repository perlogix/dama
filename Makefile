CLI_NAME:='dama'
SERVER_NAME:='dama-proxy'
VERSION:=$(shell date "+%Y%m%d")
PACKAGES:=$(shell go list ./... | grep -v /vendor/)
LDFLAGS:='-s -w -X "main.version=$(VERSION)"'

define config
images: ["perlogix/minimal:latest"]
expire: "1300"
deployexpire: "86400"
uploadsize: 2000000000
envsize: 20
https:
  listen: "0.0.0.0"
  port: "8443"
  pem: "./dama.pem"
  key: "./dama.key"
  debug: false
  verifytls: false
db:
  network: "unix"
  address: "./tmp/redis.sock"
  db: 0
  maxretries: 20
docker:
  endpoint: "unix:///var/run/docker.sock"
  cpushares: 512
  memory: 1073741824
gotty:
  tls: false
endef
export config

default: build

run:
	sudo -E ./dama-proxy

cli:
	go build -ldflags $(LDFLAGS) -o $(CLI_NAME) ./cmd/cli

server:
	CGO_ENABLED=0 GOOS=linux go build -ldflags $(LDFLAGS) -a -installsuffix cgo -tags=jsoniter -o $(SERVER_NAME) .

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags $(LDFLAGS) -a -installsuffix cgo -tags=jsoniter -o $(SERVER_NAME) .
	go build -ldflags $(LDFLAGS) -o $(CLI_NAME) ./cmd/cli

buildall:
	CGO_ENABLED=0 GOOS=linux go build -ldflags $(LDFLAGS) -a -installsuffix cgo -tags=jsoniter -o $(SERVER_NAME) .
	GOOS=linux go build -ldflags $(LDFLAGS) -o $(CLI_NAME) ./cmd/cli
	GOOS=darwin go build -ldflags $(LDFLAGS) -o $(CLI_NAME)-darwin ./cmd/cli
	GOOS=windows go build -ldflags $(LDFLAGS) -o $(CLI_NAME)-windows ./cmd/cli

gofmt:
	go fmt ./...

lint: gofmt
	$(GOPATH)/bin/golint $(PACKAGES)
	$(GOPATH)/bin/golangci-lint run
	$(GOPATH)/bin/gosec -quiet -no-fail ./...

update-deps:
	go get -u ./...
	go mod tidy

certs:
	openssl req -subj '/CN=dama/O=dama/C=US' -new -newkey rsa:2048 -sha256 -days 365 -nodes -x509 -keyout dama.key -out dama.pem

config:
	@echo "$$config" > config.yml

docker:
	docker build -t perlogix/dama:latest .