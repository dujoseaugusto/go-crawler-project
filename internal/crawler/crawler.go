package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
)

type Crawler struct {
	config *config.Config
	repo   repository.PropertyRepository
}

func NewCrawler(cfg *config.Config) *Crawler {
	repo, err := repository.NewMongoRepository(cfg.MongoURI, "crawler", "properties")
	if err != nil {
		log.Fatalf("Failed to create MongoDB repository: %v", err)
	}
	return &Crawler{
		config: cfg,
		repo:   repo,
	}
}

func (c *Crawler) StartCrawling(ctx context.Context) error {
	urls, err := LoadURLsFromFile(c.config.SitesFile)
	if err != nil {
		return fmt.Errorf("failed to load URLs: %v", err)
	}
	
	StartCrawling(ctx, c.repo, urls)
	return nil
}

func mapToProperty(endereco, cidade, descricao, valor string) repository.Property {
	return repository.Property{
		Endereco:  endereco,
		Cidade:    cidade,
		Descricao: descricao,
		Valor:     valor,
	}
}

func StartCrawling(ctx context.Context, repo repository.PropertyRepository, urls []string) {
	c := colly.NewCollector()

	c.OnHTML(".property-listing", func(e *colly.HTMLElement) {
		property := mapToProperty(
			e.ChildText(".property-address"),
			e.ChildText(".property-city"),
			e.ChildText(".property-description"),
			e.ChildText(".property-price"),
		)

		if err := repo.SaveProperty(ctx, property); err != nil {
			log.Printf("Error saving property: %v", err)
		}
	})

	c.OnRequest(func(r *colly.Request) {
		log.Printf("Visiting: %s", r.URL)
	})

	for _, url := range urls {
		if err := c.Visit(url); err != nil {
			log.Printf("Error visiting URL %s: %v", url, err)
		}
	}
}

func LoadURLsFromFile(filePath string) ([]string, error) {
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
