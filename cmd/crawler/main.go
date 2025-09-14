package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/joho/godotenv"
)

func main() {
	// Parse command line flags
	var (
		mode                 = flag.String("mode", "full", "Crawling mode: 'full' or 'incremental'")
		enableAI             = flag.Bool("enable-ai", true, "Enable AI processing")
		enableFingerprinting = flag.Bool("enable-fingerprinting", true, "Enable page fingerprinting")
		maxAge               = flag.Duration("max-age", 24*time.Hour, "Maximum age before reprocessing")
		aiThreshold          = flag.Duration("ai-threshold", 6*time.Hour, "Minimum time before AI reprocessing")
		showStats            = flag.Bool("stats", false, "Show statistics and exit")
		cleanup              = flag.Bool("cleanup", false, "Cleanup old records and exit")
		help                 = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Configurar logger
	appLogger := logger.NewLogger("crawler_main")
	appLogger.Info("Starting Go Crawler Application")

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		appLogger.Warn("Warning: Error loading .env file, using default environment variables")
	}

	// Load application configuration
	cfg := config.LoadConfig()
	appLogger.WithFields(map[string]interface{}{
		"port":                  cfg.Port,
		"sites_file":            cfg.SitesFile,
		"mode":                  *mode,
		"enable_ai":             *enableAI,
		"enable_fingerprinting": *enableFingerprinting,
		"max_age":               *maxAge,
	}).Info("Configuration loaded")

	// Create a context for the crawler
	ctx := context.Background()

	// Initialize MongoDB repository
	repo, err := repository.NewMongoRepository(cfg.MongoURI, "crawler", "properties")
	if err != nil {
		appLogger.Fatal("Failed to create MongoDB repository", err)
	}
	defer repo.Close()
	appLogger.Info("MongoDB repository initialized")

	// Initialize URL repository for incremental mode
	var urlRepo repository.URLRepository
	if *mode == "incremental" || *showStats || *cleanup {
		urlRepo, err = repository.NewMongoURLRepository(cfg.MongoURI, "crawler")
		if err != nil {
			appLogger.Fatal("Failed to create URL repository", err)
		}
		defer urlRepo.Close()
		appLogger.Info("URL repository initialized")
	}

	// Handle special commands
	if *showStats {
		showStatistics(ctx, urlRepo, appLogger)
		return
	}

	if *cleanup {
		performCleanup(ctx, urlRepo, appLogger)
		return
	}

	// Load URLs from configuration file
	urls, err := loadURLsFromFile(cfg.SitesFile)
	if err != nil {
		appLogger.Fatal("Failed to load URLs from file", err)
	}
	appLogger.WithField("urls_count", len(urls)).Info("URLs loaded from configuration")

	// Initialize AI service (optional)
	var aiService *ai.GeminiService
	if *enableAI {
		if aiSvc, err := ai.NewGeminiService(ctx); err == nil {
			aiService = aiSvc
			appLogger.Info("AI service initialized successfully")
		} else {
			appLogger.WithError(err).Warn("AI service not available, continuing without AI")
		}
	}

	// Start crawling based on mode
	startTime := time.Now()
	appLogger.WithField("mode", *mode).Info("Starting crawler execution")

	if *mode == "incremental" {
		runIncrementalCrawling(ctx, repo, urlRepo, aiService, urls, *enableAI, *enableFingerprinting, *maxAge, *aiThreshold, appLogger)
	} else {
		runFullCrawling(ctx, repo, aiService, urls, appLogger)
	}

	duration := time.Since(startTime)
	appLogger.WithField("total_duration", duration).Info("Crawler execution completed")
}

// loadURLsFromFile carrega URLs do arquivo de configuração
func loadURLsFromFile(filePath string) ([]string, error) {
	var urls []string
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &urls); err != nil {
		return nil, err
	}

	return urls, nil
}

// calculateSuccessRate calcula a taxa de sucesso
func calculateSuccessRate(saved, found int) float64 {
	if found == 0 {
		return 0
	}
	return float64(saved) / float64(found) * 100
}

// runFullCrawling executa crawling completo (modo tradicional)
func runFullCrawling(ctx context.Context, repo repository.PropertyRepository, aiService *ai.GeminiService, urls []string, appLogger *logger.Logger) {
	appLogger.Info("Running full crawling mode")

	// Create and start the traditional crawler engine
	engine := crawler.NewCrawlerEngine(repo, aiService)

	if err := engine.Start(ctx, urls); err != nil {
		appLogger.Fatal("Full crawler execution failed", err)
	}

	// Log final statistics
	stats := engine.GetStats()
	appLogger.WithFields(map[string]interface{}{
		"urls_visited":     stats.URLsVisited,
		"properties_found": stats.PropertiesFound,
		"properties_saved": stats.PropertiesSaved,
		"errors":           stats.ErrorsCount,
		"success_rate":     calculateSuccessRate(stats.PropertiesSaved, stats.PropertiesFound),
	}).Info("Full crawling completed")
}

