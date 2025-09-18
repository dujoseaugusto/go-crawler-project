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

// AIIntegratedCrawler é um crawler que integra completamente IA com padrões de referência
type AIIntegratedCrawler struct {
	config            *config.Config
	repo              repository.PropertyRepository
	aiService         *ai.GeminiService
	enhancedAI        *ai.EnhancedGeminiService
	aiTrainer         *AIEnhancedTrainer
	patternValidator  *PatternValidator
	enhancedExtractor *EnhancedExtractor
	logger            *logger.Logger
	collector         *colly.Collector
	detailCollector   *colly.Collector
	visitedURLs       map[string]bool
	visitedMutex      sync.Mutex
	stats             *AIIntegratedStats
}

// AIIntegratedStats estatísticas específicas para crawler com IA
type AIIntegratedStats struct {
	PagesVisited          int                    `json:"pages_visited"`
	PropertiesFound       int                    `json:"properties_found"`
	PropertiesSaved       int                    `json:"properties_saved"`
	AIClassifications     int                    `json:"ai_classifications"`
	AIValidations         int                    `json:"ai_validations"`
	AIEnhancements        int                    `json:"ai_enhancements"`
	PatternMatches        int                    `json:"pattern_matches"`
	HighConfidenceMatches int                    `json:"high_confidence_matches"`
	StartTime             time.Time              `json:"start_time"`
	LastUpdate            time.Time              `json:"last_update"`
	DomainStats           map[string]int         `json:"domain_stats"`
	AIPerformanceStats    map[string]interface{} `json:"ai_performance_stats"`
	mutex                 sync.RWMutex
}

// NewAIIntegratedCrawler cria um novo crawler integrado com IA
func NewAIIntegratedCrawler(ctx context.Context, cfg *config.Config) (*AIIntegratedCrawler, error) {
	// Inicializa repositório
	repo, err := repository.NewMongoRepository(cfg.MongoURI, "crawler", "properties")
	if err != nil {
		log.Fatalf("Failed to create MongoDB repository: %v", err)
	}

	// Inicializa serviços de IA
	aiService, err := ai.NewGeminiService(ctx)
	if err != nil {
		log.Printf("Warning: Failed to create basic AI service: %v", err)
	}

	enhancedAI, err := ai.NewEnhancedGeminiService(ctx)
	if err != nil {
		log.Printf("Warning: Failed to create enhanced AI service: %v", err)
	}

	// Inicializa treinador com IA
	aiTrainer, err := NewAIEnhancedTrainer(ctx)
	if err != nil {
		log.Printf("Warning: Failed to create AI trainer: %v", err)
	}

	// Inicializa outros componentes
	var patternValidator *PatternValidator
	var enhancedExtractor *EnhancedExtractor

	if aiTrainer != nil {
		patternValidator = NewPatternValidator(aiTrainer.referenceTrainer)
		enhancedExtractor = NewEnhancedExtractor(aiTrainer.referenceTrainer, patternValidator)
	}

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

	return &AIIntegratedCrawler{
		config:            cfg,
		repo:              repo,
		aiService:         aiService,
		enhancedAI:        enhancedAI,
		aiTrainer:         aiTrainer,
		patternValidator:  patternValidator,
		enhancedExtractor: enhancedExtractor,
		logger:            logger.NewLogger("ai_integrated_crawler"),
		collector:         mainCollector,
		detailCollector:   detailCollector,
		visitedURLs:       make(map[string]bool),
		stats: &AIIntegratedStats{
			StartTime:          time.Now(),
			DomainStats:        make(map[string]int),
			AIPerformanceStats: make(map[string]interface{}),
		},
	}, nil
}

// TrainWithAI treina o crawler usando IA avançada
func (aic *AIIntegratedCrawler) TrainWithAI(ctx context.Context, referenceFile string) error {
	aic.logger.WithField("reference_file", referenceFile).Info("Starting AI-enhanced training")

	if aic.aiTrainer == nil {
		return fmt.Errorf("AI trainer not available")
	}

	startTime := time.Now()
	err := aic.aiTrainer.TrainWithAIAnalysis(ctx, referenceFile)
	trainingDuration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("AI training failed: %w", err)
	}

	// Atualiza extrator com novos padrões
	if aic.enhancedExtractor != nil {
		aic.enhancedExtractor.loadDomainSpecificSelectors()
	}

	aic.logger.WithField("duration", trainingDuration.String()).Info("AI-enhanced training completed")
	return nil
}

