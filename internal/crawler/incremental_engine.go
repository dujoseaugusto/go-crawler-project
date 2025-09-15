package crawler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
)

// IncrementalCrawlerEngine implementa crawling incremental com fingerprinting
type IncrementalCrawlerEngine struct {
	repository repository.PropertyRepository
	urlRepo    repository.URLRepository
	aiService  *ai.GeminiService
	extractor  *DataExtractor
	validator  *PropertyValidator
	urlManager *PersistentURLManager
	logger     *logger.Logger
	config     IncrementalConfig
	stats      *IncrementalStats
}

// IncrementalConfig configurações para o crawler incremental
type IncrementalConfig struct {
	EnableAI             bool          `json:"enable_ai"`
	EnableFingerprinting bool          `json:"enable_fingerprinting"`
	MaxAge               time.Duration `json:"max_age"`
	AIThreshold          time.Duration `json:"ai_threshold"`
	CleanupInterval      time.Duration `json:"cleanup_interval"`
	MaxConcurrency       int           `json:"max_concurrency"`
	DelayBetweenRequests time.Duration `json:"delay_between_requests"`
	UserAgent            string        `json:"user_agent"`
}

// IncrementalStats estatísticas do crawling incremental
type IncrementalStats struct {
	StartTime           time.Time     `json:"start_time"`
	EndTime             time.Time     `json:"end_time"`
	TotalURLs           int           `json:"total_urls"`
	ProcessedURLs       int           `json:"processed_urls"`
	SkippedURLs         int           `json:"skipped_urls"`
	NewProperties       int           `json:"new_properties"`
	UpdatedProperties   int           `json:"updated_properties"`
	FailedURLs          int           `json:"failed_urls"`
	AIProcessingCount   int           `json:"ai_processing_count"`
	AISkippedCount      int           `json:"ai_skipped_count"`
	FingerprintHits     int           `json:"fingerprint_hits"`
	FingerprintMisses   int           `json:"fingerprint_misses"`
	ContentChanges      int           `json:"content_changes"`
	ProcessingTimeTotal time.Duration `json:"processing_time_total"`
	AISavingsEstimate   time.Duration `json:"ai_savings_estimate"`
}

// NewIncrementalCrawlerEngine cria um novo engine de crawling incremental
func NewIncrementalCrawlerEngine(
	propertyRepo repository.PropertyRepository,
	urlRepo repository.URLRepository,
	aiService *ai.GeminiService,
	config IncrementalConfig,
) *IncrementalCrawlerEngine {
	// Configurações padrão
	if config.MaxAge == 0 {
		config.MaxAge = 24 * time.Hour
	}
	if config.AIThreshold == 0 {
		config.AIThreshold = 6 * time.Hour
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 7 * 24 * time.Hour
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 5
	}
	if config.DelayBetweenRequests == 0 {
		config.DelayBetweenRequests = 1 * time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "Go-Crawler-Incremental/1.0"
	}

	// Configuração do URL Manager persistente
	urlManagerConfig := PersistentURLConfig{
		MaxAge:               config.MaxAge,
		CleanupInterval:      config.CleanupInterval,
		EnableFingerprinting: config.EnableFingerprinting,
		AIThreshold:          config.AIThreshold,
	}

	return &IncrementalCrawlerEngine{
		repository: propertyRepo,
		urlRepo:    urlRepo,
		aiService:  aiService,
		extractor:  NewDataExtractor(),
		validator:  NewPropertyValidator(),
		urlManager: NewPersistentURLManager(urlRepo, urlManagerConfig),
		logger:     logger.NewLogger("incremental_crawler"),
		config:     config,
		stats:      &IncrementalStats{},
	}
}