// runIncrementalCrawling executa crawling incremental
func runIncrementalCrawling(ctx context.Context, repo repository.PropertyRepository, urlRepo repository.URLRepository, aiService *ai.GeminiService, urls []string, enableAI, enableFingerprinting bool, maxAge, aiThreshold time.Duration, appLogger *logger.Logger) {
	appLogger.Info("Running incremental crawling mode")

	// Configure incremental crawler
	config := crawler.IncrementalConfig{
		EnableAI:             enableAI,
		EnableFingerprinting: enableFingerprinting,
		MaxAge:               maxAge,
		AIThreshold:          aiThreshold,
		CleanupInterval:      7 * 24 * time.Hour, // 7 days
		MaxConcurrency:       5,
		DelayBetweenRequests: 1 * time.Second,
		UserAgent:            "Go-Crawler-Incremental/2.0",
	}

	// Create incremental engine
	engine := crawler.NewIncrementalCrawlerEngine(repo, urlRepo, aiService, config)

	if err := engine.Start(ctx, urls); err != nil {
		appLogger.Fatal("Incremental crawler execution failed", err)
	}

	// Log final statistics
	stats := engine.GetStatistics()
	appLogger.WithFields(map[string]interface{}{
		"total_urls":          stats.TotalURLs,
		"processed_urls":      stats.ProcessedURLs,
		"skipped_urls":        stats.SkippedURLs,
		"new_properties":      stats.NewProperties,
		"failed_urls":         stats.FailedURLs,
		"ai_processing_count": stats.AIProcessingCount,
		"ai_skipped_count":    stats.AISkippedCount,
		"ai_savings_estimate": stats.AISavingsEstimate,
		"processing_time":     stats.ProcessingTimeTotal,
		"efficiency_gain":     fmt.Sprintf("%.1f%%", float64(stats.SkippedURLs)/float64(stats.TotalURLs)*100),
	}).Info("Incremental crawling completed")
}

// showStatistics mostra estatísticas do sistema
func showStatistics(ctx context.Context, urlRepo repository.URLRepository, appLogger *logger.Logger) {
	appLogger.Info("Fetching system statistics")

	stats, err := urlRepo.GetStatistics(ctx)
	if err != nil {
		appLogger.Fatal("Failed to get statistics", err)
	}

	fmt.Println("\n=== CRAWLER STATISTICS ===")
	fmt.Printf("Total URLs processed: %d\n", stats.TotalURLs)
	fmt.Printf("Processed today: %d\n", stats.ProcessedToday)
	fmt.Printf("Successful today: %d\n", stats.SuccessfulToday)
	fmt.Printf("Failed today: %d\n", stats.FailedToday)
	fmt.Printf("Skipped today: %d\n", stats.SkippedToday)
	fmt.Printf("AI savings (total): %d URLs\n", stats.TotalAISavings)

	if stats.ProcessedToday > 0 {
		successRate := float64(stats.SuccessfulToday) / float64(stats.ProcessedToday) * 100
		fmt.Printf("Success rate today: %.1f%%\n", successRate)
	}

	if stats.TotalAISavings > 0 && stats.TotalURLs > 0 {
		aiSavingsPercent := float64(stats.TotalAISavings) / float64(stats.TotalURLs) * 100
		fmt.Printf("AI processing savings: %.1f%%\n", aiSavingsPercent)
	}

	fmt.Println("========================")
}

// performCleanup executa limpeza de registros antigos
func performCleanup(ctx context.Context, urlRepo repository.URLRepository, appLogger *logger.Logger) {
	appLogger.Info("Starting cleanup of old records")

	maxAge := 30 * 24 * time.Hour // 30 days
	if err := urlRepo.CleanupOldRecords(ctx, maxAge); err != nil {
		appLogger.Fatal("Failed to cleanup old records", err)
	}

	appLogger.Info("Cleanup completed successfully")
}

// showHelp mostra ajuda sobre os comandos disponíveis
func showHelp() {
	fmt.Println(`
Go Crawler - Sistema de Crawling Incremental

USAGE:
    ./crawler [OPTIONS]

OPTIONS:
    -mode string
        Crawling mode: 'full' or 'incremental' (default "full")
        
    -enable-ai
        Enable AI processing (default true)
        
    -enable-fingerprinting
        Enable page fingerprinting for change detection (default true)
        
    -max-age duration
        Maximum age before reprocessing a URL (default 24h)
        
    -ai-threshold duration
        Minimum time before AI reprocessing (default 6h)
        
    -stats
        Show statistics and exit
        
    -cleanup
        Cleanup old records and exit
        
    -help
        Show this help message

EXAMPLES:
    # Full crawling (traditional mode)
    ./crawler -mode=full
    
    # Incremental crawling with AI
    ./crawler -mode=incremental -enable-ai=true
    
    # Incremental crawling without AI (faster)
    ./crawler -mode=incremental -enable-ai=false
    
    # Custom thresholds
    ./crawler -mode=incremental -max-age=12h -ai-threshold=3h
    
    # Show statistics
    ./crawler -stats
    
    # Cleanup old records
    ./crawler -cleanup

INCREMENTAL MODE BENEFITS:
    - 85-90% reduction in processing time
    - 90%+ reduction in AI costs
    - Intelligent change detection
    - Automatic optimization based on page behavior
    
For more information, see: INCREMENTAL_CRAWLING_SOLUTIONS.md
`)
}
