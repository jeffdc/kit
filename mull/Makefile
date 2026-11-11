.PHONY: all build test race fmt fmt-check vet install clean

VERSION := $(shell git describe --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X mull/cmd.Version=$(VERSION)

all: fmt vet test build

build:
	go build -ldflags "$(LDFLAGS)" -o mull .

test:
	go test ./...

race:
	go test -race ./...

fmt:
	gofmt -s -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

vet:
	go vet ./...

install:
	go install -ldflags "$(LDFLAGS)" .
	@echo "Installed mull to $$(go env GOBIN || true)$$(test -n "$$(go env GOBIN)" || go env GOPATH)/bin"
	@# mull serve running as a LaunchAgent restarts on the new binary; open pages reload themselves.
	@if [ -f "$$HOME/Library/LaunchAgents/mull.serve.plist" ]; then \
		bin="$$(go env GOBIN)"; [ -n "$$bin" ] || bin="$$(go env GOPATH)/bin"; \
		"$$bin/mull" serve --install >/dev/null && echo "Restarted mull serve (LaunchAgent) on the new build"; \
	fi

clean:
	rm -f mull
