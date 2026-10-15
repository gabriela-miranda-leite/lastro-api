BINARY=lastro-api
MAIN=./cmd/api/main.go
MIGRATE=migrate -path db/migrations -database "$(DATABASE_URL)"

.PHONY: build run test lint migrate-up migrate-down swagger

build:
	go build -o bin/$(BINARY) $(MAIN)

run:
	go run $(MAIN)

test:
	go test ./...

lint:
	golangci-lint run

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

swagger:
	swag init -g cmd/api/main.go -o docs