// Start inicia o crawling incremental
func (ice *IncrementalCrawlerEngine) Start(ctx context.Context, urls []string) error {
	ice.stats.StartTime = time.Now()
	ice.stats.TotalURLs = len(urls)

	ice.logger.WithFields(map[string]interface{}{
		"total_urls":            len(urls),
		"enable_ai":             ice.config.EnableAI,
		"enable_fingerprinting": ice.config.EnableFingerprinting,
		"max_age":               ice.config.MaxAge,
		"ai_threshold":          ice.config.AIThreshold,
	}).Info("Starting incremental crawling")

	// Carrega URLs visitadas do banco
	if err := ice.urlManager.LoadVisitedURLs(ctx); err != nil {
		ice.logger.Error("Failed to load visited URLs", err)
		return fmt.Errorf("failed to load visited URLs: %v", err)
	}

	// Limpa registros antigos se necessário
	if err := ice.urlManager.CleanupOldRecords(ctx); err != nil {
		ice.logger.Warnf("Failed to cleanup old records: %v", err)
	}

	// Configura o collector
	collector := ice.setupCollector()

	// Processa cada URL
	for _, url := range urls {
		if err := ice.processURL(ctx, collector, url); err != nil {
			ice.logger.WithField("url", url).Error("Failed to process URL", err)
			ice.stats.FailedURLs++
		}
	}

	// Aguarda conclusão
	collector.Wait()

	// Finaliza estatísticas
	ice.stats.EndTime = time.Now()
	ice.stats.ProcessingTimeTotal = ice.stats.EndTime.Sub(ice.stats.StartTime)

	// Log das estatísticas finais
	ice.logFinalStatistics()

	return nil
}

// setupCollector configura o collector do Colly
func (ice *IncrementalCrawlerEngine) setupCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.UserAgent(ice.config.UserAgent),
	)

	// Configurações de performance
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: ice.config.MaxConcurrency,
		Delay:       ice.config.DelayBetweenRequests,
	})

	// Handler para encontrar links de propriedades
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		ice.handlePropertyLinks(e, c)
	})

	// Handler para páginas de propriedades
	c.OnHTML("html", func(e *colly.HTMLElement) {
		ice.handlePropertyPage(context.Background(), e)
	})

	// Handler para erros
	c.OnError(func(r *colly.Response, err error) {
		ice.logger.WithFields(map[string]interface{}{
			"url":         r.Request.URL.String(),
			"status_code": r.StatusCode,
		}).Error("Request failed", err)

		// Marca como falha
		ice.urlManager.MarkURLProcessed(context.Background(), r.Request.URL.String(), "failed", err.Error())
		ice.stats.FailedURLs++
	})

	// Handler para requisições
	c.OnRequest(func(r *colly.Request) {
		ice.logger.WithField("url", r.URL.String()).Debug("Processing URL")
	})

	return c
}

// processURL processa uma URL individual
func (ice *IncrementalCrawlerEngine) processURL(ctx context.Context, collector *colly.Collector, url string) error {
	// Verifica se deve processar a URL
	decision, err := ice.urlManager.ShouldProcessURL(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to check if should process URL: %v", err)
	}

	if !decision.ShouldProcess {
		ice.logger.WithFields(map[string]interface{}{
			"url":    url,
			"reason": decision.Reason,
		}).Debug("Skipping URL")
		ice.stats.SkippedURLs++
		return nil
	}

	// Visita a URL
	if err := collector.Visit(url); err != nil {
		return fmt.Errorf("failed to visit URL: %v", err)
	}

	ice.stats.ProcessedURLs++
	return nil
}

