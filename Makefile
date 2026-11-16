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

## check-openapi: fail if the spec files on disk don't match what the code's
##      annotations generate. In CI the checkout is the commit, so this is
##      also the "committed specs are stale" check; locally it passes with
##      uncommitted-but-current specs. Stale files are left regenerated (just
##      commit them); if generation fails the originals are restored.
check-openapi:
	@tmp=$$(mktemp -d); cp $(SPECS) $$tmp/; \
	if ! $(GO) generate ./$(APIDOCS)/... >$$tmp/generate.log 2>&1; then \
		cat $$tmp/generate.log; cp $$tmp/*.json $(APIDOCS)/; rm -rf $$tmp; echo "go generate failed"; exit 1; \
	fi; \
	stale=0; for f in $(SPECS); do cmp -s $$f $$tmp/$$(basename $$f) || { echo "stale: $$f"; stale=1; }; done; \
	rm -rf $$tmp; \
	if [ $$stale -ne 0 ]; then echo "API specs were out of date and have been regenerated: commit them"; exit 1; fi; \
	echo "API specs up to date"

clean:
	rm -rf $(BIN)
