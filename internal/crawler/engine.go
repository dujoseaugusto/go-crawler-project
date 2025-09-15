package crawler

import (
	"context"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

// CrawlerEngine é o motor principal do crawler refatorado
type CrawlerEngine struct {
	extractor  *DataExtractor
	urlManager *URLManager
	validator  *PropertyValidator
	repository repository.PropertyRepository
	aiService  *ai.GeminiService
	logger     *logger.Logger
	config     *CrawlerConfig
	stats      *CrawlerStats
}

// CrawlerConfig contém configurações do crawler
type CrawlerConfig struct {
	MaxDepth       int
	Parallelism    int
	Delay          time.Duration
	MaxURLs        int
	EnableAI       bool
	BatchSize      int
	RequestTimeout time.Duration
}

// CrawlerStats mantém estatísticas do crawler
type CrawlerStats struct {
	URLsVisited     int
	PropertiesFound int
	PropertiesSaved int
	ErrorsCount     int
	StartTime       time.Time
	mutex           sync.RWMutex
}

// NewCrawlerEngine cria um novo motor de crawler
func NewCrawlerEngine(repo repository.PropertyRepository, aiService *ai.GeminiService) *CrawlerEngine {
	config := &CrawlerConfig{
		MaxDepth:       10,
		Parallelism:    3,
		Delay:          2 * time.Second,
		MaxURLs:        10000,
		EnableAI:       aiService != nil,
		BatchSize:      10,
		RequestTimeout: 30 * time.Second,
	}

	return &CrawlerEngine{
		extractor:  NewDataExtractor(),
		urlManager: NewURLManager(),
		validator:  NewPropertyValidator(),
		repository: repo,
		aiService:  aiService,
		logger:     logger.NewLogger("crawler_engine"),
		config:     config,
		stats:      &CrawlerStats{StartTime: time.Now()},
	}
}

// Start inicia o processo de crawling
func (ce *CrawlerEngine) Start(ctx context.Context, urls []string) error {
	ce.logger.WithFields(map[string]interface{}{
		"initial_urls": len(urls),
		"config":       ce.config,
	}).Info("Starting crawler engine")

	// Configura o coletor principal
	collector := ce.setupCollector()

	// Configura handlers
	ce.setupHandlers(ctx, collector)

	// Inicia o crawling
	for _, url := range urls {
		if ce.urlManager.ShouldSkipURL(url) {
			continue
		}

		ce.logger.WithField("url", url).Info("Starting crawl")
		if err := collector.Visit(url); err != nil {
			ce.logger.WithField("url", url).Error("Failed to visit initial URL", err)
			ce.incrementErrorCount()
		}
	}

	// Aguarda conclusão
	collector.Wait()

	// Processa buffer restante da IA
	if ce.aiService != nil {
		if err := ce.aiService.FlushBatch(ctx); err != nil {
			ce.logger.Error("Failed to flush AI batch", err)
		}
	}

	// Log das estatísticas finais
	ce.logFinalStats()

	return nil
}

// setupCollector configura o coletor Colly
func (ce *CrawlerEngine) setupCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.MaxDepth(ce.config.MaxDepth),
		colly.Async(true),
	)

	// Adiciona extensões
	extensions.RandomUserAgent(c)
	extensions.Referer(c)

	// Configura rate limiting
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: ce.config.Parallelism,
		Delay:       ce.config.Delay,
	})

	// Timeout para requisições
	c.SetRequestTimeout(ce.config.RequestTimeout)

	return c
}

// setupHandlers configura os handlers do coletor
func (ce *CrawlerEngine) setupHandlers(ctx context.Context, c *colly.Collector) {
	// Handler para encontrar links de propriedades
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		ce.handlePropertyLinks(e, c)
	})

	// Handler principal para extrair dados
	c.OnHTML("body", func(e *colly.HTMLElement) {
		ce.handlePropertyData(ctx, e)
	})

	// Handler para requisições
	c.OnRequest(func(r *colly.Request) {
		ce.logger.WithField("url", r.URL.String()).Debug("Visiting URL")
		ce.incrementURLsVisited()
	})

	// Handler para erros
	c.OnError(func(r *colly.Response, err error) {
		ce.logger.WithFields(map[string]interface{}{
			"url":         r.Request.URL.String(),
			"status_code": r.StatusCode,
		}).Error("Request failed", err)
		ce.incrementErrorCount()
	})
}

// handlePropertyLinks processa links encontrados na página
func (ce *CrawlerEngine) handlePropertyLinks(e *colly.HTMLElement, c *colly.Collector) {
	link := e.Attr("href")
	absoluteLink := e.Request.AbsoluteURL(link)

	// Verifica se é um link válido e não visitado
	if ce.urlManager.ShouldSkipURL(absoluteLink) || ce.urlManager.IsVisited(absoluteLink) {
		return
	}

	// Verifica se parece ser um link de propriedade
	if ce.urlManager.IsValidPropertyLink(absoluteLink, e.Request.URL.Host) {
		ce.urlManager.MarkVisited(absoluteLink)

		// Limita o número de URLs para evitar memory leak
		if ce.urlManager.GetVisitedCount() > ce.config.MaxURLs {
			ce.urlManager.CleanupOldURLs(ce.config.MaxURLs / 2)
		}

		c.Visit(absoluteLink)
	}
}

