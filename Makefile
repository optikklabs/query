.PHONY: build run fmt vet test

DEV_JWT_SECRET ?= optikk-local-development-secret-change-before-deploy

build:
	go build -v ./cmd/query

run:
	OPTIKK_AUTH_JWT_SECRET="$${OPTIKK_AUTH_JWT_SECRET:-$(DEV_JWT_SECRET)}" go run ./cmd/query

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...
