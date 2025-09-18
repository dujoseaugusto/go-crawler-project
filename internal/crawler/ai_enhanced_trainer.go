package crawler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

// AIEnhancedTrainer combina o treinamento de padrões com análise de IA
type AIEnhancedTrainer struct {
	referenceTrainer *ReferencePatternTrainer
	enhancedAI       *ai.EnhancedGeminiService
	logger           *logger.Logger
	collector        *colly.Collector
	aiAnalysisCache  map[string]*ai.PatternAnalysisResult
	cacheMutex       sync.RWMutex
}

// NewAIEnhancedTrainer cria um novo treinador com IA
func NewAIEnhancedTrainer(ctx context.Context) (*AIEnhancedTrainer, error) {
	// Inicializa componentes base
	referenceTrainer := NewReferencePatternTrainer()

	enhancedAI, err := ai.NewEnhancedGeminiService(ctx)
	if err != nil {
		log.Printf("Warning: Failed to create enhanced AI service: %v", err)
		// Continua sem IA se houver erro
		return &AIEnhancedTrainer{
			referenceTrainer: referenceTrainer,
			logger:           logger.NewLogger("ai_enhanced_trainer"),
			aiAnalysisCache:  make(map[string]*ai.PatternAnalysisResult),
		}, nil
	}

	// Configura collector
	c := colly.NewCollector(
		colly.MaxDepth(1),
		colly.Async(false),
	)

	extensions.RandomUserAgent(c)
	extensions.Referer(c)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       3 * time.Second, // Mais conservador para análise detalhada
	})

	return &AIEnhancedTrainer{
		referenceTrainer: referenceTrainer,
		enhancedAI:       enhancedAI,
		logger:           logger.NewLogger("ai_enhanced_trainer"),
		collector:        c,
		aiAnalysisCache:  make(map[string]*ai.PatternAnalysisResult),
	}, nil
}

// TrainWithAIAnalysis treina padrões usando análise de IA avançada
func (aet *AIEnhancedTrainer) TrainWithAIAnalysis(ctx context.Context, referenceFile string) error {
	aet.logger.WithField("reference_file", referenceFile).Info("Starting AI-enhanced training")

	// 1. Primeiro faz o treinamento básico
	if err := aet.referenceTrainer.TrainFromReferenceFile(ctx, referenceFile); err != nil {
		return fmt.Errorf("failed basic training: %w", err)
	}

	// 2. Se IA disponível, faz análise avançada
	if aet.enhancedAI != nil {
		if err := aet.enhanceWithAIAnalysis(ctx, referenceFile); err != nil {
			aet.logger.WithError(err).Warn("AI analysis failed, continuing with basic patterns")
		}
	}

	aet.logger.Info("AI-enhanced training completed")
	return nil
}

// enhanceWithAIAnalysis melhora padrões usando análise de IA
func (aet *AIEnhancedTrainer) enhanceWithAIAnalysis(ctx context.Context, referenceFile string) error {
	aet.logger.Info("Starting AI analysis of reference pages")

	// Carrega URLs de referência
	urls, err := aet.referenceTrainer.loadURLsFromFile(referenceFile)
	if err != nil {
		return fmt.Errorf("failed to load URLs: %w", err)
	}

	// Analisa cada URL com IA
	for i, rawURL := range urls {
		if rawURL == "" {
			continue
		}

		aet.logger.WithFields(map[string]interface{}{
			"url":      rawURL,
			"progress": fmt.Sprintf("%d/%d", i+1, len(urls)),
		}).Info("Analyzing URL with AI")

		if err := aet.analyzeURLWithAI(ctx, rawURL); err != nil {
			aet.logger.WithError(err).WithField("url", rawURL).Warn("AI analysis failed for URL")
			continue
		}

		// Pausa entre análises para não sobrecarregar a API
		time.Sleep(2 * time.Second)
	}

	// Consolida análises da IA com padrões existentes
	aet.consolidateAIAnalysis()

	return nil
}

