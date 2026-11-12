.PHONY: all build test race fmt fmt-check vet install clean dev dev-stop dev-demo dev-reset bench

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

# The code-signing identity make install signs with on macOS: by default the first "Apple Development" identity in the keychain. Go's own signature is ad-hoc, a new identity every build, so macOS would announce the LaunchAgent as a new background item after each install; a real identity and a fixed identifier keep it the same item. Empty (none found, or SIGN_ID=) skips signing.
SIGN_ID ?= $(shell security find-identity -v -p codesigning 2>/dev/null | awk '/"Apple Development/ {print $$2; exit}')

install:
	go install -ldflags "$(LDFLAGS)" .
	@bin="$$(go env GOBIN)"; [ -n "$$bin" ] || bin="$$(go env GOPATH)/bin"; \
	echo "Installed mull to $$bin"; \
	if [ -n "$(SIGN_ID)" ]; then \
		codesign -f -s "$(SIGN_ID)" -i mull "$$bin/mull" 2>/dev/null && echo "Signed it as mull" || echo "Couldn't sign it; macOS may announce mull as a new background item"; \
	fi
	@# mull serve running as a LaunchAgent restarts on the new binary, without rewriting its plist; open pages reload themselves.
	@if [ -f "$$HOME/Library/LaunchAgents/mull.serve.plist" ]; then \
		bin="$$(go env GOBIN)"; [ -n "$$bin" ] || bin="$$(go env GOPATH)/bin"; \
		{ launchctl kickstart -k "gui/$$(id -u)/mull.serve" 2>/dev/null || "$$bin/mull" serve --install >/dev/null; } && echo "Restarted mull serve (LaunchAgent) on the new build"; \
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

# bench times every page mull serve shows, in-process and over HTTP, with git calls counted: on this repository's own mull branch (read only) and on a seeded one of BENCH_SIZE (small, medium or large), kept in .dev/bench/ and reused. BENCH_FLAGS passes flags through, e.g. BENCH_FLAGS="-url http://127.0.0.1:6855" to time the installed server too, or -json.
BENCH_SIZE ?= large
BENCH_REPOS ?= $(shell git rev-parse --show-toplevel)
BENCH_FLAGS ?=

bench:
	@go run ./scripts/bench seed -size $(BENCH_SIZE) -dir $(DEV)/bench/$(BENCH_SIZE)
	@go run ./scripts/bench run $(BENCH_FLAGS) $(BENCH_REPOS) $(DEV)/bench/$(BENCH_SIZE)
