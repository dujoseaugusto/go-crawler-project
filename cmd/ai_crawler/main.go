package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
)

func main() {
	// Flags de linha de comando
	var (
		referenceFile = flag.String("reference", "List-site.ini", "Path to reference URLs file")
		trainOnly     = flag.Bool("train-only", false, "Only train patterns with AI, don't crawl")
		showStats     = flag.Bool("stats", false, "Show crawler statistics")
		aiMode        = flag.String("ai-mode", "full", "AI mode: full, basic, or none")
		verbose       = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	// Configura logger
	appLogger := logger.NewLogger("ai_crawler_main")
	if *verbose {
		appLogger.Info("Verbose logging enabled")
	}

	appLogger.Info("Starting AI-integrated crawler application")

	// Verifica se chave da API do Gemini está configurada
	if os.Getenv("GEMINI_API_KEY") == "" {
		appLogger.Warn("GEMINI_API_KEY not set - AI features will be disabled")
		*aiMode = "none"
	}

	// Carrega configuração
	cfg := config.LoadConfig()

	// Cria contexto com cancelamento
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configura handler para sinais do sistema
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		appLogger.WithField("signal", sig.String()).Info("Received shutdown signal")
		cancel()
	}()

	// Cria crawler baseado no modo de IA
	var err error

	switch *aiMode {
	case "full":
		err = runFullAICrawler(ctx, cfg, *referenceFile, *trainOnly, *showStats, appLogger)
	case "basic":
		err = runBasicAICrawler(ctx, cfg, *referenceFile, *trainOnly, *showStats, appLogger)
	case "none":
		err = runNonAICrawler(ctx, cfg, *referenceFile, *trainOnly, *showStats, appLogger)
	default:
		log.Fatalf("Invalid AI mode: %s. Use 'full', 'basic', or 'none'", *aiMode)
	}

	if err != nil {
		log.Fatalf("Crawler execution failed: %v", err)
	}

	appLogger.Info("AI-integrated crawler application finished")
}