// analyzeURLWithAI analisa uma URL específica com IA
func (aet *AIEnhancedTrainer) analyzeURLWithAI(ctx context.Context, rawURL string) error {
	var htmlContent string
	var pageTitle string

	// Configura callback para capturar conteúdo
	aet.collector.OnHTML("html", func(e *colly.HTMLElement) {
		htmlContent, _ = e.DOM.Html()
		pageTitle = e.ChildText("title")
	})

	// Visita a página
	if err := aet.collector.Visit(rawURL); err != nil {
		return fmt.Errorf("failed to visit URL: %w", err)
	}

	aet.collector.Wait()

	if htmlContent == "" {
		return fmt.Errorf("no HTML content retrieved")
	}

	// 1. Classifica a página com IA
	classification, err := aet.enhancedAI.ClassifyPageContent(ctx, rawURL, pageTitle, htmlContent)
	if err != nil {
		aet.logger.WithError(err).Warn("Failed to classify page with AI")
	} else {
		aet.logger.WithFields(map[string]interface{}{
			"url":         rawURL,
			"is_property": classification.IsPropertyPage,
			"confidence":  classification.Confidence,
			"page_type":   classification.PageType,
		}).Info("AI page classification completed")

		// Se IA diz que não é página de propriedade, registra aviso
		if !classification.IsPropertyPage {
			aet.logger.WithFields(map[string]interface{}{
				"url":       rawURL,
				"reasoning": classification.Reasoning,
			}).Warn("AI classified reference URL as non-property page")
		}
	}

	// 2. Analisa padrões da página com IA
	patternAnalysis, err := aet.enhancedAI.AnalyzePagePatterns(ctx, rawURL, htmlContent)
	if err != nil {
		return fmt.Errorf("failed to analyze patterns with AI: %w", err)
	}

	// Armazena análise no cache
	aet.cacheMutex.Lock()
	aet.aiAnalysisCache[rawURL] = patternAnalysis
	aet.cacheMutex.Unlock()

	aet.logger.WithFields(map[string]interface{}{
		"url":             rawURL,
		"selectors_found": len(patternAnalysis.Selectors),
		"url_pattern":     patternAnalysis.URLPattern,
	}).Info("AI pattern analysis completed")

	return nil
}

// consolidateAIAnalysis consolida análises da IA com padrões existentes
func (aet *AIEnhancedTrainer) consolidateAIAnalysis() {
	aet.cacheMutex.RLock()
	defer aet.cacheMutex.RUnlock()

	aet.logger.WithField("analyses_count", len(aet.aiAnalysisCache)).Info("Consolidating AI analyses")

	// Agrupa análises por domínio
	domainAnalyses := make(map[string][]*ai.PatternAnalysisResult)

	for url, analysis := range aet.aiAnalysisCache {
		domain := extractDomainFromTrainerURL(url)
		domainAnalyses[domain] = append(domainAnalyses[domain], analysis)
	}

	// Para cada domínio, melhora os padrões existentes
	for domain, analyses := range domainAnalyses {
		aet.enhanceDomainPatterns(domain, analyses)
	}
}

