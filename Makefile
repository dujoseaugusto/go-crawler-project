# Makefile

.PHONY: build test test-unit test-integration test-coverage test-verbose run deploy clean

# Build commands
build:
	go build -o ./bin/crawler ./cmd/crawler/main.go
	go build -o ./bin/api ./cmd/api/main.go

build-all:
	go build -o ./bin/crawler ./cmd/crawler/main.go
	go build -o ./bin/api ./cmd/api/main.go

# Test commands
test:
	go test ./... -v

test-unit:
	go test ./internal/utils/... ./api/handler/... ./internal/service/... ./internal/crawler/... -v

test-integration:
	go test ./internal/repository/... -v

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-coverage-func:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

test-verbose:
	go test ./... -v -race

test-bench:
	go test ./... -bench=. -benchmem

# Run commands
run:
	docker-compose up --build

run-api:
	go run ./cmd/api/main.go

run-crawler:
	go run ./cmd/crawler/main.go

run-crawler-full:
	go run ./cmd/crawler/main.go -mode=full -enable-ai=true

run-crawler-incremental:
	go run ./cmd/crawler/main.go -mode=incremental -enable-ai=true -enable-fingerprinting=true

# Development commands
deps:
	go mod download
	go mod tidy

fmt:
	go fmt ./...

lint:
	golangci-lint run

vet:
	go vet ./...

# Docker commands
docker-build:
	docker build -t go-crawler-project .

docker-run-api:
	docker run -e APP_TYPE=api -p 8080:8080 go-crawler-project

docker-run-crawler:
	docker run -e APP_TYPE=crawler go-crawler-project

# Database commands
mongo-start:
	docker run -d --name mongo-test -p 27017:27017 mongo:latest

mongo-stop:
	docker stop mongo-test && docker rm mongo-test

# Clean commands
clean:
	rm -rf ./bin/
	rm -f coverage.out coverage.html
	go clean -testcache

# Deploy commands
deploy:
	gcloud run deploy go-crawler-project --image gcr.io/$(PROJECT_ID)/go-crawler-project --platform managed --region $(REGION) --allow-unauthenticated

# Help
help:
	@echo "Available commands:"
	@echo "  build              - Build crawler binary"
	@echo "  build-all          - Build all binaries"
	@echo "  test               - Run all tests"
	@echo "  test-unit          - Run unit tests only"
	@echo "  test-integration   - Run integration tests only"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  test-coverage-func - Run tests with function coverage"
	@echo "  test-verbose       - Run tests with race detection"
	@echo "  test-bench         - Run benchmark tests"
	@echo "  run                - Run with docker-compose"
	@echo "  run-api            - Run API locally"
	@echo "  run-crawler        - Run crawler locally"
	@echo "  run-crawler-full   - Run full crawler mode"
	@echo "  run-crawler-incremental - Run incremental crawler mode"
	@echo "  deps               - Download and tidy dependencies"
	@echo "  fmt                - Format code"
	@echo "  lint               - Run linter"
	@echo "  vet                - Run go vet"
	@echo "  clean              - Clean build artifacts and test cache"
	@echo "  help               - Show this help message"