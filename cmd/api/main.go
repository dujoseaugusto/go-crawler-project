package main

import (
	"log"
	"net/http"

	"github.com/dujoseaugusto/go-crawler-project/api"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
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

	// Initialize pattern learner (URL-based - legacy)
	patternLearner := crawler.NewPatternLearner()
	log.Printf("URL-based pattern learning system initialized")

	// Initialize content-based pattern learner (SHARED INSTANCE with persistence)
	contentLearner := crawler.GetSharedContentLearner()
	log.Printf("Content-based pattern learning system initialized (SHARED INSTANCE with persistence)")

	// Initialize services
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

	// Conecta o PatternLearner ao PropertyService
	propertyService.SetPatternLearner(patternLearner)
	log.Printf("PatternLearner connected to PropertyService")

	// Conecta o ContentBasedPatternLearner ao PropertyService
	propertyService.SetContentLearner(contentLearner)
	log.Printf("ContentBasedPatternLearner connected to PropertyService")

	// Setup router with all available features
	router := api.SetupRouterWithContentLearning(propertyService, citySitesService, patternLearner, contentLearner)

	// Start server
	log.Printf("Starting server on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