// enhanceDomainPatterns melhora padrões de um domínio com análises da IA
func (aet *AIEnhancedTrainer) enhanceDomainPatterns(domain string, analyses []*ai.PatternAnalysisResult) {
	aet.logger.WithFields(map[string]interface{}{
		"domain":   domain,
		"analyses": len(analyses),
	}).Info("Enhancing domain patterns with AI")

	// Obtém padrões existentes do domínio
	existingPatterns := aet.referenceTrainer.GetLearnedPatterns()

	var domainPattern *ReferencePattern
	for _, pattern := range existingPatterns {
		if strings.Contains(pattern.Domain, domain) || strings.Contains(domain, pattern.Domain) {
			domainPattern = pattern
			break
		}
	}

	if domainPattern == nil {
		aet.logger.WithField("domain", domain).Warn("No existing pattern found for domain")
		return
	}

	// Consolida sugestões de seletores da IA
	aiSelectors := make(map[string][]string)

	for _, analysis := range analyses {
		for _, selector := range analysis.Selectors {
			if selector.Confidence > 0.7 { // Apenas seletores com alta confiança
				dataType := selector.DataType
				if _, exists := aiSelectors[dataType]; !exists {
					aiSelectors[dataType] = []string{}
				}
				aiSelectors[dataType] = append(aiSelectors[dataType], selector.Selector)
			}
		}
	}

	// Mescla seletores da IA com padrões existentes
	for dataType, selectors := range aiSelectors {
		if existingSelectors, exists := domainPattern.Selectors[dataType]; exists {
			// Adiciona seletores da IA no início (alta prioridade)
			enhanced := append(selectors, existingSelectors...)
			domainPattern.Selectors[dataType] = removeDuplicateSelectors(enhanced)
		} else {
			// Cria nova entrada com seletores da IA
			domainPattern.Selectors[dataType] = selectors
		}
	}

	// Atualiza confiança baseada na análise da IA
	if len(analyses) > 0 {
		domainPattern.Confidence = (domainPattern.Confidence + 0.9) / 2 // Aumenta confiança
		domainPattern.LastTested = time.Now()
	}

	aet.logger.WithFields(map[string]interface{}{
		"domain":         domain,
		"enhanced_types": len(aiSelectors),
		"new_confidence": domainPattern.Confidence,
	}).Info("Domain patterns enhanced with AI")
}

// ValidateWithAI valida uma URL usando classificação de IA
func (aet *AIEnhancedTrainer) ValidateWithAI(ctx context.Context, url, title, content string) (*ai.PageClassificationResult, error) {
	if aet.enhancedAI == nil {
		return nil, fmt.Errorf("AI service not available")
	}

	return aet.enhancedAI.ClassifyPageContent(ctx, url, title, content)
}

// SuggestSelectorsForDomain usa IA para sugerir seletores para um domínio
func (aet *AIEnhancedTrainer) SuggestSelectorsForDomain(ctx context.Context, domain, sampleHTML string) ([]ai.SelectorSuggestion, error) {
	if aet.enhancedAI == nil {
		return nil, fmt.Errorf("AI service not available")
	}

	return aet.enhancedAI.SuggestSelectorsForSite(ctx, domain, sampleHTML)
}

// GetEnhancedPatterns retorna padrões melhorados com IA
func (aet *AIEnhancedTrainer) GetEnhancedPatterns() map[string]*ReferencePattern {
	return aet.referenceTrainer.GetLearnedPatterns()
}

// GetAIAnalyses retorna análises da IA
func (aet *AIEnhancedTrainer) GetAIAnalyses() map[string]*ai.PatternAnalysisResult {
	aet.cacheMutex.RLock()
	defer aet.cacheMutex.RUnlock()

	result := make(map[string]*ai.PatternAnalysisResult)
	for url, analysis := range aet.aiAnalysisCache {
		result[url] = analysis
	}
	return result
}

// IsAIAvailable verifica se o serviço de IA está disponível
func (aet *AIEnhancedTrainer) IsAIAvailable() bool {
	return aet.enhancedAI != nil
}

// Funções auxiliares

// extractDomainFromTrainerURL extrai domínio de uma URL
func extractDomainFromTrainerURL(rawURL string) string {
	// Implementação simples - pode ser melhorada
	parts := strings.Split(rawURL, "/")
	if len(parts) >= 3 {
		domain := parts[2]
		// Remove www.
		domain = strings.TrimPrefix(domain, "www.")
		return domain
	}
	return rawURL
}

// removeDuplicateSelectors remove seletores duplicados mantendo ordem
func removeDuplicateSelectors(selectors []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, selector := range selectors {
		if !seen[selector] {
			seen[selector] = true
			result = append(result, selector)
		}
	}

	return result
}

// ExportEnhancedPatterns exporta padrões melhorados para JSON
func (aet *AIEnhancedTrainer) ExportEnhancedPatterns() ([]byte, error) {
	return aet.referenceTrainer.ExportPatterns()
}

// ImportEnhancedPatterns importa padrões melhorados de JSON
func (aet *AIEnhancedTrainer) ImportEnhancedPatterns(data []byte) error {
	return aet.referenceTrainer.ImportPatterns(data)
}
