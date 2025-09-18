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
		configFile    = flag.String("config", "", "Path to config file")
		referenceFile = flag.String("reference", "List-site.ini", "Path to reference URLs file")
		trainOnly     = flag.Bool("train-only", false, "Only train patterns, don't crawl")
		showStats     = flag.Bool("stats", false, "Show crawler statistics")
	)
	flag.Parse()

	// Configura logger
	appLogger := logger.NewLogger("improved_crawler_main")
	appLogger.Info("Starting improved crawler application")

	// Carrega configuração
	cfg := config.LoadConfig()
	if *configFile != "" {
		// Se especificado arquivo de config, carrega configurações específicas
		appLogger.WithField("config_file", *configFile).Info("Using custom config file")
	}

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

	// Cria crawler melhorado
	improvedCrawler := crawler.NewImprovedCrawler(ctx, cfg)

	// Treina padrões usando arquivo de referência
	appLogger.WithField("reference_file", *referenceFile).Info("Training patterns from reference file")

	startTime := time.Now()
	if err := improvedCrawler.TrainFromReferenceFile(ctx, *referenceFile); err != nil {
		log.Fatalf("Failed to train from reference file: %v", err)
	}

	trainingDuration := time.Since(startTime)
	appLogger.WithField("duration", trainingDuration.String()).Info("Pattern training completed")

	// Se é apenas treinamento, sai aqui
	if *trainOnly {
		appLogger.Info("Training completed. Exiting (train-only mode)")
		return
	}

	// Inicia crawling
	appLogger.Info("Starting improved crawling process")
	crawlStartTime := time.Now()

	// Executa crawling em goroutine para permitir cancelamento
	crawlDone := make(chan error, 1)
	go func() {
		crawlDone <- improvedCrawler.StartCrawling(ctx)
	}()

	// Goroutine para mostrar estatísticas periodicamente
	if *showStats {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					stats := improvedCrawler.GetStats()
					printStats(appLogger, stats)
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
			appLogger.Error("Crawling failed", err)
		} else {
			crawlDuration := time.Since(crawlStartTime)
			appLogger.WithField("duration", crawlDuration.String()).Info("Crawling completed successfully")
		}
	case <-ctx.Done():
		appLogger.Info("Crawling cancelled by user")
	}

	// Mostra estatísticas finais
	finalStats := improvedCrawler.GetStats()
	printFinalStats(appLogger, finalStats)

	appLogger.Info("Improved crawler application finished")
}

// printStats imprime estatísticas periódicas
func printStats(logger *logger.Logger, stats *crawler.ImprovedCrawlerStats) {
	logger.WithFields(map[string]interface{}{
		"pages_visited":     stats.PagesVisited,
		"properties_found":  stats.PropertiesFound,
		"properties_saved":  stats.PropertiesSaved,
		"catalog_pages":     stats.CatalogPagesFound,
		"errors":            stats.ErrorsEncountered,
		"domains_processed": len(stats.DomainStats),
	}).Info("Crawling progress")
}

// printFinalStats imprime estatísticas finais detalhadas
func printFinalStats(logger *logger.Logger, stats *crawler.ImprovedCrawlerStats) {
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
	}).Info("Final crawling statistics")

	// Mostra estatísticas por domínio
	logger.Info("Domain statistics:")
	for domain, count := range stats.DomainStats {
		logger.WithFields(map[string]interface{}{
			"domain": domain,
			"pages":  count,
		}).Info("Domain processed")
	}
}