// StartCrawling inicia o processo de crawling com IA integrada
func (aic *AIIntegratedCrawler) StartCrawling(ctx context.Context) error {
	urls, err := LoadURLsFromFile(aic.config.SitesFile)
	if err != nil {
		return fmt.Errorf("failed to load URLs: %w", err)
	}

	aic.logger.WithField("url_count", len(urls)).Info("Starting AI-integrated crawling")
	aic.stats.StartTime = time.Now()

	// Configura handlers do crawler
	aic.setupCrawlerHandlers(ctx)

	// Inicia crawling das URLs iniciais
	for _, url := range urls {
		if !aic.isVisited(url) {
			aic.markVisited(url)
			if err := aic.collector.Visit(url); err != nil {
				aic.logger.WithField("url", url).Error("Failed to visit URL", err)
				aic.updateStats("error", url)
			}
		}
	}

	// Aguarda conclusão
	aic.collector.Wait()
	aic.detailCollector.Wait()

	// Processa buffer restante da IA
	if aic.aiService != nil {
		if err := aic.aiService.FlushBatch(ctx); err != nil {
			aic.logger.WithError(err).Warn("Failed to flush AI batch")
		}
	}

	aic.logFinalStats()
	return nil
}

// setupCrawlerHandlers configura os handlers do crawler com IA
func (aic *AIIntegratedCrawler) setupCrawlerHandlers(ctx context.Context) {
	// Handler para encontrar links de propriedades
	aic.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		aic.handlePropertyLinkWithAI(ctx, e)
	})

	// Handler principal para páginas de detalhes
	aic.detailCollector.OnHTML("body", func(e *colly.HTMLElement) {
		aic.handlePropertyPageWithAI(ctx, e)
	})

	// Handlers de log
	aic.collector.OnRequest(func(r *colly.Request) {
		aic.logger.WithField("url", r.URL.String()).Debug("Visiting listing page")
	})

	aic.detailCollector.OnRequest(func(r *colly.Request) {
		aic.logger.WithField("url", r.URL.String()).Debug("Visiting property page")
	})

	aic.collector.OnError(func(r *colly.Response, err error) {
		aic.logger.WithField("url", r.Request.URL.String()).Error("Error visiting listing page", err)
		aic.updateStats("error", r.Request.URL.String())
	})

	aic.detailCollector.OnError(func(r *colly.Response, err error) {
		aic.logger.WithField("url", r.Request.URL.String()).Error("Error visiting property page", err)
		aic.updateStats("error", r.Request.URL.String())
	})
}

