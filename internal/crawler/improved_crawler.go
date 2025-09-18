package crawler

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

// ImprovedCrawler é uma versão melhorada do crawler que usa padrões de referência
type ImprovedCrawler struct {
	config             *config.Config
	repo               repository.PropertyRepository
	aiService          *ai.GeminiService
	referenceTrainer   *ReferencePatternTrainer
	patternValidator   *PatternValidator
	enhancedExtractor  *EnhancedExtractor
	advancedClassifier *AdvancedPageClassifier
	contentLearner     *ContentBasedPatternLearner
	logger             *logger.Logger
	collector          *colly.Collector
	detailCollector    *colly.Collector
	visitedURLs        map[string]bool
	visitedMutex       sync.Mutex
	stats              *ImprovedCrawlerStats
	isTrainingMode     bool
}

// ImprovedCrawlerStats mantém estatísticas do crawler melhorado
type ImprovedCrawlerStats struct {
	PagesVisited      int            `json:"pages_visited"`
	PropertiesFound   int            `json:"properties_found"`
	PropertiesSaved   int            `json:"properties_saved"`
	CatalogPagesFound int            `json:"catalog_pages_found"`
	ErrorsEncountered int            `json:"errors_encountered"`
	StartTime         time.Time      `json:"start_time"`
	LastUpdate        time.Time      `json:"last_update"`
	AverageConfidence float64        `json:"average_confidence"`
	DomainStats       map[string]int `json:"domain_stats"`
	mutex             sync.RWMutex
}

// NewImprovedCrawler cria um novo crawler melhorado
func NewImprovedCrawler(ctx context.Context, cfg *config.Config) *ImprovedCrawler {
	// Inicializa repositório
	repo, err := repository.NewMongoRepository(cfg.MongoURI, "crawler", "properties")
	if err != nil {
		log.Fatalf("Failed to create MongoDB repository: %v", err)
	}

	// Inicializa serviço de IA
	aiService, err := ai.NewGeminiService(ctx)
	if err != nil {
		log.Printf("Warning: Failed to create Gemini service: %v", err)
	}

	// Inicializa componentes de aprendizado
	referenceTrainer := NewReferencePatternTrainer()
	patternValidator := NewPatternValidator(referenceTrainer)
	enhancedExtractor := NewEnhancedExtractor(referenceTrainer, patternValidator)
	advancedClassifier := NewAdvancedPageClassifier()
	contentLearner := NewContentBasedPatternLearner()

	// Configura coletores
	mainCollector := colly.NewCollector(
		colly.MaxDepth(10),
		colly.Async(true),
	)

	detailCollector := mainCollector.Clone()

	// Configurações dos coletores
	extensions.RandomUserAgent(mainCollector)
	extensions.Referer(mainCollector)
	extensions.RandomUserAgent(detailCollector)
	extensions.Referer(detailCollector)

	mainCollector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
	})

	detailCollector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	return &ImprovedCrawler{
		config:             cfg,
		repo:               repo,
		aiService:          aiService,
		referenceTrainer:   referenceTrainer,
		patternValidator:   patternValidator,
		enhancedExtractor:  enhancedExtractor,
		advancedClassifier: advancedClassifier,
		contentLearner:     contentLearner,
		logger:             logger.NewLogger("improved_crawler"),
		collector:          mainCollector,
		detailCollector:    detailCollector,
		visitedURLs:        make(map[string]bool),
		stats: &ImprovedCrawlerStats{
			StartTime:   time.Now(),
			DomainStats: make(map[string]int),
		},
	}
}

// TrainFromReferenceFile treina o crawler usando arquivo de referência
func (ic *ImprovedCrawler) TrainFromReferenceFile(ctx context.Context, referenceFile string) error {
	ic.logger.WithField("reference_file", referenceFile).Info("Starting training from reference file")

	ic.isTrainingMode = true
	defer func() { ic.isTrainingMode = false }()

	// Treina padrões de referência
	if err := ic.referenceTrainer.TrainFromReferenceFile(ctx, referenceFile); err != nil {
		return fmt.Errorf("failed to train reference patterns: %w", err)
	}

	// Atualiza extrator com novos padrões
	ic.enhancedExtractor.loadDomainSpecificSelectors()

	// Valida padrões aprendidos
	urls, err := ic.referenceTrainer.loadURLsFromFile(referenceFile)
	if err != nil {
		return fmt.Errorf("failed to load URLs for validation: %w", err)
	}

	if err := ic.patternValidator.ValidateAndImprovePatterns(ctx, urls); err != nil {
		ic.logger.WithError(err).Warn("Pattern validation completed with warnings")
	}

	ic.logger.Info("Training completed successfully")
	return nil
}

