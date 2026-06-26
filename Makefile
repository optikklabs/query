.PHONY: build run fmt vet test

build:
	go build -v ./cmd/query

run:
	go run ./cmd/query

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...