// handlePropertyLinkWithAI processa links usando IA para classificação
func (aic *AIIntegratedCrawler) handlePropertyLinkWithAI(ctx context.Context, e *colly.HTMLElement) {
	link := e.Attr("href")
	absoluteLink := e.Request.AbsoluteURL(link)

	// Ignora links vazios ou externos
	if absoluteLink == "" || !strings.Contains(absoluteLink, e.Request.URL.Host) {
		return
	}

	// 1. Verifica padrões aprendidos primeiro
	isPropertyLink := false
	confidence := 0.0

	if aic.aiTrainer != nil {
		matchedPattern, patternConfidence := aic.aiTrainer.referenceTrainer.MatchURL(absoluteLink)
		if matchedPattern != nil && patternConfidence > 0.5 {
			isPropertyLink = true
			confidence = patternConfidence
			aic.updateStats("pattern_match", absoluteLink)

			if patternConfidence > 0.8 {
				aic.updateStats("high_confidence_match", absoluteLink)
			}
		}
	}

	// 2. Se não tem certeza pelos padrões, usa IA para classificar (se disponível)
	if !isPropertyLink && aic.enhancedAI != nil {
		// Extrai contexto do link para análise
		linkText := strings.TrimSpace(e.Text)
		linkContext := aic.extractLinkContext(e)

		// Usa IA para classificar se parece ser link de propriedade
		if aic.shouldAnalyzeWithAI(linkText, linkContext, absoluteLink) {
			classification, err := aic.enhancedAI.ClassifyPageContent(ctx, absoluteLink, linkText, linkContext)
			if err == nil {
				aic.updateStats("ai_classification", absoluteLink)

				if classification.IsPropertyPage && classification.Confidence > 0.7 {
					isPropertyLink = true
					confidence = classification.Confidence

					aic.logger.WithFields(map[string]interface{}{
						"url":           absoluteLink,
						"ai_confidence": classification.Confidence,
						"reasoning":     classification.Reasoning,
					}).Debug("AI classified link as property page")
				}
			}
		}
	}

	// 3. Fallback para detecção baseada em URL
	if !isPropertyLink {
		isPropertyLink = aic.isLikelyPropertyURL(absoluteLink)
		confidence = 0.5 // Confiança baixa para fallback
	}

	if isPropertyLink && !aic.isVisited(absoluteLink) {
		aic.markVisited(absoluteLink)
		aic.logger.WithFields(map[string]interface{}{
			"url":        absoluteLink,
			"confidence": confidence,
		}).Info("Found property link")
		aic.detailCollector.Visit(absoluteLink)
	}
}

// handlePropertyPageWithAI processa páginas de propriedade com IA
func (aic *AIIntegratedCrawler) handlePropertyPageWithAI(ctx context.Context, e *colly.HTMLElement) {
	url := e.Request.URL.String()
	aic.updateStats("page_visited", url)

	// 1. Classificação com IA (se disponível)
	var aiClassification *ai.PageClassificationResult
	if aic.enhancedAI != nil {
		title := e.ChildText("title")
		content := e.Text

		classification, err := aic.enhancedAI.ClassifyPageContent(ctx, url, title, content)
		if err == nil {
			aiClassification = classification
			aic.updateStats("ai_classification", url)

			aic.logger.WithFields(map[string]interface{}{
				"url":            url,
				"ai_is_property": classification.IsPropertyPage,
				"ai_confidence":  classification.Confidence,
				"ai_page_type":   classification.PageType,
			}).Debug("AI page classification completed")
		}
	}

	// 2. Classificação com padrões tradicionais
	shouldProcess := false

	if aiClassification != nil {
		// Se IA tem alta confiança, usa sua decisão
		if aiClassification.Confidence > 0.8 {
			shouldProcess = aiClassification.IsPropertyPage
		} else if aiClassification.IsPropertyPage && aiClassification.Confidence > 0.6 {
			shouldProcess = true
		}
	}

	// 3. Se IA não tem certeza, usa classificadores tradicionais
	if !shouldProcess && aiClassification != nil && aiClassification.Confidence < 0.7 {
		// Usa classificadores tradicionais como backup
		shouldProcess = aic.shouldProcessWithTraditionalMethods(e)
	}

	if shouldProcess {
		aic.processPropertyPageWithAI(ctx, e, url, aiClassification)
	} else {
		aic.logger.WithField("url", url).Debug("Page not identified as property, skipping")
	}
}

