package crawler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gocolly/colly"
)

// PatternValidationResult representa o resultado de uma validação
type PatternValidationResult struct {
	URL            string                 `json:"url"`
	IsPropertyPage bool                   `json:"is_property_page"`
	Confidence     float64                `json:"confidence"`
	ExtractedData  map[string]interface{} `json:"extracted_data"`
	Selectors      map[string]string      `json:"selectors_used"`
	Errors         []string               `json:"errors"`
	ValidationTime time.Time              `json:"validation_time"`
}

// PatternValidator valida padrões usando páginas de referência conhecidas
type PatternValidator struct {
	referenceTrainer *ReferencePatternTrainer
	pageClassifier   *AdvancedPageClassifier
	contentLearner   *ContentBasedPatternLearner
	logger           *logger.Logger
	collector        *colly.Collector
	mutex            sync.RWMutex
	validationCache  map[string]*PatternValidationResult
}

// NewPatternValidator cria um novo validador de padrões
func NewPatternValidator(referenceTrainer *ReferencePatternTrainer) *PatternValidator {
	c := colly.NewCollector(
		colly.MaxDepth(1),
		colly.Async(false),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	return &PatternValidator{
		referenceTrainer: referenceTrainer,
		pageClassifier:   NewAdvancedPageClassifier(),
		contentLearner:   NewContentBasedPatternLearner(),
		logger:           logger.NewLogger("pattern_validator"),
		collector:        c,
		validationCache:  make(map[string]*PatternValidationResult),
	}
}

// ValidateURL valida se uma URL é realmente uma página de anúncio de imóvel
func (pv *PatternValidator) ValidateURL(ctx context.Context, rawURL string) (*PatternValidationResult, error) {
	pv.mutex.RLock()
	if cached, exists := pv.validationCache[rawURL]; exists {
		pv.mutex.RUnlock()
		return cached, nil
	}
	pv.mutex.RUnlock()

	pv.logger.WithField("url", rawURL).Info("Validating URL")

	result := &PatternValidationResult{
		URL:            rawURL,
		ExtractedData:  make(map[string]interface{}),
		Selectors:      make(map[string]string),
		Errors:         []string{},
		ValidationTime: time.Now(),
	}

	// 1. Verifica se corresponde a padrões de referência conhecidos
	matchedPattern, patternScore := pv.referenceTrainer.MatchURL(rawURL)
	if matchedPattern != nil && patternScore > 0.5 {
		pv.logger.WithFields(map[string]interface{}{
			"url":           rawURL,
			"pattern_id":    matchedPattern.ID,
			"pattern_score": patternScore,
		}).Info("URL matches known reference pattern")

		// Usa padrão conhecido para validação
		return pv.validateWithKnownPattern(ctx, rawURL, matchedPattern, result)
	}

	// 2. Se não corresponde a padrão conhecido, usa classificação geral
	return pv.validateWithGeneralClassification(ctx, rawURL, result)
}

// validateWithKnownPattern valida usando um padrão de referência conhecido
func (pv *PatternValidator) validateWithKnownPattern(ctx context.Context, rawURL string, pattern *ReferencePattern, result *PatternValidationResult) (*PatternValidationResult, error) {
	var pageElement *colly.HTMLElement

	// Configura callback para capturar elemento da página
	pv.collector.OnHTML("html", func(e *colly.HTMLElement) {
		pageElement = e
	})

	// Visita a página
	if err := pv.collector.Visit(rawURL); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to visit URL: %v", err))
		return result, err
	}

	pv.collector.Wait()

	if pageElement == nil {
		result.Errors = append(result.Errors, "No page content retrieved")
		return result, fmt.Errorf("no page content")
	}

	// Usa seletores do padrão conhecido para extrair dados
	extractedCount := 0
	for dataType, selectors := range pattern.Selectors {
		for _, selector := range selectors {
			content := strings.TrimSpace(pageElement.ChildText(selector))
			if content != "" && pv.isValidExtractedContent(content, dataType) {
				result.ExtractedData[dataType] = content
				result.Selectors[dataType] = selector
				extractedCount++
				break // Usa o primeiro seletor que funciona
			}
		}
	}

	// Calcula confiança baseada na quantidade de dados extraídos
	expectedDataTypes := len(pattern.Selectors)
	if expectedDataTypes > 0 {
		result.Confidence = (float64(extractedCount) / float64(expectedDataTypes)) * pattern.Confidence
	} else {
		result.Confidence = pattern.Confidence * 0.5 // Reduz confiança se não conseguiu extrair dados
	}

	// Considera como página de propriedade se extraiu dados suficientes
	result.IsPropertyPage = extractedCount >= 2 && result.Confidence > 0.4

	// Validação adicional usando classificadores
	pageType := pv.pageClassifier.ClassifyPageStrict(pageElement)
	if pageType == PageTypeProperty {
		result.Confidence = (result.Confidence + 0.8) / 2 // Média com alta confiança do classificador
		result.IsPropertyPage = true
	} else if pageType == PageTypeCatalog {
		result.Confidence *= 0.3 // Reduz muito a confiança se classificador diz que é catálogo
		result.IsPropertyPage = false
		result.Errors = append(result.Errors, "Page classified as catalog by strict classifier")
	}

	pv.cacheResult(rawURL, result)

	pv.logger.WithFields(map[string]interface{}{
		"url":              rawURL,
		"is_property_page": result.IsPropertyPage,
		"confidence":       result.Confidence,
		"extracted_count":  extractedCount,
	}).Info("Validation completed with known pattern")

	return result, nil
}

