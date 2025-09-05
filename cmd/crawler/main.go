package main

import (
	"context"
	"log"

	"go-crawler-project/internal/config"
	"go-crawler-project/internal/crawler"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Load application configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Create a context for the crawler
	ctx := context.Background()

	// Initialize and start the crawler
	c := crawler.NewCrawler(cfg)
	if err := c.StartCrawling(ctx); err != nil {
		log.Fatalf("Error starting crawler: %v", err)
	}

	log.Println("Crawler started successfully")
}
