.PHONY: build test test-coverage lint vet fmt fmt-check docker docker-up docker-down clean openapi-lint help

BINARY := bin/server

## build: Compile the server binary
build:
	go build -o $(BINARY) ./cmd/server

## test: Run all tests
test:
	CGO_ENABLED=1 go test ./...

## test-coverage: Run tests with coverage report
test-coverage:
	CGO_ENABLED=1 go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format all Go files
fmt:
	gofmt -w .
	goimports -w -local github.com/wsapi-chat/wsapi-app .

## fmt-check: Check formatting without modifying files
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

## docker: Build Docker image
docker:
	docker build -t wsapi .

## docker-up: Start with Docker Compose
docker-up:
	docker compose up --build

## docker-down: Stop Docker Compose
docker-down:
	docker compose down

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out

## openapi-lint: Validate OpenAPI specs
openapi-lint:
	npx --yes @redocly/cli lint openapi/wsapi-api.yml
	npx --yes @redocly/cli lint openapi/wsapi-events.yml

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