// validateWithGeneralClassification valida usando classificação geral
func (pv *PatternValidator) validateWithGeneralClassification(ctx context.Context, rawURL string, result *PatternValidationResult) (*PatternValidationResult, error) {
	var pageElement *colly.HTMLElement

	pv.collector.OnHTML("html", func(e *colly.HTMLElement) {
		pageElement = e
	})

	if err := pv.collector.Visit(rawURL); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to visit URL: %v", err))
		return result, err
	}

	pv.collector.Wait()

	if pageElement == nil {
		result.Errors = append(result.Errors, "No page content retrieved")
		return result, fmt.Errorf("no page content")
	}

	// Usa classificador rigoroso
	pageType := pv.pageClassifier.ClassifyPageStrict(pageElement)

	// Usa classificador baseado em conteúdo
	contentType, contentConfidence := pv.contentLearner.ClassifyPageContent(pageElement)

	// Combina resultados
	if pageType == PageTypeProperty && contentType == "property" {
		result.IsPropertyPage = true
		result.Confidence = (0.8 + contentConfidence) / 2
	} else if pageType == PageTypeProperty || contentType == "property" {
		result.IsPropertyPage = true
		result.Confidence = 0.6
	} else if pageType == PageTypeCatalog || contentType == "catalog" {
		result.IsPropertyPage = false
		result.Confidence = 0.8
		result.Errors = append(result.Errors, "Page classified as catalog")
	} else {
		result.IsPropertyPage = false
		result.Confidence = 0.3
		result.Errors = append(result.Errors, "Page type unknown or not property-related")
	}

	// Tenta extrair dados básicos usando seletores genéricos
	pv.extractBasicData(pageElement, result)

	pv.cacheResult(rawURL, result)

	pv.logger.WithFields(map[string]interface{}{
		"url":                rawURL,
		"page_type":          pageType,
		"content_type":       contentType,
		"content_confidence": contentConfidence,
		"is_property_page":   result.IsPropertyPage,
		"final_confidence":   result.Confidence,
	}).Info("Validation completed with general classification")

	return result, nil
}

// extractBasicData extrai dados básicos usando seletores genéricos
func (pv *PatternValidator) extractBasicData(e *colly.HTMLElement, result *PatternValidationResult) {
	// Seletores genéricos para diferentes tipos de dados
	genericSelectors := map[string][]string{
		"price": {
			".preco", ".price", ".valor", "[class*='preco']", "[class*='price']",
		},
		"address": {
			".endereco", ".address", ".localizacao", "[class*='endereco']", "[class*='address']",
		},
		"description": {
			".descricao", ".description", ".detalhes", "[class*='descricao']",
		},
	}

	for dataType, selectors := range genericSelectors {
		for _, selector := range selectors {
			content := strings.TrimSpace(e.ChildText(selector))
			if content != "" && pv.isValidExtractedContent(content, dataType) {
				result.ExtractedData[dataType] = content
				result.Selectors[dataType] = selector
				break
			}
		}
	}

	// Se conseguiu extrair dados, aumenta a confiança
	if len(result.ExtractedData) > 0 {
		result.Confidence = (result.Confidence + 0.2) / 1.2 // Normaliza
	}
}

// isValidExtractedContent verifica se o conteúdo extraído é válido
func (pv *PatternValidator) isValidExtractedContent(content, dataType string) bool {
	content = strings.ToLower(content)

	switch dataType {
	case "price":
		return strings.Contains(content, "r$") && len(content) < 50
	case "address":
		return len(content) > 10 && len(content) < 200 &&
			!strings.Contains(content, "todos os direitos") &&
			!strings.Contains(content, "copyright")
	case "description":
		return len(content) > 20 && len(content) < 2000 &&
			!strings.Contains(content, "javascript") &&
			!strings.Contains(content, "cookie")
	default:
		return len(content) > 5 && len(content) < 500
	}
}

// cacheResult armazena resultado no cache
func (pv *PatternValidator) cacheResult(url string, result *PatternValidationResult) {
	pv.mutex.Lock()
	defer pv.mutex.Unlock()
	pv.validationCache[url] = result
}

