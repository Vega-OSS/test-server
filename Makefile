.PHONY: all build test check run clean docker-build

BIN_DIR := bin
BINARY := $(BIN_DIR)/testserver

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BINARY) cmd/testserver/main.go

test:
	go test -v ./...

check:
	go test -v -race ./...

run: build
	./$(BINARY) --port=:8081 --vega-grpc=127.0.0.1:50051 --vega-http=http://127.0.0.1:8080

clean:
	rm -rf $(BIN_DIR)
