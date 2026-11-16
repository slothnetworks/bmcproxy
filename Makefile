GO        ?= go
BIN       ?= bin
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)
APIDOCS   := internal/apidocs
SPECS     := $(APIDOCS)/swagger.json $(APIDOCS)/openapi.json

.PHONY: all build generate swagger dev test vet check-openapi clean

all: build

## generate: regenerate swagger.json (swag) and openapi.json (cmd/openapigen)
generate:
	$(GO) generate ./$(APIDOCS)/...

## swagger: alias for generate
swagger: generate

## build: regenerate the API docs, then build every command into $(BIN)/
build: generate
	@mkdir -p $(BIN)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN)/ ./cmd/...

## dev: build, then run bmcproxy in dev mode: disruptive ops enabled, debug
##      endpoints, DEBUG logging. Stays on localhost; disruptive requests still
##      need the X-BMC-Confirm: true header. Override e.g.
##      make dev DEV_LISTEN=127.0.0.1:9090 DEV_DEBUG=transport=trace DEV_ARGS=--trace-http
DEV_LISTEN ?= 127.0.0.1:8080
DEV_DEBUG  ?= all
DEV_ARGS   ?=
dev: build
	$(BIN)/bmcproxy --listen $(DEV_LISTEN) --allow-disruptive --debug-endpoints \
		--log-level debug --debug $(DEV_DEBUG) $(DEV_ARGS)

## test: run all tests
test:
	$(GO) test ./...

## vet: go vet everything
vet:
	$(GO) vet ./...

## check-openapi: fail if the committed specs are stale (for CI)
check-openapi:
	@tmp=$$(mktemp -d); cp $(SPECS) $$tmp/; \
	$(GO) generate ./$(APIDOCS)/... >/dev/null 2>&1 || { echo "go generate failed"; rm -rf $$tmp; exit 1; }; \
	stale=0; for f in $(SPECS); do cmp -s $$f $$tmp/$$(basename $$f) || { echo "stale: $$f"; stale=1; }; done; \
	rm -rf $$tmp; \
	git diff --exit-code --stat -- $(SPECS) || stale=1; \
	if [ $$stale -ne 0 ]; then echo "API specs are out of date: run 'make generate' and commit"; exit 1; fi; \
	echo "API specs up to date"

clean:
	rm -rf $(BIN)
