FROM golang:1.24 AS builder

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
# Note: .env file should be provided via environment variables or mounted volume for security

# Use environment variable to determine which app to run
ENV APP_TYPE=crawler
ENV CRAWLER_MODE=full
ENV ENABLE_AI=true
ENV ENABLE_FINGERPRINTING=true
ENV MAX_AGE=24h
ENV AI_THRESHOLD=6h

CMD ["sh", "-c", "if [ \"$APP_TYPE\" = \"api\" ]; then ./api; else ./crawler -mode=$CRAWLER_MODE -enable-ai=$ENABLE_AI -enable-fingerprinting=$ENABLE_FINGERPRINTING -max-age=$MAX_AGE -ai-threshold=$AI_THRESHOLD; fi"]