.PHONY: all build install test fmt lint vet clean help

BINARY_NAME := watchmen
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
	GOBIN := $(shell go env GOPATH)/bin
endif

all: fmt vet test build

build:
	go build -o $(BINARY_NAME) .

install:
	go install .
	@echo "Installed to $(GOBIN)/$(BINARY_NAME)"
	@echo "Make sure $(GOBIN) is in your PATH"

test:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt:
	gofmt -s -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, running go vet only"; \
		go vet ./...; \
	fi

vet:
	go vet ./...

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

help:
	@echo "Available targets:"
	@echo "  all           - fmt, vet, test, build (default)"
	@echo "  build         - Build the binary"
	@echo "  install       - Install to $(GOBIN)"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  fmt           - Format code"
	@echo "  fmt-check     - Check if code is formatted"
	@echo "  lint          - Run golangci-lint (or vet if not installed)"
	@echo "  vet           - Run go vet"
	@echo "  clean         - Remove build artifacts"
	@echo "  help          - Show this help"
