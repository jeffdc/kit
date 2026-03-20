.PHONY: all build build-tui install test fmt lint vet clean help deploy sync

BINARY_NAME := forage
TUI_BINARY := forage-tui
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
	GOBIN := $(shell go env GOPATH)/bin
endif

all: fmt vet test build

build:
	go build -o $(BINARY_NAME) .

build-tui:
	go build -o $(TUI_BINARY) ./cmd/forage-tui

install:
	go install .
	go install ./cmd/forage-tui
	@echo "Installed $(BINARY_NAME) and $(TUI_BINARY) to $(GOBIN)"
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

deploy: install
	GOOS=linux GOARCH=amd64 go build -o /tmp/forage-server ./cmd/forage-server
	cat /tmp/forage-server | sprite exec -s forage bash -c "cat > /home/sprite/forage-server && chmod +x /home/sprite/forage-server"
	forage export -o /tmp/forage-pwa
	for f in /tmp/forage-pwa/*; do \
		cat "$$f" | sprite exec -s forage bash -c "cat > /home/sprite/www/$$(basename $$f)"; \
	done
	@echo "Deployed to https://forage-4pbc.sprites.app"
	@echo "Note: restart the Sprite service if the server binary changed"

sync: install
	@test -n "$(FILE)" || (echo "Usage: make sync FILE=path/to/forage-changes.json"; exit 1)
	forage import "$(FILE)" --changes
	$(MAKE) deploy

clean:
	rm -f $(BINARY_NAME) $(TUI_BINARY)
	rm -f coverage.out coverage.html
	rm -rf /tmp/forage-pwa

help:
	@echo "Available targets:"
	@echo "  all           - fmt, vet, test, build (default)"
	@echo "  build         - Build the binary"
	@echo "  build-tui     - Build the TUI binary"
	@echo "  install       - Install to $(GOBIN)"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  fmt           - Format code"
	@echo "  fmt-check     - Check if code is formatted"
	@echo "  lint          - Run golangci-lint (or vet if not installed)"
	@echo "  vet           - Run go vet"
	@echo "  deploy        - Build server, export PWA, deploy to Sprite"
	@echo "  sync          - Import PWA changes and redeploy (FILE=path/to/changes.json)"
	@echo "  clean         - Remove build artifacts"
	@echo "  help          - Show this help"