// runFullAICrawler executa crawler com IA completa integrada
func runFullAICrawler(ctx context.Context, cfg *config.Config, referenceFile string, trainOnly, showStats bool, appLogger *logger.Logger) error {
	appLogger.Info("Running in FULL AI mode - complete AI integration")

	// Cria crawler integrado com IA
	aiCrawler, err := crawler.NewAIIntegratedCrawler(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create AI-integrated crawler: %w", err)
	}

	// Treina com IA avançada
	appLogger.WithField("reference_file", referenceFile).Info("Training with advanced AI analysis")

	startTime := time.Now()
	if err := aiCrawler.TrainWithAI(ctx, referenceFile); err != nil {
		return fmt.Errorf("failed to train with AI: %w", err)
	}

	trainingDuration := time.Since(startTime)
	appLogger.WithField("duration", trainingDuration.String()).Info("AI training completed")

	// Se é apenas treinamento, sai aqui
	if trainOnly {
		appLogger.Info("Training completed. Exiting (train-only mode)")
		return nil
	}

	// Inicia crawling com IA
	appLogger.Info("Starting AI-integrated crawling process")
	crawlStartTime := time.Now()

	// Executa crawling em goroutine para permitir cancelamento
	crawlDone := make(chan error, 1)
	go func() {
		crawlDone <- aiCrawler.StartCrawling(ctx)
	}()

	// Goroutine para mostrar estatísticas periodicamente
	if showStats {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					stats := aiCrawler.GetStats()
					printAIStats(appLogger, stats)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Aguarda conclusão ou cancelamento
	select {
	case err := <-crawlDone:
		if err != nil {
			return fmt.Errorf("AI crawling failed: %w", err)
		}
		crawlDuration := time.Since(crawlStartTime)
		appLogger.WithField("duration", crawlDuration.String()).Info("AI crawling completed successfully")
	case <-ctx.Done():
		appLogger.Info("AI crawling cancelled by user")
	}

	// Mostra estatísticas finais
	finalStats := aiCrawler.GetStats()
	printFinalAIStats(appLogger, finalStats)

	return nil
}

// runBasicAICrawler executa crawler com IA básica
func runBasicAICrawler(ctx context.Context, cfg *config.Config, referenceFile string, trainOnly, showStats bool, appLogger *logger.Logger) error {
	appLogger.Info("Running in BASIC AI mode - improved crawler with basic AI")

	// Cria crawler melhorado
	improvedCrawler := crawler.NewImprovedCrawler(ctx, cfg)

	// Treina padrões básicos
	appLogger.WithField("reference_file", referenceFile).Info("Training basic patterns")

	startTime := time.Now()
	if err := improvedCrawler.TrainFromReferenceFile(ctx, referenceFile); err != nil {
		return fmt.Errorf("failed to train basic patterns: %w", err)
	}

	trainingDuration := time.Since(startTime)
	appLogger.WithField("duration", trainingDuration.String()).Info("Basic training completed")

	if trainOnly {
		appLogger.Info("Training completed. Exiting (train-only mode)")
		return nil
	}

	// Inicia crawling básico
	appLogger.Info("Starting improved crawling process")
	crawlStartTime := time.Now()

	crawlDone := make(chan error, 1)
	go func() {
		crawlDone <- improvedCrawler.StartCrawling(ctx)
	}()

	if showStats {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					stats := improvedCrawler.GetStats()
					printBasicStats(appLogger, stats)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	select {
	case err := <-crawlDone:
		if err != nil {
			return fmt.Errorf("improved crawling failed: %w", err)
		}
		crawlDuration := time.Since(crawlStartTime)
		appLogger.WithField("duration", crawlDuration.String()).Info("Improved crawling completed successfully")
	case <-ctx.Done():
		appLogger.Info("Improved crawling cancelled by user")
	}

	finalStats := improvedCrawler.GetStats()
	printFinalBasicStats(appLogger, finalStats)

	return nil
}

// runNonAICrawler executa crawler sem IA
func runNonAICrawler(ctx context.Context, cfg *config.Config, referenceFile string, trainOnly, showStats bool, appLogger *logger.Logger) error {
	appLogger.Info("Running in NO AI mode - traditional crawler")

	// Cria crawler tradicional
	traditionalCrawler := crawler.NewCrawler(ctx, cfg)

	if trainOnly {
		appLogger.Info("No training available in non-AI mode. Exiting.")
		return nil
	}

	// Inicia crawling tradicional
	appLogger.Info("Starting traditional crawling process")

	return traditionalCrawler.StartCrawling(ctx)
}

// printAIStats imprime estatísticas do crawler com IA
func printAIStats(logger *logger.Logger, stats *crawler.AIIntegratedStats) {
	aiUsageRate := 0.0
	if stats.PagesVisited > 0 {
		aiUsageRate = float64(stats.AIClassifications) / float64(stats.PagesVisited) * 100
	}

	logger.WithFields(map[string]interface{}{
		"pages_visited":           stats.PagesVisited,
		"properties_found":        stats.PropertiesFound,
		"properties_saved":        stats.PropertiesSaved,
		"ai_classifications":      stats.AIClassifications,
		"ai_validations":          stats.AIValidations,
		"ai_enhancements":         stats.AIEnhancements,
		"pattern_matches":         stats.PatternMatches,
		"high_confidence_matches": stats.HighConfidenceMatches,
		"ai_usage_rate":           fmt.Sprintf("%.1f%%", aiUsageRate),
	}).Info("AI crawling progress")
}

// printBasicStats imprime estatísticas do crawler básico
func printBasicStats(logger *logger.Logger, stats *crawler.ImprovedCrawlerStats) {
	logger.WithFields(map[string]interface{}{
		"pages_visited":     stats.PagesVisited,
		"properties_found":  stats.PropertiesFound,
		"properties_saved":  stats.PropertiesSaved,
		"catalog_pages":     stats.CatalogPagesFound,
		"errors":            stats.ErrorsEncountered,
		"domains_processed": len(stats.DomainStats),
	}).Info("Improved crawling progress")
}

// printFinalAIStats imprime estatísticas finais do crawler com IA
func printFinalAIStats(logger *logger.Logger, stats *crawler.AIIntegratedStats) {
	duration := time.Since(stats.StartTime)

	successRate := 0.0
	if stats.PropertiesFound > 0 {
		successRate = float64(stats.PropertiesSaved) / float64(stats.PropertiesFound) * 100
	}

	aiUsageRate := 0.0
	if stats.PagesVisited > 0 {
		aiUsageRate = float64(stats.AIClassifications) / float64(stats.PagesVisited) * 100
	}

	patternMatchRate := 0.0
	if stats.PagesVisited > 0 {
		patternMatchRate = float64(stats.PatternMatches) / float64(stats.PagesVisited) * 100
	}

	logger.WithFields(map[string]interface{}{
		"total_duration":          duration.String(),
		"pages_visited":           stats.PagesVisited,
		"properties_found":        stats.PropertiesFound,
		"properties_saved":        stats.PropertiesSaved,
		"success_rate":            fmt.Sprintf("%.2f%%", successRate),
		"ai_classifications":      stats.AIClassifications,
		"ai_validations":          stats.AIValidations,
		"ai_enhancements":         stats.AIEnhancements,
		"ai_usage_rate":           fmt.Sprintf("%.2f%%", aiUsageRate),
		"pattern_matches":         stats.PatternMatches,
		"high_confidence_matches": stats.HighConfidenceMatches,
		"pattern_match_rate":      fmt.Sprintf("%.2f%%", patternMatchRate),
		"domains_processed":       len(stats.DomainStats),
		"avg_pages_per_min":       fmt.Sprintf("%.1f", float64(stats.PagesVisited)/duration.Minutes()),
	}).Info("Final AI-integrated crawling statistics")

	// Mostra estatísticas por domínio
	logger.Info("Domain statistics:")
	for domain, count := range stats.DomainStats {
		logger.WithFields(map[string]interface{}{
			"domain": domain,
			"pages":  count,
		}).Info("Domain processed")
	}
}

// printFinalBasicStats imprime estatísticas finais do crawler básico
func printFinalBasicStats(logger *logger.Logger, stats *crawler.ImprovedCrawlerStats) {
	duration := time.Since(stats.StartTime)

	successRate := 0.0
	if stats.PropertiesFound > 0 {
		successRate = float64(stats.PropertiesSaved) / float64(stats.PropertiesFound) * 100
	}

	logger.WithFields(map[string]interface{}{
		"total_duration":    duration.String(),
		"pages_visited":     stats.PagesVisited,
		"properties_found":  stats.PropertiesFound,
		"properties_saved":  stats.PropertiesSaved,
		"catalog_pages":     stats.CatalogPagesFound,
		"errors":            stats.ErrorsEncountered,
		"success_rate":      fmt.Sprintf("%.2f%%", successRate),
		"domains_processed": len(stats.DomainStats),
		"avg_pages_per_min": fmt.Sprintf("%.1f", float64(stats.PagesVisited)/duration.Minutes()),
	}).Info("Final improved crawling statistics")
}
