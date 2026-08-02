# ---------- Build Stage ----------
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w" -o query ./cmd/query

# ---------- Runtime Stage ----------
FROM alpine:3.20

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/query .
COPY config.yml .

EXPOSE 19090 19091

# Numeric so runAsNonRoot can verify the user (65534 = nobody).
USER 65534:65534

ENTRYPOINT ["./query"]