// StartCrawling inicia o processo de crawling melhorado
func (ic *ImprovedCrawler) StartCrawling(ctx context.Context) error {
	urls, err := LoadURLsFromFile(ic.config.SitesFile)
	if err != nil {
		return fmt.Errorf("failed to load URLs: %w", err)
	}

	ic.logger.WithField("url_count", len(urls)).Info("Starting improved crawling")
	ic.stats.StartTime = time.Now()

	// Configura handlers do crawler
	ic.setupCrawlerHandlers(ctx)

	// Inicia crawling das URLs iniciais
	for _, url := range urls {
		if !ic.isVisited(url) {
			ic.markVisited(url)
			if err := ic.collector.Visit(url); err != nil {
				ic.logger.WithField("url", url).Error("Failed to visit URL", err)
				ic.updateStats("error", url)
			}
		}
	}

	// Aguarda conclusão
	ic.collector.Wait()
	ic.detailCollector.Wait()

	// Processa buffer restante da IA
	if ic.aiService != nil {
		if err := ic.aiService.FlushBatch(ctx); err != nil {
			ic.logger.WithError(err).Warn("Failed to flush AI batch")
		}
	}

	ic.logFinalStats()
	return nil
}

// setupCrawlerHandlers configura os handlers do crawler
func (ic *ImprovedCrawler) setupCrawlerHandlers(ctx context.Context) {
	// Handler para encontrar links de propriedades
	ic.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		ic.handlePropertyLink(e)
	})

	// Handler principal para páginas de detalhes
	ic.detailCollector.OnHTML("body", func(e *colly.HTMLElement) {
		ic.handlePropertyPage(ctx, e)
	})

	// Handlers de log
	ic.collector.OnRequest(func(r *colly.Request) {
		ic.logger.WithField("url", r.URL.String()).Debug("Visiting listing page")
	})

	ic.detailCollector.OnRequest(func(r *colly.Request) {
		ic.logger.WithField("url", r.URL.String()).Debug("Visiting property page")
	})

	ic.collector.OnError(func(r *colly.Response, err error) {
		ic.logger.WithField("url", r.Request.URL.String()).Error("Error visiting listing page", err)
		ic.updateStats("error", r.Request.URL.String())
	})

	ic.detailCollector.OnError(func(r *colly.Response, err error) {
		ic.logger.WithField("url", r.Request.URL.String()).Error("Error visiting property page", err)
		ic.updateStats("error", r.Request.URL.String())
	})
}

// handlePropertyLink processa links encontrados em páginas de listagem
func (ic *ImprovedCrawler) handlePropertyLink(e *colly.HTMLElement) {
	link := e.Attr("href")
	absoluteLink := e.Request.AbsoluteURL(link)

	// Ignora links vazios ou externos
	if absoluteLink == "" || !strings.Contains(absoluteLink, e.Request.URL.Host) {
		return
	}

	// Usa padrões aprendidos para verificar se é link de propriedade
	matchedPattern, confidence := ic.referenceTrainer.MatchURL(absoluteLink)

	isPropertyLink := false

	if matchedPattern != nil && confidence > 0.5 {
		isPropertyLink = true
		ic.logger.WithFields(map[string]interface{}{
			"url":        absoluteLink,
			"pattern_id": matchedPattern.ID,
			"confidence": confidence,
		}).Debug("Property link found using learned pattern")
	} else {
		// Fallback para detecção baseada em URL
		isPropertyLink = ic.isLikelyPropertyURL(absoluteLink)
	}

	if isPropertyLink && !ic.isVisited(absoluteLink) {
		ic.markVisited(absoluteLink)
		ic.logger.WithField("url", absoluteLink).Info("Found property link")
		ic.detailCollector.Visit(absoluteLink)
	}
}

// handlePropertyPage processa páginas de propriedade
func (ic *ImprovedCrawler) handlePropertyPage(ctx context.Context, e *colly.HTMLElement) {
	url := e.Request.URL.String()
	ic.updateStats("page_visited", url)

	// Classifica a página usando classificador rigoroso
	pageType := ic.advancedClassifier.ClassifyPageStrict(e)

	// Verifica também com classificador baseado em conteúdo
	contentType, contentConfidence := ic.contentLearner.ClassifyPageContent(e)

	ic.logger.WithFields(map[string]interface{}{
		"url":                url,
		"page_type":          pageType,
		"content_type":       contentType,
		"content_confidence": contentConfidence,
	}).Debug("Page classification completed")

	// Decide se deve processar como propriedade individual
	shouldProcess := false

	if pageType == PageTypeProperty && contentType == "property" {
		shouldProcess = true
	} else if pageType == PageTypeProperty || (contentType == "property" && contentConfidence > 0.7) {
		shouldProcess = true
	} else if pageType == PageTypeCatalog || contentType == "catalog" {
		ic.logger.WithField("url", url).Debug("Page identified as catalog, extracting individual properties")
		ic.handleCatalogPage(ctx, e)
		return
	}

	if shouldProcess {
		ic.processPropertyPage(ctx, e, url)
	} else {
		ic.logger.WithField("url", url).Debug("Page not identified as property, skipping")
	}
}

