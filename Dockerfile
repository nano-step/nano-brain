FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /nano-brain ./cmd/nano-brain

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /nano-brain /usr/local/bin/nano-brain
EXPOSE 3100
ENTRYPOINT ["nano-brain"]
