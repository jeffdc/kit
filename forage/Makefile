.PHONY: all build test test-coverage fmt fmt-check vet lint run dev-data wake backup deploy clean help

# Last commit touching this folder, plus -dirty if this folder has uncommitted changes.
VERSION := $(shell git log -1 --format=%h -- . 2>/dev/null || echo dev)$(shell test -z "$$(git status --porcelain -- . 2>/dev/null)" || echo -dirty)
LDFLAGS := -X main.version=$(VERSION)

SPRITE    := sprite exec -s forage
SITE      := https://forage-4pbc.sprites.app
REMOTE    := /home/sprite/forage-server
REMOTE_DB := /home/sprite/forage/forage.db

DEV_DIR  := .dev
DEV_PORT := 8091

all: fmt vet test build

build:
	go build -ldflags "$(LDFLAGS)" -o forage-server ./cmd/forage-server

test:
	go test ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt:
	gofmt -s -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

vet:
	go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else go vet ./...; fi

# Local server on :$(DEV_PORT) with API key "dev". PWA files are served from
# disk, so edits to internal/pwa/assets show on reload without a rebuild.
run:
	@test -f $(DEV_DIR)/forage.db || echo "No dev database; starting empty (run 'make dev-data' to copy the latest backup)."
	FORAGE_DIR=$(DEV_DIR) FORAGE_API_KEY=dev FORAGE_DEV_WWW=internal/pwa/assets PORT=$(DEV_PORT) \
		go run -ldflags "$(LDFLAGS)" ./cmd/forage-server

# Copy the newest local backup into the dev data directory (overwrites it).
dev-data:
	@latest=$$(ls -t forage-backup-*.db 2>/dev/null | head -1); \
	test -n "$$latest" || { echo "No forage-backup-*.db found; run 'make backup' first."; exit 1; }; \
	mkdir -p $(DEV_DIR) && cp "$$latest" $(DEV_DIR)/forage.db && echo "Dev data: $$latest"

# The Sprite sleeps when idle; any HTTP request wakes it.
wake:
	@curl -s -m 60 -o /dev/null $(SITE)/api/health || true

backup: wake
	@set -eu; \
	backup="forage-backup-$$(date -u +%Y%m%dT%H%M%SZ).db"; \
	$(SPRITE) sqlite3 $(REMOTE_DB) ".timeout 10000" ".backup '/tmp/forage-backup.db'"; \
	printf '\n' | $(SPRITE) bash -c 'read -r; cat /tmp/forage-backup.db' > "$$backup"; \
	test "$$(sqlite3 "$$backup" 'PRAGMA integrity_check')" = ok || { echo "Backup $$backup failed integrity check"; exit 1; }; \
	echo "Backup saved to $$backup"

# Tests, then a verified backup, then upload beside the running binary, swap,
# restart, and confirm the new build is answering. If any step fails, make
# stops and the old server keeps running (the swap is the last step before
# the restart).
deploy: test backup
	@set -eu; \
	mkdir -p build; \
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o build/forage-server-linux ./cmd/forage-server; \
	echo "Uploading $(VERSION)..."; \
	cat build/forage-server-linux | $(SPRITE) bash -c "cat > $(REMOTE).new && chmod +x $(REMOTE).new"; \
	$(SPRITE) mv $(REMOTE).new $(REMOTE); \
	echo "sprite-env services stop web && sprite-env services start web" | $(SPRITE) bash; \
	for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s -m 10 $(SITE)/api/health | grep -q '"build":"$(VERSION)"'; then \
			echo "Deployed $(VERSION) to $(SITE)"; exit 0; \
		fi; \
		sleep 3; \
	done; \
	echo "Deploy uploaded, but $(SITE)/api/health is not reporting $(VERSION)"; exit 1

clean:
	rm -rf forage-server build coverage.out coverage.html

help:
	@echo "Targets:"
	@echo "  all       fmt, vet, test, build (default)"
	@echo "  build     build ./forage-server"
	@echo "  test      run tests"
	@echo "  run       local server on :$(DEV_PORT) (API key: dev)"
	@echo "  dev-data  copy the newest backup into $(DEV_DIR)/ for 'make run'"
	@echo "  backup    download a verified copy of the production database"
	@echo "  deploy    test, back up, upload, swap, restart, verify"
	@echo "  clean     remove build output"
