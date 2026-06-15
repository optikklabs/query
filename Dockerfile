FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy module manifests first for layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" -o query ./cmd/query

FROM alpine:3.20
WORKDIR /app

COPY --from=builder /app/query .
COPY config.yml .

RUN chown -R 1000:1000 /app
USER 1000:1000

# Match default config.yml: HTTP server.port (override with -p when running).
EXPOSE 19090
ENTRYPOINT ["./query"]