// processPropertyPageWithAI processa uma página de propriedade com IA
func (aic *AIIntegratedCrawler) processPropertyPageWithAI(ctx context.Context, e *colly.HTMLElement, url string, aiClassification *ai.PageClassificationResult) {
	aic.logger.WithField("url", url).Info("Processing property page with AI")

	// 1. Extrai dados usando extrator melhorado
	var property repository.Property
	if aic.enhancedExtractor != nil {
		property = aic.enhancedExtractor.ExtractPropertyData(e, url)
	} else {
		// Fallback para extração básica
		property = aic.extractBasicPropertyData(e, url)
	}

	aic.updateStats("property_found", url)

	// 2. Valida e melhora dados com IA (se disponível)
	if aic.enhancedAI != nil {
		htmlContent, _ := e.DOM.Html()
		validatedProperty, err := aic.enhancedAI.ValidateExtractedData(ctx, property, htmlContent)
		if err == nil {
			property = validatedProperty
			aic.updateStats("ai_validation", url)
			aic.logger.WithField("url", url).Debug("Property data validated with AI")
		}
	}

	// 3. Processa com IA básica se disponível
	if aic.aiService != nil {
		processedProperty, err := aic.aiService.ProcessPropertyData(ctx, property)
		if err == nil {
			property = processedProperty
			aic.updateStats("ai_enhancement", url)
			aic.logger.WithField("url", url).Debug("Property data enhanced by AI")
		}
	}

	// 4. Valida se os dados são suficientes
	if !aic.isValidProperty(property) {
		aic.logger.WithField("url", url).Warn("Extracted property data is insufficient")
		return
	}

	// 5. Salva propriedade
	if err := aic.repo.Save(ctx, property); err != nil {
		aic.logger.WithField("url", url).Error("Failed to save property", err)
		aic.updateStats("error", url)
	} else {
		aic.updateStats("property_saved", url)
		aic.logger.WithFields(map[string]interface{}{
			"url":           url,
			"address":       property.Endereco,
			"price":         property.Valor,
			"property_type": property.TipoImovel,
		}).Info("Property saved successfully")
	}
}

// Funções auxiliares

// extractLinkContext extrai contexto ao redor de um link
func (aic *AIIntegratedCrawler) extractLinkContext(e *colly.HTMLElement) string {
	// Extrai texto do elemento pai e irmãos para contexto
	parent := e.DOM.Parent()
	context := strings.TrimSpace(parent.Text())

	// Limita tamanho do contexto
	if len(context) > 500 {
		context = context[:500] + "..."
	}

	return context
}

// shouldAnalyzeWithAI decide se deve usar IA para analisar um link
func (aic *AIIntegratedCrawler) shouldAnalyzeWithAI(linkText, context, url string) bool {
	// Usa IA apenas se não conseguir decidir pelos métodos tradicionais
	hasPropertyKeywords := strings.Contains(strings.ToLower(linkText+context), "imovel") ||
		strings.Contains(strings.ToLower(linkText+context), "casa") ||
		strings.Contains(strings.ToLower(linkText+context), "apartamento")

	hasUnclearURL := !strings.Contains(url, "/imovel/") &&
		!strings.Contains(url, "/propriedade/") &&
		!strings.Contains(url, "/casa/")

	return hasPropertyKeywords && hasUnclearURL
}

// shouldProcessWithTraditionalMethods usa métodos tradicionais para decidir processamento
func (aic *AIIntegratedCrawler) shouldProcessWithTraditionalMethods(e *colly.HTMLElement) bool {
	// Implementação básica - pode usar classificadores existentes
	text := strings.ToLower(e.Text)

	// Indicadores básicos de página de propriedade
	hasPrice := strings.Contains(text, "r$")
	hasRooms := strings.Contains(text, "quartos") || strings.Contains(text, "dormitórios")
	hasAddress := strings.Contains(text, "rua") || strings.Contains(text, "avenida")

	return hasPrice && (hasRooms || hasAddress)
}

// extractBasicPropertyData extração básica como fallback
func (aic *AIIntegratedCrawler) extractBasicPropertyData(e *colly.HTMLElement, url string) repository.Property {
	// Implementação básica usando funções existentes
	endereco := getFirstNonEmpty(e, ".endereco", ".address", ".localizacao")
	valor := getFirstNonEmpty(e, ".preco", ".price", ".valor")
	descricao := getFirstNonEmpty(e, ".descricao", ".description", ".detalhes")

	return mapToProperty(endereco, "", descricao, valor, url)
}

// isLikelyPropertyURL verifica se URL parece ser de propriedade
func (aic *AIIntegratedCrawler) isLikelyPropertyURL(url string) bool {
	urlLower := strings.ToLower(url)

	propertyPatterns := []string{
		"/imovel/", "/propriedade/", "/anuncio/", "/detalhes/",
		"/casa/", "/apartamento/", "/ref-", "/codigo-", "/id-",
	}

	for _, pattern := range propertyPatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}

	return false
}

