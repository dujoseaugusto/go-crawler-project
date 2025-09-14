FROM golang:1.24 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build both applications
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crawler ./cmd/crawler/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api_bin ./cmd/api/main.go

FROM alpine:latest

# Install necessary dependencies
RUN apk add --no-cache ca-certificates curl

WORKDIR /root/

# Copy binaries and configs
COPY --from=builder /app/crawler .
COPY --from=builder /app/api_bin ./api
COPY --from=builder /app/configs ./configs

# Set default environment variables
ENV APP_TYPE=api
ENV PORT=8080
ENV CRAWLER_MODE=incremental
ENV ENABLE_AI=true
ENV ENABLE_FINGERPRINTING=true
ENV MAX_AGE=24h
ENV AI_THRESHOLD=6h

# Expose port for Railway
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD if [ "$APP_TYPE" = "api" ]; then curl -f http://localhost:8080/health || exit 1; else exit 0; fi

# Start command
CMD ["sh", "-c", "if [ \"$APP_TYPE\" = \"api\" ]; then ./api; else ./crawler -mode=$CRAWLER_MODE -enable-ai=$ENABLE_AI -enable-fingerprinting=$ENABLE_FINGERPRINTING -max-age=$MAX_AGE -ai-threshold=$AI_THRESHOLD; fi"]