.PHONY: all build install test fmt vet clean help

BINARY_NAME := flint
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
	GOBIN := $(shell go env GOPATH)/bin
endif

all: fmt vet test build

build:
	go build -o $(BINARY_NAME) .

install:
	go install .
	@echo "Installed $(BINARY_NAME) to $(GOBIN)"
	@echo "Make sure $(GOBIN) is in your PATH"

test:
	go test -v ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

clean:
	rm -f $(BINARY_NAME)

help:
	@echo "Available targets:"
	@echo "  all     - fmt, vet, test, build (default)"
	@echo "  build   - Build the binary"
	@echo "  install - Install to $(GOBIN)"
	@echo "  test    - Run tests"
	@echo "  fmt     - Format code"
	@echo "  vet     - Run go vet"
	@echo "  clean   - Remove build artifacts"
	@echo "  help    - Show this help"
