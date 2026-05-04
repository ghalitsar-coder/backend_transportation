.PHONY: run build test migrate-up migrate-down

run:
	go run cmd/server/main.go

build:
	go build -o tmp/server cmd/server/main.go

test:
	go test -v ./...
