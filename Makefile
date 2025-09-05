# Makefile

.PHONY: build test run deploy

build:
	go build -o ./bin/crawler ./cmd/crawler/main.go

test:
	go test ./...

run:
	docker-compose up --build

deploy:
	gcloud run deploy go-crawler-project --image gcr.io/$(PROJECT_ID)/go-crawler-project --platform managed --region $(REGION) --allow-unauthenticated