// handlePropertyData processa dados de propriedade na página
func (ce *CrawlerEngine) handlePropertyData(ctx context.Context, e *colly.HTMLElement) {
	url := e.Request.URL.String()

	// Verifica se é uma página de catálogo
	if ce.urlManager.IsCatalogPage(e) {
		ce.handleCatalogPage(ctx, e)
		return
	}

	// Extrai dados da propriedade
	property := ce.extractor.ExtractProperty(e, url)
	ce.incrementPropertiesFound()

	// Valida os dados
	validation := ce.validator.ValidateProperty(property)
	if !validation.IsValid {
		ce.logger.WithFields(map[string]interface{}{
			"url":    url,
			"errors": validation.Errors,
		}).Debug("Property validation failed")
		return
	}

	// Melhora os dados
	property = ce.validator.EnhanceProperty(property)

	// Processa com IA se disponível e necessário
	if ce.config.EnableAI && ce.aiService != nil {
		if processedProperty, err := ce.aiService.ProcessPropertyData(ctx, *property); err == nil {
			property = &processedProperty
		} else {
			ce.logger.WithField("url", url).Warn("AI processing failed, using original data")
		}
	}

	// Salva no repositório
	if err := ce.repository.Save(ctx, *property); err != nil {
		ce.logger.WithField("url", url).Error("Failed to save property", err)
		ce.incrementErrorCount()
	} else {
		ce.incrementPropertiesSaved()
		ce.logger.WithFields(map[string]interface{}{
			"endereco": property.Endereco,
			"valor":    property.Valor,
			"tipo":     property.TipoImovel,
		}).Info("Property saved successfully")
	}
}

// handleCatalogPage processa páginas de catálogo
func (ce *CrawlerEngine) handleCatalogPage(ctx context.Context, e *colly.HTMLElement) {
	ce.logger.WithField("url", e.Request.URL.String()).Info("Processing catalog page")

	// Extrai links individuais
	links := ce.urlManager.ExtractPropertyLinks(e)

	// Se não encontrou links, tenta extrair dados individuais do catálogo
	if len(links) == 0 {
		ce.extractIndividualPropertiesFromCatalog(ctx, e)
	}
}

// extractIndividualPropertiesFromCatalog extrai propriedades individuais de catálogos
func (ce *CrawlerEngine) extractIndividualPropertiesFromCatalog(ctx context.Context, e *colly.HTMLElement) {
	url := e.Request.URL.String()
	var properties []*repository.Property

	// Procura por elementos que representam propriedades individuais
	e.ForEach(".imovel, .property, .card, .item, .listing", func(i int, el *colly.HTMLElement) {
		if i >= ce.config.BatchSize { // Limita para evitar spam
			return
		}

		property := ce.extractor.ExtractProperty(el, url)
		if ce.validator.IsValidForSaving(property) {
			properties = append(properties, property)
		}
	})

	// Salva as propriedades encontradas
	for _, property := range properties {
		ce.incrementPropertiesFound()

		if err := ce.repository.Save(ctx, *property); err != nil {
			ce.logger.Error("Failed to save catalog property", err)
			ce.incrementErrorCount()
		} else {
			ce.incrementPropertiesSaved()
		}
	}

	ce.logger.WithFields(map[string]interface{}{
		"url":        url,
		"properties": len(properties),
	}).Info("Catalog properties processed")
}

// Métodos para estatísticas (thread-safe)
func (ce *CrawlerEngine) incrementURLsVisited() {
	ce.stats.mutex.Lock()
	defer ce.stats.mutex.Unlock()
	ce.stats.URLsVisited++
}

func (ce *CrawlerEngine) incrementPropertiesFound() {
	ce.stats.mutex.Lock()
	defer ce.stats.mutex.Unlock()
	ce.stats.PropertiesFound++
}

func (ce *CrawlerEngine) incrementPropertiesSaved() {
	ce.stats.mutex.Lock()
	defer ce.stats.mutex.Unlock()
	ce.stats.PropertiesSaved++
}

func (ce *CrawlerEngine) incrementErrorCount() {
	ce.stats.mutex.Lock()
	defer ce.stats.mutex.Unlock()
	ce.stats.ErrorsCount++
}

// GetStats retorna estatísticas atuais
func (ce *CrawlerEngine) GetStats() CrawlerStats {
	ce.stats.mutex.RLock()
	defer ce.stats.mutex.RUnlock()

	// Retorna uma cópia sem o mutex para evitar problemas de concorrência
	return CrawlerStats{
		URLsVisited:     ce.stats.URLsVisited,
		PropertiesFound: ce.stats.PropertiesFound,
		PropertiesSaved: ce.stats.PropertiesSaved,
		ErrorsCount:     ce.stats.ErrorsCount,
		StartTime:       ce.stats.StartTime,
	}
}

// logFinalStats registra estatísticas finais
func (ce *CrawlerEngine) logFinalStats() {
	stats := ce.GetStats()
	duration := time.Since(stats.StartTime)

	ce.logger.WithFields(map[string]interface{}{
		"duration":         duration.String(),
		"urls_visited":     stats.URLsVisited,
		"properties_found": stats.PropertiesFound,
		"properties_saved": stats.PropertiesSaved,
		"errors":           stats.ErrorsCount,
		"success_rate":     float64(stats.PropertiesSaved) / float64(stats.PropertiesFound) * 100,
		"urls_per_minute":  float64(stats.URLsVisited) / duration.Minutes(),
	}).Info("Crawler execution completed")
}
