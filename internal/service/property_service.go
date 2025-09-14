package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
)

type PropertyService struct {
	repo    repository.PropertyRepository
	urlRepo repository.URLRepository
	config  *config.Config
	logger  *logger.Logger
}

func NewPropertyService(repo repository.PropertyRepository, urlRepo repository.URLRepository, cfg *config.Config) *PropertyService {
	return &PropertyService{
		repo:    repo,
		urlRepo: urlRepo,
		config:  cfg,
		logger:  logger.NewLogger("property_service"),
	}
}

func (s *PropertyService) SaveProperty(ctx context.Context, property repository.Property) error {
	if property.Endereco == "" || property.Cidade == "" || property.Descricao == "" {
		return errors.New("property fields cannot be empty")
	}
	return s.repo.Save(ctx, property)
}

func (s *PropertyService) GetAllProperties(ctx context.Context) ([]repository.Property, error) {
	return s.repo.FindAll(ctx)
}

func (s *PropertyService) SearchProperties(ctx context.Context, filter repository.PropertyFilter, pagination repository.PaginationParams) (*repository.PropertySearchResult, error) {
	return s.repo.FindWithFilters(ctx, filter, pagination)
}

// ForceCrawling inicia manualmente o processo de coleta de dados usando o sistema incremental
func (s *PropertyService) ForceCrawling(ctx context.Context) error {
	s.logger.Info("Starting incremental crawling process")

	// Carrega as URLs do arquivo de configuração
	urls, err := s.loadURLsFromFile(s.config.SitesFile)
	if err != nil {
		s.logger.Error("Failed to load URLs from file", err)
		return fmt.Errorf("erro ao carregar URLs: %v", err)
	}

	s.logger.WithField("urls_count", len(urls)).Info("URLs loaded successfully")

	// Inicializar o serviço de IA (opcional)
	var aiService *ai.GeminiService
	if aiSvc, err := ai.NewGeminiService(ctx); err == nil {
		aiService = aiSvc
		s.logger.Info("AI service initialized successfully")
	} else {
		s.logger.WithError(err).Warn("AI service not available, continuing without AI")
	}

	// Configuração do sistema incremental
	config := crawler.IncrementalConfig{
		EnableAI:             aiService != nil,   // Habilitar IA se disponível
		EnableFingerprinting: true,               // Habilitar detecção de mudanças
		MaxAge:               24 * time.Hour,     // Não reprocessar URLs por 24h
		AIThreshold:          6 * time.Hour,      // Não usar IA por 6h se não houve mudanças
		CleanupInterval:      7 * 24 * time.Hour, // Limpar registros antigos a cada 7 dias
		MaxConcurrency:       3,                  // Máximo 3 requisições simultâneas
		DelayBetweenRequests: 2 * time.Second,    // 2s entre requisições
		UserAgent:           "Go-Crawler-Incremental/1.0",
	}

	// Criar e iniciar o crawler incremental
	engine := crawler.NewIncrementalCrawlerEngine(s.repo, s.urlRepo, aiService, config)

	s.logger.Info("Starting incremental crawler engine")
	if err := engine.Start(ctx, urls); err != nil {
		s.logger.Error("Incremental crawler engine failed", err)
		return fmt.Errorf("erro no crawler incremental: %v", err)
	}

	// Log das estatísticas finais
	stats := engine.GetStatistics()
	s.logger.WithFields(map[string]interface{}{
		"total_urls":           stats.TotalURLs,
		"processed_urls":       stats.ProcessedURLs,
		"skipped_urls":         stats.SkippedURLs,
		"new_properties":       stats.NewProperties,
		"updated_properties":   stats.UpdatedProperties,
		"failed_urls":          stats.FailedURLs,
		"ai_processing_count":  stats.AIProcessingCount,
		"ai_skipped_count":     stats.AISkippedCount,
		"fingerprint_hits":     stats.FingerprintHits,
		"fingerprint_misses":   stats.FingerprintMisses,
		"content_changes":      stats.ContentChanges,
		"processing_time":      stats.ProcessingTimeTotal.String(),
		"ai_savings_estimate":  stats.AISavingsEstimate.String(),
		"success_rate":         s.calculateSuccessRate(stats.NewProperties+stats.UpdatedProperties, stats.ProcessedURLs),
		"skip_rate":            s.calculateSkipRate(stats.SkippedURLs, stats.TotalURLs),
	}).Info("Incremental crawling process completed")

	// Executar limpeza de registros antigos se necessário
	if err := s.urlRepo.CleanupOldRecords(ctx, config.CleanupInterval); err != nil {
		s.logger.WithError(err).Warn("Failed to cleanup old records")
	}

	return nil
}

// loadURLsFromFile carrega URLs do arquivo de configuração
func (s *PropertyService) loadURLsFromFile(filePath string) ([]string, error) {
	var urls []string
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo %s: %v", filePath, err)
	}

	if err := json.Unmarshal(data, &urls); err != nil {
		return nil, fmt.Errorf("erro ao fazer parse do JSON: %v", err)
	}

	return urls, nil
}

// calculateSuccessRate calcula a taxa de sucesso
func (s *PropertyService) calculateSuccessRate(saved, found int) float64 {
	if found == 0 {
		return 0
	}
	return float64(saved) / float64(found) * 100
}

// calculateSkipRate calcula a taxa de URLs puladas (otimização)
func (s *PropertyService) calculateSkipRate(skipped, visited int) float64 {
	if visited == 0 {
		return 0
	}
	return float64(skipped) / float64(visited) * 100
}
