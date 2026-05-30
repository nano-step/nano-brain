VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build lint test test-integration test-e2e clean web-install web-dev web-build web-check

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

web-install:
	cd web 2>/dev/null && npm ci || echo "web/ not present (Story 9.5b)"

web-dev:
	cd web 2>/dev/null && npm run dev || echo "web/ not present (Story 9.5b)"

web-build:
	cd web 2>/dev/null && npm run build || echo "web/ not present (Story 9.5b)"

web-check:
	cd web 2>/dev/null && npm run lint && npm run typecheck || echo "web/ not present (Story 9.5b)"
