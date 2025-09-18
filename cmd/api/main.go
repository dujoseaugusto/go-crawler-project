package main

import (
	"log"
	"net/http"

	"github.com/dujoseaugusto/go-crawler-project/api"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/dujoseaugusto/go-crawler-project/internal/service"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file, using default environment variables")
	}

	// Load application configuration
	cfg := config.LoadConfig()

	// Initialize MongoDB repository
	repo, err := repository.NewMongoRepository(cfg.MongoURI, "crawler", "properties")
	if err != nil {
		log.Fatalf("Failed to create MongoDB repository: %v", err)
	}
	defer repo.Close()

	// Initialize URL repository for incremental crawling
	urlRepo, err := repository.NewMongoURLRepository(cfg.MongoURI, "crawler")
	if err != nil {
		log.Fatalf("Failed to create MongoDB URL repository: %v", err)
	}
	defer urlRepo.Close()

	// Initialize city sites repository
	citySitesRepo, err := repository.NewMongoCitySitesRepository(cfg.MongoURI, "crawler")
	if err != nil {
		log.Printf("Warning: Failed to create city sites repository: %v", err)
		citySitesRepo = nil
	}
	if citySitesRepo != nil {
		defer citySitesRepo.Close()
	}

	// Initialize services (simplified - no learning systems)
	var propertyService *service.PropertyService
	var citySitesService *service.CitySitesService

	if citySitesRepo != nil {
		// Use new service with city sites support
		propertyService = service.NewPropertyServiceWithCitySites(repo, urlRepo, citySitesRepo, cfg)
		citySitesService = service.NewCitySitesService(citySitesRepo)
		log.Printf("City sites management enabled")
	} else {
		// Fallback to original service
		propertyService = service.NewPropertyService(repo, urlRepo, cfg)
		log.Printf("Using fallback mode without city sites management")
	}

	log.Printf("Sistema simplificado - todas as páginas são tratadas como propriedades")

	// Setup router (simplified)
	router := api.SetupRouterWithCitySites(propertyService, citySitesService)

	// Start server
	log.Printf("Starting server on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
