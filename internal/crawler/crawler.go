package crawler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gocolly/colly"
	"go-crawler-project/internal/repository"
)

type Property struct {
	Endereco  string `json:"endereco"`
	Cidade    string `json:"cidade"`
	Descricao string `json:"descricao"`
	Valor     string `json:"valor"`
}

func StartCrawling(ctx context.Context, repo repository.PropertyRepository, urls []string) {
	c := colly.NewCollector()

	c.OnHTML(".property-listing", func(e *colly.HTMLElement) {
		property := Property{
			Endereco:  e.ChildText(".property-address"),
			Cidade:    e.ChildText(".property-city"),
			Descricao: e.ChildText(".property-description"),
			Valor:     e.ChildText(".property-price"),
		}

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