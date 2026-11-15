.PHONY: test test-race vet build run-api run-web check

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

build:
	go build ./...

run-api:
	go run ./cmd/api

run-web:
	cd web && npm run dev

check: test test-race vet build
