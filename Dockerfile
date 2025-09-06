FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o crawler ./cmd/crawler/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o api_bin ./cmd/api/main.go

FROM alpine:latest

# Instalar dependências necessárias
RUN apk add --no-cache ca-certificates

WORKDIR /root/

COPY --from=builder /app/crawler .
COPY --from=builder /app/api_bin ./api
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/.env .

# Use environment variable to determine which app to run
ENV APP_TYPE=crawler
CMD if [ "$APP_TYPE" = "api" ]; then ./api; else ./crawler; fi