BINARY=lastro-api
MAIN=./cmd/api/main.go
MIGRATE=migrate -path db/migrations -database "$(DATABASE_URL)"

.PHONY: build run test lint migrate-up migrate-down swagger load-test load-test-dashboard load-test-transactions load-test-idempotency

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

load-test: load-test-dashboard load-test-transactions load-test-idempotency

load-test-dashboard:
	k6 run load-tests/k6/scripts/dashboard.js --out json=load-tests/k6/reports/dashboard.json

load-test-transactions:
	k6 run load-tests/k6/scripts/transactions.js --out json=load-tests/k6/reports/transactions.json

load-test-idempotency:
	k6 run load-tests/k6/scripts/idempotency.js --out json=load-tests/k6/reports/idempotency.json
