.PHONY: build run fmt vet lint

DEV_JWT_SECRET ?= optikk-local-development-secret-change-before-deploy
# base64 of 32 bytes; local-only, like DEV_JWT_SECRET.
DEV_LLM_KEY ?= b3B0aWtrLWxvY2FsLWRldi1sbG0ta2V5LTMyYnl0ZXM=

build:
	go build -v -o bin/query ./cmd/query

run:
	OPTIKK_AUTH_JWT_SECRET="$${OPTIKK_AUTH_JWT_SECRET:-$(DEV_JWT_SECRET)}" \
	OPTIKK_LLM_KEY_ENCRYPTION_KEY="$${OPTIKK_LLM_KEY_ENCRYPTION_KEY:-$(DEV_LLM_KEY)}" \
	go run ./cmd/query

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...
