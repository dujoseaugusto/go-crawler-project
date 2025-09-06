package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
)

type PropertyService struct {
	repo   repository.PropertyRepository
	config *config.Config
}

func NewPropertyService(repo repository.PropertyRepository, cfg *config.Config) *PropertyService {
	return &PropertyService{
		repo:   repo,
		config: cfg,
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

// ForceCrawling inicia manualmente o processo de coleta de dados
func (s *PropertyService) ForceCrawling(ctx context.Context) error {
	// Limpa o banco de dados antes de iniciar nova coleta
	if err := s.repo.ClearAll(ctx); err != nil {
		return fmt.Errorf("erro ao limpar banco de dados: %v", err)
	}

	// Carrega as URLs do arquivo de configuração
	urls, err := crawler.LoadURLsFromFile(s.config.SitesFile)
	if err != nil {
		return err
	}

	// Inicializar o serviço de IA
	aiService, err := ai.NewGeminiService(ctx)
	if err != nil {
		// Se não conseguir inicializar a IA, continua sem ela
		crawler.StartCrawling(ctx, s.repo, urls, nil)
	} else {
		// Inicia o processo de coleta com IA
		crawler.StartCrawling(ctx, s.repo, urls, aiService)
	}
	return nil
}
