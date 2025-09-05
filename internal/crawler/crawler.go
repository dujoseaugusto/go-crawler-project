package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
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
	// Coletor principal para navegar nas páginas iniciais
	c := colly.NewCollector(
		colly.MaxDepth(10), // Limite de profundidade de links
		colly.Async(true),  // Permite coleta assíncrona
	)
	
	// Adiciona extensões úteis como User-Agent aleatório
	extensions.RandomUserAgent(c)
	extensions.Referer(c)
	
	// Coletor para páginas de detalhes de imóveis
	detailCollector := c.Clone()
	
	// Controle de concorrência
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1,
	})
	
	// Controle para evitar visitar a mesma URL várias vezes
	visitedURLs := make(map[string]bool)
	var visitedMutex sync.Mutex
	
	// Função para verificar se uma URL já foi visitada
	isVisited := func(url string) bool {
		visitedMutex.Lock()
		defer visitedMutex.Unlock()
		return visitedURLs[url]
	}
	
	// Função para marcar uma URL como visitada
	markVisited := func(url string) {
		visitedMutex.Lock()
		visitedURLs[url] = true
		visitedMutex.Unlock()
	}

	// Procura por links para páginas de detalhes de imóveis
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteLink := e.Request.AbsoluteURL(link)
		
		// Ignora links vazios ou externos
		if absoluteLink == "" || !strings.Contains(absoluteLink, e.Request.URL.Host) {
			return
		}
		
		// Verifica se parece ser um link para detalhes de imóvel
		if strings.Contains(absoluteLink, "imovel") || 
		   strings.Contains(absoluteLink, "propriedade") || 
		   strings.Contains(absoluteLink, "casa") || 
		   strings.Contains(absoluteLink, "apartamento") || 
		   strings.Contains(absoluteLink, "comprar") || 
		   strings.Contains(absoluteLink, "venda") {
			
			if !isVisited(absoluteLink) {
				markVisited(absoluteLink)
				log.Printf("Encontrou link de imóvel: %s", absoluteLink)
				detailCollector.Visit(absoluteLink)
			}
		}
	})

	// Procura por dados de imóveis nas páginas de detalhes
	detailCollector.OnHTML("body", func(e *colly.HTMLElement) {
		// Tenta vários padrões comuns de classes/IDs para dados de imóveis
		endereco := getFirstNonEmpty(e,
			".endereco", ".address", ".property-address", "[itemprop=address]",
			"#endereco", "#address", ".street-address", ".location")
		
		cidade := getFirstNonEmpty(e,
			".cidade", ".city", ".property-city", "[itemprop=addressLocality]",
			"#cidade", "#city", ".locality", ".neighborhood")
		
		descricao := getFirstNonEmpty(e,
			".descricao", ".description", ".property-description", "[itemprop=description]",
			"#descricao", "#description", ".details", ".features")
		
		valor := getFirstNonEmpty(e,
			".valor", ".price", ".property-price", "[itemprop=price]",
			"#valor", "#price", ".value", ".amount")
		
		// Se encontrou pelo menos endereço e valor, considera um imóvel válido
		if endereco != "" && valor != "" {
			property := mapToProperty(
				endereco,
				cidade,
				descricao,
				valor,
			)
			
			log.Printf("Imóvel encontrado: %s - %s", endereco, valor)
			
			if err := repo.Save(ctx, property); err != nil {
				log.Printf("Error saving property: %v", err)
			}
		}
	})

	c.OnRequest(func(r *colly.Request) {
		log.Printf("Visitando página de listagem: %s", r.URL)
	})
	
	detailCollector.OnRequest(func(r *colly.Request) {
		log.Printf("Visitando página de detalhes: %s", r.URL)
	})

	// Inicia a coleta a partir das URLs iniciais
	for _, url := range urls {
		if !isVisited(url) {
			markVisited(url)
			if err := c.Visit(url); err != nil {
				log.Printf("Error visiting URL %s: %v", url, err)
			}
		}
	}
	
	// Aguarda a conclusão de todas as solicitações
	c.Wait()
	detailCollector.Wait()
}

// getFirstNonEmpty tenta vários seletores e retorna o primeiro que encontrar conteúdo
func getFirstNonEmpty(e *colly.HTMLElement, selectors ...string) string {
	for _, selector := range selectors {
		text := strings.TrimSpace(e.ChildText(selector))
		if text != "" {
			return text
		}
	}
	return ""
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
