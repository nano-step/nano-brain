# Build stage
FROM golang:1.23-bookworm AS builder

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /nano-brain \
    ./cmd/nano-brain

# Runtime stage
FROM gcr.io/distroless/static-debian12

COPY --from=builder /nano-brain /nano-brain

EXPOSE 3100

ENTRYPOINT ["/nano-brain"]
