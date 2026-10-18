BINARY=lastro-api
MAIN=./cmd/api/main.go
MIGRATE=migrate -path db/migrations -database "$(DATABASE_URL)"

.PHONY: build run test lint migrate-up migrate-down swagger coverage load-test load-test-dashboard load-test-transactions load-test-idempotency

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

# coverage: run tests with coverage and fail if totals fall below minimums.
# Global minimum: 75%. Service layer (packages with a service.go): 80%.
coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "--- Coverage summary ---"; \
	go tool cover -func=coverage.out | grep '^total:'; \
	total=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{gsub(/%/,""); print $$3}'); \
	echo "Global total: $${total}%"; \
	if awk "BEGIN{exit !($${total}+0 < 75)}"; then \
	  echo "FAIL: global coverage $${total}% is below 75%"; exit 1; \
	fi; \
	echo "Global threshold OK (≥75%)."

load-test: load-test-dashboard load-test-transactions load-test-idempotency

load-test-dashboard:
	k6 run load-tests/k6/scripts/dashboard.js --out json=load-tests/k6/reports/dashboard.json

load-test-transactions:
	k6 run load-tests/k6/scripts/transactions.js --out json=load-tests/k6/reports/transactions.json

load-test-idempotency:
	k6 run load-tests/k6/scripts/idempotency.js --out json=load-tests/k6/reports/idempotency.json
