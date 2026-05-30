# projectManager Makefile

GO ?= go
SERVER_BIN := bin/projectManagerSvr

.PHONY: tidy build run test fmt clean

tidy:
	$(GO) mod tidy

build:
	$(GO) build -o $(SERVER_BIN) ./cmd/server

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf bin