// handlePropertyPage processa uma página de propriedade
func (ice *IncrementalCrawlerEngine) handlePropertyPage(ctx context.Context, e *colly.HTMLElement) {
	startTime := time.Now()
	url := e.Request.URL.String()

	// Gera fingerprint da página se habilitado
	var currentFingerprint string
	if ice.config.EnableFingerprinting {
		currentFingerprint = ice.urlManager.GeneratePageFingerprint(e)
	}

	// Verifica se é uma página de catálogo
	if ice.isCatalogPage(e) {
		ice.handleCatalogPage(ctx, e)
		return
	}

	// Extrai dados da propriedade
	property := ice.extractor.ExtractProperty(e, url)
	if property == nil {
		ice.logger.WithField("url", url).Debug("No property data found")
		ice.urlManager.MarkURLProcessed(ctx, url, "skipped", "no property data")
		return
	}

	// Valida os dados
	validation := ice.validator.ValidateProperty(property)
	if !validation.IsValid {
		ice.logger.WithFields(map[string]interface{}{
			"url":    url,
			"errors": validation.Errors,
		}).Debug("Property validation failed")
		ice.urlManager.MarkURLProcessed(ctx, url, "failed", "validation failed")
		return
	}

	// Melhora os dados
	property = ice.validator.EnhanceProperty(property)

	// Determina se deve usar IA
	shouldUseAI := false
	aiReason := "ai_disabled"
	if ice.config.EnableAI && ice.aiService != nil {
		shouldUseAI, aiReason = ice.urlManager.ShouldUseAI(ctx, url)
	}

	// Processa com IA se necessário
	aiProcessed := false
	if shouldUseAI {
		if processedProperty, err := ice.aiService.ProcessPropertyData(ctx, *property); err == nil {
			property = &processedProperty
			aiProcessed = true
			ice.stats.AIProcessingCount++
			ice.logger.WithField("url", url).Debug("Property processed with AI")
		} else {
			ice.logger.WithField("url", url).WithError(err).Warn("AI processing failed, using original data")
		}
	} else {
		ice.stats.AISkippedCount++
		ice.logger.WithFields(map[string]interface{}{
			"url":    url,
			"reason": aiReason,
		}).Debug("Skipped AI processing")
	}

	// Salva no repositório
	if err := ice.repository.Save(ctx, *property); err != nil {
		ice.logger.WithField("url", url).Error("Failed to save property", err)
		ice.urlManager.MarkURLProcessed(ctx, url, "failed", err.Error())
		return
	}

	// Atualiza estatísticas
	ice.stats.NewProperties++

	// Salva fingerprint se habilitado
	if ice.config.EnableFingerprinting {
		propertyCount := 1 // Esta página tem 1 propriedade
		if err := ice.urlManager.SavePageFingerprint(ctx, url, currentFingerprint, propertyCount, aiProcessed); err != nil {
			ice.logger.WithField("url", url).WithError(err).Warn("Failed to save fingerprint")
		}
	}

	// Marca como processada com sucesso
	ice.urlManager.MarkURLProcessed(ctx, url, "success", "")

	// Atualiza tempo de processamento
	processingTime := time.Since(startTime)
	ice.stats.ProcessingTimeTotal += processingTime

	ice.logger.WithFields(map[string]interface{}{
		"url":             url,
		"endereco":        property.Endereco,
		"valor":           property.Valor,
		"tipo":            property.TipoImovel,
		"ai_processed":    aiProcessed,
		"ai_reason":       aiReason,
		"processing_time": processingTime,
	}).Info("Property processed successfully")
}

// isCatalogPage verifica se é uma página de catálogo
func (ice *IncrementalCrawlerEngine) isCatalogPage(e *colly.HTMLElement) bool {
	url := e.Request.URL.String()
	text := strings.ToLower(e.Text)

	// Indicadores de página de catálogo
	catalogIndicators := []string{
		"imoveis/?", "busca", "listagem", "catalogo", "pesquisa",
		"resultados", "filtros", "ordenar", "pagina=", "page=",
	}

	indicatorCount := 0
	for _, indicator := range catalogIndicators {
		if strings.Contains(url, indicator) {
			indicatorCount++
		}
	}

	// Verifica se tem múltiplos preços (indicativo de listagem)
	priceCount := strings.Count(text, "r$")
	roomCount := strings.Count(text, "quartos")

	return indicatorCount > 0 && (priceCount > 5 || roomCount > 3)
}

// handleCatalogPage processa páginas de catálogo
func (ice *IncrementalCrawlerEngine) handleCatalogPage(ctx context.Context, e *colly.HTMLElement) {
	url := e.Request.URL.String()
	ice.logger.WithField("url", url).Info("Processing catalog page")

	// Gera fingerprint do catálogo
	if ice.config.EnableFingerprinting {
		currentFingerprint := ice.urlManager.GeneratePageFingerprint(e)

		// Conta quantas propriedades tem no catálogo
		propertyCount := ice.countPropertiesInCatalog(e)

		// Salva fingerprint do catálogo
		if err := ice.urlManager.SavePageFingerprint(ctx, url, currentFingerprint, propertyCount, false); err != nil {
			ice.logger.WithField("url", url).WithError(err).Warn("Failed to save catalog fingerprint")
		}
	}

	// Marca como processada
	ice.urlManager.MarkURLProcessed(ctx, url, "success", "catalog page")
}