// processPropertyPage processa uma página de propriedade individual
func (ic *ImprovedCrawler) processPropertyPage(ctx context.Context, e *colly.HTMLElement, url string) {
	ic.logger.WithField("url", url).Info("Processing property page")

	// Extrai dados usando extrator melhorado
	property := ic.enhancedExtractor.ExtractPropertyData(e, url)

	// Valida se os dados extraídos são suficientes
	if !ic.isValidProperty(property) {
		ic.logger.WithField("url", url).Warn("Extracted property data is insufficient")
		return
	}

	ic.updateStats("property_found", url)

	// Processa com IA se disponível
	if ic.aiService != nil {
		processedProperty, err := ic.aiService.ProcessPropertyData(ctx, property)
		if err != nil {
			ic.logger.WithError(err).WithField("url", url).Warn("AI processing failed, using original data")
		} else {
			property = processedProperty
			ic.logger.WithField("url", url).Debug("Property data enhanced by AI")
		}
	}

	// Salva propriedade
	if err := ic.repo.Save(ctx, property); err != nil {
		ic.logger.WithField("url", url).Error("Failed to save property", err)
		ic.updateStats("error", url)
	} else {
		ic.updateStats("property_saved", url)
		ic.logger.WithFields(map[string]interface{}{
			"url":           url,
			"address":       property.Endereco,
			"price":         property.Valor,
			"property_type": property.TipoImovel,
		}).Info("Property saved successfully")
	}
}

// handleCatalogPage processa páginas de catálogo extraindo propriedades individuais
func (ic *ImprovedCrawler) handleCatalogPage(ctx context.Context, e *colly.HTMLElement) {
	url := e.Request.URL.String()
	ic.updateStats("catalog_found", url)

	// Extrai links de propriedades individuais do catálogo
	propertyLinks := ic.extractPropertyLinksFromCatalog(e)

	ic.logger.WithFields(map[string]interface{}{
		"url":            url,
		"property_links": len(propertyLinks),
	}).Info("Processing catalog page")

	// Visita cada link de propriedade encontrado
	for _, link := range propertyLinks {
		if !ic.isVisited(link) && len(propertyLinks) <= 20 { // Limita para evitar sobrecarga
			ic.markVisited(link)
			ic.detailCollector.Visit(link)
		}
	}

	// Se não encontrou links, tenta extrair dados diretamente do catálogo
	if len(propertyLinks) == 0 {
		ic.extractPropertiesFromCatalogContent(ctx, e)
	}
}

// extractPropertyLinksFromCatalog extrai links de propriedades de uma página de catálogo
func (ic *ImprovedCrawler) extractPropertyLinksFromCatalog(e *colly.HTMLElement) []string {
	var links []string

	e.ForEach("a[href]", func(_ int, el *colly.HTMLElement) {
		href := el.Attr("href")
		absoluteLink := e.Request.AbsoluteURL(href)

		// Ignora links externos ou inválidos
		if !strings.Contains(absoluteLink, e.Request.URL.Host) ||
			strings.Contains(absoluteLink, "whatsapp") ||
			strings.Contains(absoluteLink, "mailto:") {
			return
		}

		// Verifica se parece ser link de propriedade
		if ic.isLikelyPropertyURL(absoluteLink) {
			links = append(links, absoluteLink)
		}
	})

	return links
}

// extractPropertiesFromCatalogContent extrai propriedades diretamente do conteúdo do catálogo
func (ic *ImprovedCrawler) extractPropertiesFromCatalogContent(ctx context.Context, e *colly.HTMLElement) {
	url := e.Request.URL.String()

	// Procura por elementos que representam propriedades individuais
	e.ForEach(".property-card, .imovel-item, .listing-item, .property, .imovel", func(i int, el *colly.HTMLElement) {
		if i >= 10 { // Limita para evitar spam
			return
		}

		// Cria um elemento temporário para extração
		tempElement := &colly.HTMLElement{
			Name:     "div",
			Text:     el.Text,
			Request:  e.Request,
			Response: e.Response,
			DOM:      el.DOM,
		}

		property := ic.enhancedExtractor.ExtractPropertyData(tempElement, url)

		if ic.isValidProperty(property) {
			ic.updateStats("property_found", url)

			if err := ic.repo.Save(ctx, property); err != nil {
				ic.logger.WithError(err).Warn("Failed to save catalog property")
			} else {
				ic.updateStats("property_saved", url)
			}
		}
	})
}

