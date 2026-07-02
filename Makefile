VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build lint test test-integration test-e2e clean generate-openapi

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ./bin/nano-brain ./cmd/nano-brain/

lint:
	golangci-lint run ./...

test:
	go test -race -short ./...

test-integration:
	go test -race -tags=integration ./...

test-e2e:
	go test -race -tags=e2e ./...

clean:
	rm -rf ./bin/

# generate-openapi regenerates the canonical docs/openapi.json AND copies it
# to internal/server/handlers/openapi.json (the //go:embed source for the
# GET /api/openapi.json handler). Go's //go:embed directive cannot reach
# paths outside its own package directory, so the served copy must be
# colocated; writing both here guarantees they never drift apart.
generate-openapi:
	go run ./internal/openapigen/cmd/generate-openapi > docs/openapi.json
	cp docs/openapi.json internal/server/handlers/openapi.json
