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
	repo          repository.PropertyRepository
	urlRepo       repository.URLRepository
	citySitesRepo repository.CitySitesRepository
	config        *config.Config
	logger        *logger.Logger
}

// CleanupOptions define as opções para limpeza do banco
type CleanupOptions struct {
	Properties bool `json:"properties"`
	URLs       bool `json:"urls"`
	All        bool `json:"all"`
}

// CleanupResult retorna o resultado da operação de limpeza
type CleanupResult struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	PropertiesCleared bool   `json:"properties_cleared"`
	URLsCleared       bool   `json:"urls_cleared"`
	Error             string `json:"error,omitempty"`
}

func NewPropertyService(repo repository.PropertyRepository, urlRepo repository.URLRepository, cfg *config.Config) *PropertyService {
	return &PropertyService{
		repo:          repo,
		urlRepo:       urlRepo,
		citySitesRepo: nil, // Será inicializado quando necessário
		config:        cfg,
		logger:        logger.NewLogger("property_service"),
	}
}

// NewPropertyServiceWithCitySites cria um PropertyService com repositório de cidades
func NewPropertyServiceWithCitySites(repo repository.PropertyRepository, urlRepo repository.URLRepository, citySitesRepo repository.CitySitesRepository, cfg *config.Config) *PropertyService {
	return &PropertyService{
		repo:          repo,
		urlRepo:       urlRepo,
		citySitesRepo: citySitesRepo,
		config:        cfg,
		logger:        logger.NewLogger("property_service"),
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
func (s *PropertyService) ForceCrawling(ctx context.Context, cities []string) error {
	s.logger.WithFields(map[string]interface{}{
		"cities": cities,
	}).Info("Starting incremental crawling process")

	var urls []string
	var err error
	var source string

	// Tenta carregar URLs do repositório de cidades primeiro
	if s.citySitesRepo != nil {
		if len(cities) == 0 {
			// Pega todos os sites ativos de todas as cidades
			urls, err = s.citySitesRepo.GetAllActiveSites(ctx)
			source = "database_all_cities"
		} else {
			// Pega sites apenas das cidades especificadas
			urls, err = s.citySitesRepo.GetSitesByCities(ctx, cities)
			source = "database_specific_cities"
		}

		if err != nil {
			s.logger.WithError(err).Warn("Failed to load URLs from city sites repository, falling back to sites.json")
		}
	}

	// Fallback para sites.json se não conseguiu carregar do banco ou não há sites
	if len(urls) == 0 {
		urls, err = s.loadURLsFromFile(s.config.SitesFile)
		if err != nil {
			s.logger.Error("Failed to load URLs from file", err)
			return fmt.Errorf("erro ao carregar URLs: %v", err)
		}
		source = "sites_json_fallback"
	}

	s.logger.WithFields(map[string]interface{}{
		"urls_count": len(urls),
		"source":     source,
		"cities":     cities,
	}).Info("URLs loaded successfully")

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
		UserAgent:            "Go-Crawler-Incremental/1.0",
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
		"total_urls":          stats.TotalURLs,
		"processed_urls":      stats.ProcessedURLs,
		"skipped_urls":        stats.SkippedURLs,
		"new_properties":      stats.NewProperties,
		"updated_properties":  stats.UpdatedProperties,
		"failed_urls":         stats.FailedURLs,
		"ai_processing_count": stats.AIProcessingCount,
		"ai_skipped_count":    stats.AISkippedCount,
		"fingerprint_hits":    stats.FingerprintHits,
		"fingerprint_misses":  stats.FingerprintMisses,
		"content_changes":     stats.ContentChanges,
		"processing_time":     stats.ProcessingTimeTotal.String(),
		"ai_savings_estimate": stats.AISavingsEstimate.String(),
		"success_rate":        s.calculateSuccessRate(stats.NewProperties+stats.UpdatedProperties, stats.ProcessedURLs),
		"skip_rate":           s.calculateSkipRate(stats.SkippedURLs, stats.TotalURLs),
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

// CleanupDatabase limpa o banco de dados conforme as opções especificadas
func (s *PropertyService) CleanupDatabase(ctx context.Context, options CleanupOptions) CleanupResult {
	result := CleanupResult{
		Success: false,
	}

	s.logger.WithFields(map[string]interface{}{
		"properties": options.Properties,
		"urls":       options.URLs,
		"all":        options.All,
	}).Info("Iniciando limpeza do banco de dados")

	// Se "all" está marcado, limpa tudo
	if options.All {
		options.Properties = true
		options.URLs = true
	}

	// Limpar propriedades
	if options.Properties {
		if err := s.repo.ClearAll(ctx); err != nil {
			result.Error = fmt.Sprintf("Erro ao limpar propriedades: %v", err)
			s.logger.Error("Erro ao limpar propriedades", err)
			return result
		}
		result.PropertiesCleared = true
		s.logger.Info("Propriedades limpas com sucesso")
	}

	// Limpar URLs processadas
	if options.URLs {
		if mongoURLRepo, ok := s.urlRepo.(*repository.MongoURLRepository); ok {
			// Limpa tudo (idade 0)
			if err := mongoURLRepo.CleanupOldRecords(ctx, 0); err != nil {
				result.Error = fmt.Sprintf("Erro ao limpar URLs: %v", err)
				s.logger.Error("Erro ao limpar URLs", err)
				return result
			}
			result.URLsCleared = true
			s.logger.Info("URLs limpas com sucesso")
		} else {
			result.Error = "Repositório de URLs não suporta limpeza"
			s.logger.WithField("type", fmt.Sprintf("%T", s.urlRepo)).Info("Repositório de URLs não é do tipo MongoURLRepository")
			return result
		}
	}

	// Determinar mensagem de sucesso
	if result.PropertiesCleared && result.URLsCleared {
		result.Message = "Banco de dados completamente limpo (propriedades + URLs)"
	} else if result.PropertiesCleared {
		result.Message = "Propriedades limpas com sucesso"
	} else if result.URLsCleared {
		result.Message = "URLs processadas limpas com sucesso"
	} else {
		result.Message = "Nenhuma operação de limpeza realizada"
	}

	result.Success = true
	s.logger.WithField("message", result.Message).Info("Limpeza concluída com sucesso")

	return result
}
