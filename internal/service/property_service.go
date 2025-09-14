package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
)

type PropertyService struct {
	repo   repository.PropertyRepository
	config *config.Config
	logger *logger.Logger
}

func NewPropertyService(repo repository.PropertyRepository, cfg *config.Config) *PropertyService {
	return &PropertyService{
		repo:   repo,
		config: cfg,
		logger: logger.NewLogger("property_service"),
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

// ForceCrawling inicia manualmente o processo de coleta de dados usando o novo crawler engine
func (s *PropertyService) ForceCrawling(ctx context.Context) error {
	s.logger.Info("Starting forced crawling process")

	// Limpa o banco de dados antes de iniciar nova coleta
	s.logger.Info("Clearing database before crawling")
	if err := s.repo.ClearAll(ctx); err != nil {
		s.logger.Error("Failed to clear database", err)
		return fmt.Errorf("erro ao limpar banco de dados: %v", err)
	}

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

	// Criar e iniciar o novo crawler engine
	engine := crawler.NewCrawlerEngine(s.repo, aiService)

	s.logger.Info("Starting crawler engine")
	if err := engine.Start(ctx, urls); err != nil {
		s.logger.Error("Crawler engine failed", err)
		return fmt.Errorf("erro no crawler: %v", err)
	}

	// Log das estatísticas finais
	stats := engine.GetStats()
	s.logger.WithFields(map[string]interface{}{
		"urls_visited":     stats.URLsVisited,
		"properties_found": stats.PropertiesFound,
		"properties_saved": stats.PropertiesSaved,
		"errors":           stats.ErrorsCount,
		"success_rate":     s.calculateSuccessRate(stats.PropertiesSaved, stats.PropertiesFound),
	}).Info("Crawling process completed")

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