// isLikelyPropertyURL verifica se uma URL parece ser de uma propriedade individual
func (ic *ImprovedCrawler) isLikelyPropertyURL(url string) bool {
	urlLower := strings.ToLower(url)

	// Padrões que indicam propriedade individual
	propertyPatterns := []string{
		"/imovel/", "/propriedade/", "/anuncio/", "/detalhes/",
		"/casa/", "/apartamento/", "/ref-", "/codigo-", "/id-",
	}

	for _, pattern := range propertyPatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}

	// Verifica se termina com número (comum em URLs de propriedade)
	if strings.Contains(urlLower, "/") {
		parts := strings.Split(urlLower, "/")
		lastPart := parts[len(parts)-1]
		if len(lastPart) > 3 && strings.ContainsAny(lastPart, "0123456789") {
			return true
		}
	}

	return false
}

// isValidProperty verifica se uma propriedade tem dados suficientes
func (ic *ImprovedCrawler) isValidProperty(property repository.Property) bool {
	// Deve ter pelo menos endereço OU valor
	hasBasicInfo := property.Endereco != "" || property.Valor > 0

	// Deve ter alguma descrição ou características
	hasDetails := property.Descricao != "" || len(property.Caracteristicas) > 0

	// Validações de qualidade
	validAddress := property.Endereco == "" || (len(property.Endereco) > 10 && len(property.Endereco) < 300)
	validDescription := property.Descricao == "" || (len(property.Descricao) > 20 && len(property.Descricao) < 2000)
	validPrice := property.Valor >= 0 && property.Valor < 100000000 // Máximo 100 milhões

	return hasBasicInfo && hasDetails && validAddress && validDescription && validPrice
}

// Funções de controle de URLs visitadas
func (ic *ImprovedCrawler) isVisited(url string) bool {
	ic.visitedMutex.Lock()
	defer ic.visitedMutex.Unlock()
	return ic.visitedURLs[url]
}

func (ic *ImprovedCrawler) markVisited(url string) {
	ic.visitedMutex.Lock()
	ic.visitedURLs[url] = true
	ic.visitedMutex.Unlock()
}

// updateStats atualiza estatísticas do crawler
func (ic *ImprovedCrawler) updateStats(statType, rawURL string) {
	ic.stats.mutex.Lock()
	defer ic.stats.mutex.Unlock()

	ic.stats.LastUpdate = time.Now()

	switch statType {
	case "page_visited":
		ic.stats.PagesVisited++
	case "property_found":
		ic.stats.PropertiesFound++
	case "property_saved":
		ic.stats.PropertiesSaved++
	case "catalog_found":
		ic.stats.CatalogPagesFound++
	case "error":
		ic.stats.ErrorsEncountered++
	}

	// Atualiza estatísticas por domínio
	if rawURL != "" {
		if parsedURL, err := url.Parse(rawURL); err == nil {
			ic.stats.DomainStats[parsedURL.Host]++
		}
	}
}

// GetStats retorna estatísticas atuais do crawler
func (ic *ImprovedCrawler) GetStats() *ImprovedCrawlerStats {
	ic.stats.mutex.RLock()
	defer ic.stats.mutex.RUnlock()

	// Cria cópia das estatísticas
	stats := &ImprovedCrawlerStats{
		PagesVisited:      ic.stats.PagesVisited,
		PropertiesFound:   ic.stats.PropertiesFound,
		PropertiesSaved:   ic.stats.PropertiesSaved,
		CatalogPagesFound: ic.stats.CatalogPagesFound,
		ErrorsEncountered: ic.stats.ErrorsEncountered,
		StartTime:         ic.stats.StartTime,
		LastUpdate:        ic.stats.LastUpdate,
		DomainStats:       make(map[string]int),
	}

	for domain, count := range ic.stats.DomainStats {
		stats.DomainStats[domain] = count
	}

	return stats
}

// logFinalStats registra estatísticas finais
func (ic *ImprovedCrawler) logFinalStats() {
	stats := ic.GetStats()
	duration := time.Since(stats.StartTime)

	ic.logger.WithFields(map[string]interface{}{
		"duration":          duration.String(),
		"pages_visited":     stats.PagesVisited,
		"properties_found":  stats.PropertiesFound,
		"properties_saved":  stats.PropertiesSaved,
		"catalog_pages":     stats.CatalogPagesFound,
		"errors":            stats.ErrorsEncountered,
		"success_rate":      float64(stats.PropertiesSaved) / float64(stats.PropertiesFound) * 100,
		"domains_processed": len(stats.DomainStats),
	}).Info("Crawling completed")

	// Log estatísticas por domínio
	for domain, count := range stats.DomainStats {
		ic.logger.WithFields(map[string]interface{}{
			"domain": domain,
			"pages":  count,
		}).Debug("Domain statistics")
	}
}