// ValidateBatch valida múltiplas URLs em lote
func (pv *PatternValidator) ValidateBatch(ctx context.Context, urls []string) ([]*PatternValidationResult, error) {
	pv.logger.WithField("url_count", len(urls)).Info("Starting batch validation")

	var results []*PatternValidationResult
	var errors []string

	for i, url := range urls {
		if url == "" {
			continue
		}

		pv.logger.WithFields(map[string]interface{}{
			"url":      url,
			"progress": fmt.Sprintf("%d/%d", i+1, len(urls)),
		}).Info("Validating URL in batch")

		result, err := pv.ValidateURL(ctx, url)
		if err != nil {
			errors = append(errors, fmt.Sprintf("URL %s: %v", url, err))
			continue
		}

		results = append(results, result)

		// Pausa entre requisições para evitar sobrecarga
		time.Sleep(1 * time.Second)
	}

	pv.logger.WithFields(map[string]interface{}{
		"total_urls":     len(urls),
		"successful":     len(results),
		"errors":         len(errors),
		"property_pages": pv.countPropertyPages(results),
	}).Info("Batch validation completed")

	if len(errors) > 0 {
		return results, fmt.Errorf("validation errors: %v", errors)
	}

	return results, nil
}

// countPropertyPages conta quantas páginas foram identificadas como de propriedade
func (pv *PatternValidator) countPropertyPages(results []*PatternValidationResult) int {
	count := 0
	for _, result := range results {
		if result.IsPropertyPage {
			count++
		}
	}
	return count
}

// GetValidationStats retorna estatísticas de validação
func (pv *PatternValidator) GetValidationStats() map[string]interface{} {
	pv.mutex.RLock()
	defer pv.mutex.RUnlock()

	stats := make(map[string]interface{})

	totalValidations := len(pv.validationCache)
	propertyPages := 0
	catalogPages := 0
	unknownPages := 0
	totalConfidence := 0.0

	for _, result := range pv.validationCache {
		totalConfidence += result.Confidence

		if result.IsPropertyPage {
			propertyPages++
		} else {
			// Verifica se foi identificado como catálogo ou desconhecido
			isCatalog := false
			for _, err := range result.Errors {
				if strings.Contains(err, "catalog") {
					isCatalog = true
					break
				}
			}

			if isCatalog {
				catalogPages++
			} else {
				unknownPages++
			}
		}
	}

	stats["total_validations"] = totalValidations
	stats["property_pages"] = propertyPages
	stats["catalog_pages"] = catalogPages
	stats["unknown_pages"] = unknownPages

	if totalValidations > 0 {
		stats["avg_confidence"] = totalConfidence / float64(totalValidations)
		stats["property_rate"] = float64(propertyPages) / float64(totalValidations)
	}

	return stats
}

// ClearCache limpa o cache de validação
func (pv *PatternValidator) ClearCache() {
	pv.mutex.Lock()
	defer pv.mutex.Unlock()
	pv.validationCache = make(map[string]*PatternValidationResult)
	pv.logger.Info("Validation cache cleared")
}

// GetCachedResult retorna resultado em cache se existir
func (pv *PatternValidator) GetCachedResult(url string) (*PatternValidationResult, bool) {
	pv.mutex.RLock()
	defer pv.mutex.RUnlock()
	result, exists := pv.validationCache[url]
	return result, exists
}

// UpdateReferenceTrainer atualiza o treinador de referência
func (pv *PatternValidator) UpdateReferenceTrainer(trainer *ReferencePatternTrainer) {
	pv.referenceTrainer = trainer
	pv.logger.Info("Reference trainer updated")
}

// ValidateAndImprovePatterns valida URLs e melhora padrões baseado nos resultados
func (pv *PatternValidator) ValidateAndImprovePatterns(ctx context.Context, referenceURLs []string) error {
	pv.logger.WithField("reference_count", len(referenceURLs)).Info("Starting pattern improvement process")

	// Valida todas as URLs de referência
	results, err := pv.ValidateBatch(ctx, referenceURLs)
	if err != nil {
		pv.logger.WithError(err).Warn("Some validations failed, continuing with successful ones")
	}

	// Analisa resultados para identificar padrões que precisam de melhoria
	successfulResults := make([]*PatternValidationResult, 0)
	failedResults := make([]*PatternValidationResult, 0)

	for _, result := range results {
		if result.IsPropertyPage && result.Confidence > 0.6 {
			successfulResults = append(successfulResults, result)
		} else {
			failedResults = append(failedResults, result)
		}
	}

	pv.logger.WithFields(map[string]interface{}{
		"successful": len(successfulResults),
		"failed":     len(failedResults),
	}).Info("Validation results analyzed")

	// Se há muitas falhas, sugere retreinamento
	if len(failedResults) > len(successfulResults)/2 {
		pv.logger.Warn("High failure rate detected, consider retraining reference patterns")

		// Retreina com URLs que falharam
		var failedURLs []string
		for _, result := range failedResults {
			failedURLs = append(failedURLs, result.URL)
		}

		if err := pv.referenceTrainer.TrainFromReferenceFile(ctx, "List-site.ini"); err != nil {
			return fmt.Errorf("failed to retrain patterns: %w", err)
		}
	}

	return nil
}