// isValidProperty verifica se propriedade tem dados suficientes
func (aic *AIIntegratedCrawler) isValidProperty(property repository.Property) bool {
	hasBasicInfo := property.Endereco != "" || property.Valor > 0
	hasDetails := property.Descricao != "" || len(property.Caracteristicas) > 0
	validAddress := property.Endereco == "" || (len(property.Endereco) > 10 && len(property.Endereco) < 300)
	validPrice := property.Valor >= 0 && property.Valor < 100000000

	return hasBasicInfo && hasDetails && validAddress && validPrice
}

// Funções de controle de URLs visitadas
func (aic *AIIntegratedCrawler) isVisited(url string) bool {
	aic.visitedMutex.Lock()
	defer aic.visitedMutex.Unlock()
	return aic.visitedURLs[url]
}

func (aic *AIIntegratedCrawler) markVisited(url string) {
	aic.visitedMutex.Lock()
	aic.visitedURLs[url] = true
	aic.visitedMutex.Unlock()
}

// updateStats atualiza estatísticas do crawler
func (aic *AIIntegratedCrawler) updateStats(statType, rawURL string) {
	aic.stats.mutex.Lock()
	defer aic.stats.mutex.Unlock()

	aic.stats.LastUpdate = time.Now()

	switch statType {
	case "page_visited":
		aic.stats.PagesVisited++
	case "property_found":
		aic.stats.PropertiesFound++
	case "property_saved":
		aic.stats.PropertiesSaved++
	case "ai_classification":
		aic.stats.AIClassifications++
	case "ai_validation":
		aic.stats.AIValidations++
	case "ai_enhancement":
		aic.stats.AIEnhancements++
	case "pattern_match":
		aic.stats.PatternMatches++
	case "high_confidence_match":
		aic.stats.HighConfidenceMatches++
	}

	// Atualiza estatísticas por domínio
	if rawURL != "" {
		if parsedURL, err := url.Parse(rawURL); err == nil {
			aic.stats.DomainStats[parsedURL.Host]++
		}
	}
}

// GetStats retorna estatísticas atuais do crawler
func (aic *AIIntegratedCrawler) GetStats() *AIIntegratedStats {
	aic.stats.mutex.RLock()
	defer aic.stats.mutex.RUnlock()

	// Cria cópia das estatísticas
	stats := &AIIntegratedStats{
		PagesVisited:          aic.stats.PagesVisited,
		PropertiesFound:       aic.stats.PropertiesFound,
		PropertiesSaved:       aic.stats.PropertiesSaved,
		AIClassifications:     aic.stats.AIClassifications,
		AIValidations:         aic.stats.AIValidations,
		AIEnhancements:        aic.stats.AIEnhancements,
		PatternMatches:        aic.stats.PatternMatches,
		HighConfidenceMatches: aic.stats.HighConfidenceMatches,
		StartTime:             aic.stats.StartTime,
		LastUpdate:            aic.stats.LastUpdate,
		DomainStats:           make(map[string]int),
		AIPerformanceStats:    make(map[string]interface{}),
	}

	for domain, count := range aic.stats.DomainStats {
		stats.DomainStats[domain] = count
	}

	for key, value := range aic.stats.AIPerformanceStats {
		stats.AIPerformanceStats[key] = value
	}

	return stats
}

// logFinalStats registra estatísticas finais
func (aic *AIIntegratedCrawler) logFinalStats() {
	stats := aic.GetStats()
	duration := time.Since(stats.StartTime)

	aic.logger.WithFields(map[string]interface{}{
		"duration":                duration.String(),
		"pages_visited":           stats.PagesVisited,
		"properties_found":        stats.PropertiesFound,
		"properties_saved":        stats.PropertiesSaved,
		"ai_classifications":      stats.AIClassifications,
		"ai_validations":          stats.AIValidations,
		"ai_enhancements":         stats.AIEnhancements,
		"pattern_matches":         stats.PatternMatches,
		"high_confidence_matches": stats.HighConfidenceMatches,
		"success_rate":            float64(stats.PropertiesSaved) / float64(stats.PropertiesFound) * 100,
		"ai_usage_rate":           float64(stats.AIClassifications) / float64(stats.PagesVisited) * 100,
	}).Info("AI-integrated crawling completed")
}
