.PHONY: all build test race fmt fmt-check vet install clean dev dev-stop dev-demo dev-reset

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

# A scratch mull serve for trying the UI: this tree's build, on DEV_PORT, serving a seeded demo repo, with its own config dir so the real registry and last-looked state are never touched. Everything lives in .dev/ (ignored). It's stopped through the PID file it writes, so it can only ever stop itself.
DEV := $(CURDIR)/.dev
DEV_PORT ?= 6899

dev: build dev-demo dev-stop
	@cd $(DEV)/demo && { XDG_CONFIG_HOME=$(DEV)/cfg $(CURDIR)/mull serve --port $(DEV_PORT) >$(DEV)/serve.log 2>&1 </dev/null & echo $$! >$(DEV)/serve.pid; }
	@for i in $$(seq 50); do curl -sf localhost:$(DEV_PORT)/api/build >/dev/null && break; sleep 0.1; done
	@curl -sf localhost:$(DEV_PORT)/api/build >/dev/null || { echo "mull serve didn't start; see $(DEV)/serve.log"; exit 1; }
	@echo "mull serve (dev) on http://localhost:$(DEV_PORT)/#/demo, pid $$(cat $(DEV)/serve.pid), log $(DEV)/serve.log"

dev-stop:
	@if [ -f $(DEV)/serve.pid ]; then \
		pid=$$(cat $(DEV)/serve.pid); kill $$pid 2>/dev/null || true; rm -f $(DEV)/serve.pid; \
		for i in $$(seq 50); do kill -0 $$pid 2>/dev/null || break; sleep 0.1; done; \
	fi

dev-demo: build
	@./scripts/dev-demo.sh $(DEV) $(CURDIR)/mull

# dev-reset starts again from a fresh demo repo.
dev-reset: dev-stop
	rm -rf $(DEV)
	@$(MAKE) --no-print-directory dev
