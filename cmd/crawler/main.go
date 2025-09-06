package main

import (
	"context"
	"log"

	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Load application configuration
	cfg := config.LoadConfig()

	// Create a context for the crawler
	ctx := context.Background()

	// Initialize and start the crawler
	c := crawler.NewCrawler(ctx, cfg)
	if err := c.StartCrawling(ctx); err != nil {
		log.Fatalf("Error starting crawler: %v", err)
	}

	log.Println("Crawler started successfully")
}