// countPropertiesInCatalog conta propriedades em uma página de catálogo
func (ice *IncrementalCrawlerEngine) countPropertiesInCatalog(e *colly.HTMLElement) int {
	// Conta elementos que parecem ser propriedades individuais
	propertySelectors := []string{
		".property", ".imovel", ".listing", ".item",
		"[class*=property]", "[class*=imovel]", "[class*=listing]",
	}

	maxCount := 0
	for _, selector := range propertySelectors {
		count := 0
		e.ForEach(selector, func(_ int, el *colly.HTMLElement) {
			// Verifica se tem indicadores de propriedade
			text := strings.ToLower(el.Text)
			if strings.Contains(text, "r$") || strings.Contains(text, "quartos") {
				count++
			}
		})
		if count > maxCount {
			maxCount = count
		}
	}

	return maxCount
}

// GetStatistics retorna estatísticas do crawling
func (ice *IncrementalCrawlerEngine) GetStatistics() *IncrementalStats {
	// Calcula economia estimada de IA
	if ice.stats.AISkippedCount > 0 {
		// Estima que cada processamento de IA economizado salva ~2 segundos
		ice.stats.AISavingsEstimate = time.Duration(ice.stats.AISkippedCount) * 2 * time.Second
	}

	return ice.stats
}

// logFinalStatistics registra estatísticas finais
func (ice *IncrementalCrawlerEngine) logFinalStatistics() {
	stats := ice.GetStatistics()

	// Calcula percentuais
	processedPercent := float64(stats.ProcessedURLs) / float64(stats.TotalURLs) * 100
	skippedPercent := float64(stats.SkippedURLs) / float64(stats.TotalURLs) * 100
	aiSavingsPercent := float64(stats.AISkippedCount) / float64(stats.ProcessedURLs) * 100

	ice.logger.WithFields(map[string]interface{}{
		"total_urls":          stats.TotalURLs,
		"processed_urls":      stats.ProcessedURLs,
		"skipped_urls":        stats.SkippedURLs,
		"processed_percent":   fmt.Sprintf("%.1f%%", processedPercent),
		"skipped_percent":     fmt.Sprintf("%.1f%%", skippedPercent),
		"new_properties":      stats.NewProperties,
		"failed_urls":         stats.FailedURLs,
		"ai_processing_count": stats.AIProcessingCount,
		"ai_skipped_count":    stats.AISkippedCount,
		"ai_savings_percent":  fmt.Sprintf("%.1f%%", aiSavingsPercent),
		"ai_savings_estimate": stats.AISavingsEstimate,
		"processing_time":     stats.ProcessingTimeTotal,
		"fingerprint_hits":    stats.FingerprintHits,
		"fingerprint_misses":  stats.FingerprintMisses,
		"content_changes":     stats.ContentChanges,
	}).Info("Incremental crawling completed")

	// Log de economia
	if stats.SkippedURLs > 0 {
		ice.logger.WithFields(map[string]interface{}{
			"skipped_urls":    stats.SkippedURLs,
			"total_urls":      stats.TotalURLs,
			"efficiency_gain": fmt.Sprintf("%.1f%%", skippedPercent),
			"ai_savings":      stats.AISavingsEstimate,
		}).Info("Incremental crawling efficiency gains")
	}
}

// handlePropertyLinks processa links encontrados na página
func (ice *IncrementalCrawlerEngine) handlePropertyLinks(e *colly.HTMLElement, c *colly.Collector) {
	// FUNCIONALIDADE ANTI-DUPLICAÇÃO: Verifica se deve parar de seguir links internos
	// Se detectou que é uma página de imóvel, não segue mais links para evitar duplicatas
	if ice.urlManager.ShouldSkipInternalLinks(e) {
		ice.logger.WithField("url", e.Request.URL.String()).Info("Skipping internal links - property page detected")
		return
	}

	link := e.Attr("href")
	absoluteLink := e.Request.AbsoluteURL(link)

	// Normaliza a URL antes de verificar
	absoluteLink = ice.urlManager.NormalizeURL(absoluteLink)

	// Verifica se é um link válido e não visitado
	if ice.urlManager.ShouldSkipURL(absoluteLink) || ice.urlManager.IsVisited(absoluteLink) {
		return
	}

	// Verifica se parece ser um link de propriedade
	if ice.urlManager.IsValidPropertyLink(absoluteLink, e.Request.URL.Host) {
		ice.urlManager.MarkVisited(absoluteLink)

		// Limita o número de URLs para evitar memory leak
		if ice.urlManager.GetVisitedCount() > 10000 {
			ice.urlManager.CleanupOldURLs(5000)
		}

		// Visita o link encontrado
		c.Visit(absoluteLink)
	}
}
