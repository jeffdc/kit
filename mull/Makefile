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

# The new mull is not installed until cutover: the installed mull is the old
# one, still in daily use. See "Transition" in docs/design.md.
install:
	@echo "Not installing: this is mull-next, which replaces the installed mull only at cutover." >&2
	@echo "Use ./mull from 'make build' while developing." >&2
	@exit 1

clean:
	rm -f mull
