package service

import (
	"context"
	"errors"

	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
)

type PropertyService struct {
	repo repository.PropertyRepository
	config *config.Config
}

func NewPropertyService(repo repository.PropertyRepository, cfg *config.Config) *PropertyService {
	return &PropertyService{
		repo: repo,
		config: cfg,
	}
}

func (s *PropertyService) SaveProperty(ctx context.Context, property repository.Property) error {
	if property.Endereco == "" || property.Cidade == "" || property.Descricao == "" || property.Valor == "" {
		return errors.New("property fields cannot be empty")
	}
	return s.repo.Save(ctx, property)
}

func (s *PropertyService) GetAllProperties(ctx context.Context) ([]repository.Property, error) {
	return s.repo.FindAll(ctx)
}

// ForceCrawling inicia manualmente o processo de coleta de dados
func (s *PropertyService) ForceCrawling(ctx context.Context) error {
	// Carrega as URLs do arquivo de configuração
	urls, err := crawler.LoadURLsFromFile(s.config.SitesFile)
	if err != nil {
		return err
	}
	
	// Inicia o processo de coleta
	crawler.StartCrawling(ctx, s.repo, urls)
	return nil
}
