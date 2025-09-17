package crawler

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gocolly/colly"
)

// ContentPattern representa um padrão de conteúdo aprendido
type ContentPattern struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"` // "catalog" ou "property"
	Features   map[string]interface{} `json:"features"`
	Confidence float64                `json:"confidence"`
	Examples   []ContentExample       `json:"examples"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	MatchCount int                    `json:"match_count"`
}

// ContentExample representa um exemplo de conteúdo usado para aprendizado
type ContentExample struct {
	URL        string                 `json:"url"`
	Features   map[string]interface{} `json:"features"`
	TextSample string                 `json:"text_sample"` // Primeiros 200 chars
	AddedAt    time.Time              `json:"added_at"`
}

// ContentBasedPatternLearner aprende padrões baseado no conteúdo das páginas
type ContentBasedPatternLearner struct {
	catalogPatterns  map[string]*ContentPattern
	propertyPatterns map[string]*ContentPattern
	mutex            sync.RWMutex
	logger           *logger.Logger
}

// NewContentBasedPatternLearner cria um novo aprendiz baseado em conteúdo
func NewContentBasedPatternLearner() *ContentBasedPatternLearner {
	return &ContentBasedPatternLearner{
		catalogPatterns:  make(map[string]*ContentPattern),
		propertyPatterns: make(map[string]*ContentPattern),
		logger:           logger.NewLogger("content_pattern_learner"),
	}
}

// LearnFromCatalogPages aprende padrões de páginas de catálogo
func (cpl *ContentBasedPatternLearner) LearnFromCatalogPages(examples []ContentExample) error {
	cpl.mutex.Lock()
	defer cpl.mutex.Unlock()

	cpl.logger.WithField("examples_count", len(examples)).Info("Learning catalog patterns from page content")

	// Analisa características comuns das páginas de catálogo
	catalogFeatures := cpl.extractCommonFeatures(examples, "catalog")

	// Cria ou atualiza padrões
	for featureType, pattern := range catalogFeatures {
		patternID := fmt.Sprintf("catalog_%s_%d", featureType, time.Now().Unix())

		cpl.catalogPatterns[patternID] = &ContentPattern{
			ID:         patternID,
			Type:       "catalog",
			Features:   pattern.Features,
			Confidence: pattern.Confidence,
			Examples:   examples,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			MatchCount: 1,
		}
	}

	cpl.logger.WithField("patterns_learned", len(catalogFeatures)).Info("Catalog content patterns learned")
	return nil
}

// LearnFromPropertyPages aprende padrões de páginas de propriedade
func (cpl *ContentBasedPatternLearner) LearnFromPropertyPages(examples []ContentExample) error {
	cpl.mutex.Lock()
	defer cpl.mutex.Unlock()

	cpl.logger.WithField("examples_count", len(examples)).Info("Learning property patterns from page content")

	// Analisa características comuns das páginas de propriedade
	propertyFeatures := cpl.extractCommonFeatures(examples, "property")

	// Cria ou atualiza padrões
	for featureType, pattern := range propertyFeatures {
		patternID := fmt.Sprintf("property_%s_%d", featureType, time.Now().Unix())

		cpl.propertyPatterns[patternID] = &ContentPattern{
			ID:         patternID,
			Type:       "property",
			Features:   pattern.Features,
			Confidence: pattern.Confidence,
			Examples:   examples,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			MatchCount: 1,
		}
	}

	cpl.logger.WithField("patterns_learned", len(propertyFeatures)).Info("Property content patterns learned")
	return nil
}

// extractCommonFeatures extrai características comuns dos exemplos
func (cpl *ContentBasedPatternLearner) extractCommonFeatures(examples []ContentExample, pageType string) map[string]*ContentPattern {
	patterns := make(map[string]*ContentPattern)

	if len(examples) == 0 {
		return patterns
	}

	// Analisa características agregadas
	totalExamples := len(examples)

	// Características de preço
	priceFeatures := cpl.analyzePricePatterns(examples)
	if priceFeatures != nil {
		patterns["price"] = &ContentPattern{
			Features:   priceFeatures,
			Confidence: cpl.calculateConfidence(totalExamples, pageType),
		}
	}

	// Características de texto
	textFeatures := cpl.analyzeTextPatterns(examples)
	if textFeatures != nil {
		patterns["text"] = &ContentPattern{
			Features:   textFeatures,
			Confidence: cpl.calculateConfidence(totalExamples, pageType),
		}
	}

	// Características estruturais
	structureFeatures := cpl.analyzeStructurePatterns(examples)
	if structureFeatures != nil {
		patterns["structure"] = &ContentPattern{
			Features:   structureFeatures,
			Confidence: cpl.calculateConfidence(totalExamples, pageType),
		}
	}

	return patterns
}

// analyzePricePatterns analisa padrões relacionados a preços
func (cpl *ContentBasedPatternLearner) analyzePricePatterns(examples []ContentExample) map[string]interface{} {
	features := make(map[string]interface{})

	totalPrices := 0
	priceCountSamples := make([]int, 0)

	for _, example := range examples {
		if priceCount, ok := example.Features["price_count"].(int); ok {
			totalPrices += priceCount
			priceCountSamples = append(priceCountSamples, priceCount)
		}
	}

	if len(priceCountSamples) > 0 {
		avgPrices := float64(totalPrices) / float64(len(priceCountSamples))
		features["avg_price_count"] = avgPrices
		features["min_price_count"] = cpl.minInt(priceCountSamples)
		features["max_price_count"] = cpl.maxInt(priceCountSamples)

		// Determina se é padrão de múltiplos preços (catálogo) ou preço único (propriedade)
		if avgPrices > 3 {
			features["pattern_type"] = "multiple_prices" // Indica catálogo
		} else {
			features["pattern_type"] = "single_price" // Indica propriedade
		}

		return features
	}

	return nil
}

// analyzeTextPatterns analisa padrões de texto
func (cpl *ContentBasedPatternLearner) analyzeTextPatterns(examples []ContentExample) map[string]interface{} {
	features := make(map[string]interface{})

	// Palavras-chave comuns em catálogos
	catalogKeywords := []string{
		"imóveis", "resultados", "encontrados", "buscar", "filtrar",
		"ordenar", "página", "próxima", "anterior", "listagem",
		"catálogo", "pesquisa", "busca", "filtros",
	}

	// Palavras-chave comuns em propriedades individuais
	propertyKeywords := []string{
		"detalhes", "descrição", "características", "localização",
		"galeria", "fotos", "contato", "agendar", "visita",
		"proprietário", "corretor", "exclusivo",
	}

	catalogScore := 0
	propertyScore := 0
	totalWords := 0

	for _, example := range examples {
		if textSample, ok := example.Features["text_sample"].(string); ok {
			textLower := strings.ToLower(textSample)
			totalWords += len(strings.Fields(textSample))

			// Conta palavras-chave de catálogo
			for _, keyword := range catalogKeywords {
				catalogScore += strings.Count(textLower, keyword)
			}

			// Conta palavras-chave de propriedade
			for _, keyword := range propertyKeywords {
				propertyScore += strings.Count(textLower, keyword)
			}
		}
	}

	if totalWords > 0 {
		features["catalog_keyword_density"] = float64(catalogScore) / float64(totalWords)
		features["property_keyword_density"] = float64(propertyScore) / float64(totalWords)
		features["dominant_pattern"] = "catalog"

		if propertyScore > catalogScore {
			features["dominant_pattern"] = "property"
		}

		return features
	}

	return nil
}

// analyzeStructurePatterns analisa padrões estruturais
func (cpl *ContentBasedPatternLearner) analyzeStructurePatterns(examples []ContentExample) map[string]interface{} {
	features := make(map[string]interface{})

	totalLinks := 0
	totalImages := 0
	hasPagination := 0
	hasFilters := 0

	for _, example := range examples {
		if linkCount, ok := example.Features["link_count"].(int); ok {
			totalLinks += linkCount
		}
		if imageCount, ok := example.Features["image_count"].(int); ok {
			totalImages += imageCount
		}
		if pagination, ok := example.Features["has_pagination"].(bool); ok && pagination {
			hasPagination++
		}
		if filters, ok := example.Features["has_filters"].(bool); ok && filters {
			hasFilters++
		}
	}

	exampleCount := len(examples)
	if exampleCount > 0 {
		features["avg_link_count"] = float64(totalLinks) / float64(exampleCount)
		features["avg_image_count"] = float64(totalImages) / float64(exampleCount)
		features["pagination_frequency"] = float64(hasPagination) / float64(exampleCount)
		features["filters_frequency"] = float64(hasFilters) / float64(exampleCount)

		// Determina padrão estrutural
		if float64(hasPagination)/float64(exampleCount) > 0.5 || float64(hasFilters)/float64(exampleCount) > 0.3 {
			features["structure_pattern"] = "catalog" // Muita paginação/filtros = catálogo
		} else {
			features["structure_pattern"] = "property" // Pouca paginação/filtros = propriedade
		}

		return features
	}

	return nil
}

// ClassifyPageContent classifica uma página baseado no seu conteúdo
func (cpl *ContentBasedPatternLearner) ClassifyPageContent(e *colly.HTMLElement) (string, float64) {
	cpl.mutex.RLock()
	defer cpl.mutex.RUnlock()

	// Extrai características da página atual
	currentFeatures := cpl.extractPageFeatures(e)
	text := strings.ToLower(e.Text)

	// REGRAS HEURÍSTICAS MELHORADAS (mais rigorosas e específicas)
	// Indicadores FORTES de catálogo (alta confiança) - frases completas para evitar falsos positivos
	strongCatalogIndicators := []string{
		"imóveis ao buscar", "resultados encontrados", "imobiliária em",
		"filtrar por", "ordenar por", "próxima página", "página anterior",
		"buscar imóveis", "encontrados", "resultados para", "8 imóveis",
		"resultados da busca", "refinar busca", "ordenar resultados",
		"mostrando resultados", "total de imóveis", "páginas de resultados",
	}
	
	for _, indicator := range strongCatalogIndicators {
		if strings.Contains(text, indicator) {
			cpl.logger.WithFields(map[string]interface{}{
				"url": e.Request.URL.String(),
				"strong_catalog_indicator": indicator,
			}).Debug("Strong catalog indicator found")
			return "catalog", 0.95 // Alta confiança
		}
	}
	
	// Indicadores FORTES de propriedade individual
	strongPropertyIndicators := []string{
		"descrição do imóvel", "características do imóvel", "detalhes do imóvel",
		"agendar visita", "entre em contato", "proprietário", "corretor responsável",
		"valor do imóvel", "área construída", "área total", "iptu",
	}
	
	for _, indicator := range strongPropertyIndicators {
		if strings.Contains(text, indicator) {
			cpl.logger.WithFields(map[string]interface{}{
				"url": e.Request.URL.String(),
				"strong_property_indicator": indicator,
			}).Debug("Strong property indicator found")
			return "property", 0.90 // Alta confiança
		}
	}

	// Análise por contagem de preços (melhorada)
	if priceCount, ok := currentFeatures["price_count"].(int); ok {
		if priceCount > 3 { // Múltiplos preços = catálogo
			cpl.logger.WithFields(map[string]interface{}{
				"url": e.Request.URL.String(),
				"price_count": priceCount,
			}).Debug("Multiple prices indicate catalog")
			return "catalog", 0.85
		}
	}

	// Análise por contagem de imóveis (mais rigorosa para evitar falsos positivos de navegação)
	if imoveisCount, ok := currentFeatures["imoveis_count"].(int); ok {
		if imoveisCount > 8 { // Aumentado de 5 para 8 - muitas referências a "imóveis" = catálogo
			cpl.logger.WithFields(map[string]interface{}{
				"url": e.Request.URL.String(),
				"imoveis_count": imoveisCount,
			}).Debug("Very high imoveis count indicates catalog")
			return "catalog", 0.80
		}
	}

	// NOVA REGRA: Detecta páginas com conteúdo muito genérico/repetitivo
	if textLength, ok := currentFeatures["text_length"].(int); ok {
		if textLength < 500 { // Páginas muito curtas podem ser filtros/erro
			cpl.logger.WithFields(map[string]interface{}{
				"url": e.Request.URL.String(),
				"text_length": textLength,
			}).Debug("Very short content might indicate filter/error page")
			// Não retorna diretamente, deixa para análise de padrões
		}
	}

	// Compara com padrões de catálogo
	catalogScore := cpl.matchContentPatterns(currentFeatures, cpl.catalogPatterns)

	// Compara com padrões de propriedade
	propertyScore := cpl.matchContentPatterns(currentFeatures, cpl.propertyPatterns)

	cpl.logger.WithFields(map[string]interface{}{
		"url":            e.Request.URL.String(),
		"catalog_score":  catalogScore,
		"property_score": propertyScore,
	}).Debug("Content classification scores")

	// Threshold mais rigoroso para evitar falsos positivos
	minThreshold := 0.25 // Aumentado de 0.1 para 0.25

	if catalogScore > propertyScore && catalogScore > minThreshold {
		return "catalog", catalogScore
	} else if propertyScore > catalogScore && propertyScore > minThreshold {
		return "property", propertyScore
	}

	// Se não conseguiu classificar com confiança, assume como "unknown"
	return "unknown", 0.0
}

// extractPageFeatures extrai características de uma página
func (cpl *ContentBasedPatternLearner) extractPageFeatures(e *colly.HTMLElement) map[string]interface{} {
	features := make(map[string]interface{})

	text := e.Text
	textLower := strings.ToLower(text)

	// Características de preço
	priceRegex := regexp.MustCompile(`R\$\s*[\d.,]+`)
	priceMatches := priceRegex.FindAllString(text, -1)
	features["price_count"] = len(priceMatches)

	// Características de texto
	features["text_length"] = len(text)
	features["text_sample"] = cpl.truncateText(text, 200)

	// Palavras-chave específicas
	features["quartos_count"] = strings.Count(textLower, "quartos")
	features["banheiros_count"] = strings.Count(textLower, "banheiros")
	features["imoveis_count"] = strings.Count(textLower, "imóveis") + strings.Count(textLower, "imoveis")

	// Características estruturais
	features["link_count"] = len(e.ChildAttrs("a", "href"))
	features["image_count"] = len(e.ChildAttrs("img", "src"))
	features["has_pagination"] = e.ChildText(".pagination") != "" || e.ChildText(".paginacao") != "" || strings.Contains(textLower, "próxima") || strings.Contains(textLower, "anterior")
	features["has_filters"] = e.ChildText(".filters") != "" || e.ChildText(".filtros") != "" || strings.Contains(textLower, "filtrar") || strings.Contains(textLower, "ordenar")

	return features
}

// matchContentPatterns compara características atuais com padrões aprendidos
func (cpl *ContentBasedPatternLearner) matchContentPatterns(currentFeatures map[string]interface{}, patterns map[string]*ContentPattern) float64 {
	maxScore := 0.0

	for _, pattern := range patterns {
		score := cpl.calculateSimilarity(currentFeatures, pattern.Features)
		adjustedScore := score * pattern.Confidence

		if adjustedScore > maxScore {
			maxScore = adjustedScore
		}
	}

	return maxScore
}

// calculateSimilarity calcula similaridade entre características
func (cpl *ContentBasedPatternLearner) calculateSimilarity(current, pattern map[string]interface{}) float64 {
	totalScore := 0.0
	comparisons := 0

	// Compara preços
	if currentPrices, ok := current["price_count"].(int); ok {
		if patternAvg, ok := pattern["avg_price_count"].(float64); ok {
			// Score baseado na proximidade da contagem de preços
			diff := float64(currentPrices) - patternAvg
			if diff < 0 {
				diff = -diff
			}

			// Score inverso da diferença (quanto menor a diferença, maior o score)
			priceScore := 1.0 / (1.0 + diff/10.0) // Normaliza a diferença
			totalScore += priceScore * 0.4        // Peso 40% para preços
			comparisons++
		}
	}

	// Compara densidade de palavras-chave
	if catalogDensity, ok := pattern["catalog_keyword_density"].(float64); ok {
		if textSample, ok := current["text_sample"].(string); ok {
			currentDensity := cpl.calculateKeywordDensity(textSample, []string{"imóveis", "resultados", "buscar", "filtrar"})
			densityScore := 1.0 - (catalogDensity-currentDensity)*(catalogDensity-currentDensity) // Score baseado na proximidade
			if densityScore < 0 {
				densityScore = 0
			}
			totalScore += densityScore * 0.3 // Peso 30% para palavras-chave
			comparisons++
		}
	}

	// Compara características estruturais
	if hasPagination, ok := current["has_pagination"].(bool); ok {
		if paginationFreq, ok := pattern["pagination_frequency"].(float64); ok {
			structureScore := 0.0
			if hasPagination && paginationFreq > 0.5 {
				structureScore = 1.0 // Match: tem paginação e padrão espera paginação
			} else if !hasPagination && paginationFreq <= 0.5 {
				structureScore = 1.0 // Match: não tem paginação e padrão não espera
			}
			totalScore += structureScore * 0.3 // Peso 30% para estrutura
			comparisons++
		}
	}

	if comparisons > 0 {
		return totalScore / float64(comparisons)
	}

	return 0.0
}

// calculateKeywordDensity calcula densidade de palavras-chave
func (cpl *ContentBasedPatternLearner) calculateKeywordDensity(text string, keywords []string) float64 {
	textLower := strings.ToLower(text)
	totalWords := len(strings.Fields(text))
	keywordCount := 0

	for _, keyword := range keywords {
		keywordCount += strings.Count(textLower, keyword)
	}

	if totalWords > 0 {
		return float64(keywordCount) / float64(totalWords)
	}

	return 0.0
}

// calculateConfidence calcula confiança baseado no número de exemplos
func (cpl *ContentBasedPatternLearner) calculateConfidence(exampleCount int, pageType string) float64 {
	baseConfidence := float64(exampleCount) / 5.0 // Máximo 1.0 com 5+ exemplos (mais generoso)
	if baseConfidence > 1.0 {
		baseConfidence = 1.0
	}

	// Confiança mínima de 0.3 para começar a classificar (mais baixa)
	if baseConfidence < 0.3 {
		baseConfidence = 0.3
	}

	return baseConfidence
}

// Funções auxiliares
func (cpl *ContentBasedPatternLearner) minInt(slice []int) int {
	if len(slice) == 0 {
		return 0
	}
	min := slice[0]
	for _, v := range slice[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func (cpl *ContentBasedPatternLearner) maxInt(slice []int) int {
	if len(slice) == 0 {
		return 0
	}
	max := slice[0]
	for _, v := range slice[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func (cpl *ContentBasedPatternLearner) truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// GetLearnedPatterns retorna todos os padrões aprendidos
func (cpl *ContentBasedPatternLearner) GetLearnedPatterns() map[string][]*ContentPattern {
	cpl.mutex.RLock()
	defer cpl.mutex.RUnlock()

	result := make(map[string][]*ContentPattern)

	var catalogPatterns []*ContentPattern
	for _, pattern := range cpl.catalogPatterns {
		catalogPatterns = append(catalogPatterns, pattern)
	}
	result["catalog"] = catalogPatterns

	var propertyPatterns []*ContentPattern
	for _, pattern := range cpl.propertyPatterns {
		propertyPatterns = append(propertyPatterns, pattern)
	}
	result["property"] = propertyPatterns

	return result
}

// ExportPatterns exporta os padrões para JSON
func (cpl *ContentBasedPatternLearner) ExportPatterns() ([]byte, error) {
	patterns := cpl.GetLearnedPatterns()
	return json.MarshalIndent(patterns, "", "  ")
}

// ImportPatterns importa padrões de JSON
func (cpl *ContentBasedPatternLearner) ImportPatterns(data []byte) error {
	cpl.mutex.Lock()
	defer cpl.mutex.Unlock()

	var importedData map[string][]*ContentPattern
	if err := json.Unmarshal(data, &importedData); err != nil {
		return fmt.Errorf("failed to unmarshal patterns: %w", err)
	}

	// Reconstrói os mapas de padrões
	cpl.catalogPatterns = make(map[string]*ContentPattern)
	cpl.propertyPatterns = make(map[string]*ContentPattern)

	if catalogPatterns, ok := importedData["catalog"]; ok {
		for _, pattern := range catalogPatterns {
			cpl.catalogPatterns[pattern.ID] = pattern
		}
	}

	if propertyPatterns, ok := importedData["property"]; ok {
		for _, pattern := range propertyPatterns {
			cpl.propertyPatterns[pattern.ID] = pattern
		}
	}

	cpl.logger.Info("Content-based patterns imported successfully")
	return nil
}
