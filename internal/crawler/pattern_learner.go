package crawler

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
)

// LearnedPattern representa um padrão aprendido
type LearnedPattern struct {
	ID         string    `json:"id"`
	Pattern    string    `json:"pattern"`
	Type       string    `json:"type"` // "catalog" ou "property"
	Confidence float64   `json:"confidence"`
	Examples   []string  `json:"examples"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	MatchCount int       `json:"match_count"`
}

// PatternLearner aprende padrões de URLs
type PatternLearner struct {
	catalogPatterns  map[string]*LearnedPattern
	propertyPatterns map[string]*LearnedPattern
	mutex            sync.RWMutex
	logger           *logger.Logger
}

// NewPatternLearner cria um novo aprendiz de padrões
func NewPatternLearner() *PatternLearner {
	return &PatternLearner{
		catalogPatterns:  make(map[string]*LearnedPattern),
		propertyPatterns: make(map[string]*LearnedPattern),
		logger:           logger.NewLogger("pattern_learner"),
	}
}

// LearnCatalogURLs aprende padrões de URLs de catálogo
func (pl *PatternLearner) LearnCatalogURLs(urls []string) error {
	pl.mutex.Lock()
	defer pl.mutex.Unlock()

	pl.logger.WithField("url_count", len(urls)).Info("Learning catalog URL patterns")

	patterns := pl.extractPatternsFromURLs(urls, "catalog")

	for _, pattern := range patterns {
		existing, exists := pl.catalogPatterns[pattern.Pattern]
		if exists {
			// Atualiza padrão existente
			existing.Examples = append(existing.Examples, pattern.Examples...)
			existing.UpdatedAt = time.Now()
			existing.Confidence = pl.calculateConfidence(existing.Examples)
			existing.MatchCount++
		} else {
			// Novo padrão
			pl.catalogPatterns[pattern.Pattern] = pattern
		}
	}

	pl.logger.WithField("patterns_learned", len(patterns)).Info("Catalog patterns learned successfully")
	return nil
}

// LearnPropertyURLs aprende padrões de URLs de propriedade
func (pl *PatternLearner) LearnPropertyURLs(urls []string) error {
	pl.mutex.Lock()
	defer pl.mutex.Unlock()

	pl.logger.WithField("url_count", len(urls)).Info("Learning property URL patterns")

	patterns := pl.extractPatternsFromURLs(urls, "property")

	for _, pattern := range patterns {
		existing, exists := pl.propertyPatterns[pattern.Pattern]
		if exists {
			existing.Examples = append(existing.Examples, pattern.Examples...)
			existing.UpdatedAt = time.Now()
			existing.Confidence = pl.calculateConfidence(existing.Examples)
			existing.MatchCount++
		} else {
			pl.propertyPatterns[pattern.Pattern] = pattern
		}
	}

	pl.logger.WithField("patterns_learned", len(patterns)).Info("Property patterns learned successfully")
	return nil
}

// extractPatternsFromURLs extrai padrões comuns de uma lista de URLs
func (pl *PatternLearner) extractPatternsFromURLs(urls []string, patternType string) []*LearnedPattern {
	var patterns []*LearnedPattern

	// Agrupa URLs por domínio
	domainGroups := make(map[string][]string)
	for _, rawURL := range urls {
		if parsedURL, err := url.Parse(rawURL); err == nil {
			domain := parsedURL.Host
			domainGroups[domain] = append(domainGroups[domain], rawURL)
		}
	}

	// Para cada domínio, extrai padrões
	for domain, domainURLs := range domainGroups {
		domainPatterns := pl.extractDomainPatterns(domain, domainURLs, patternType)
		patterns = append(patterns, domainPatterns...)
	}

	return patterns
}

// extractDomainPatterns extrai padrões específicos de um domínio
func (pl *PatternLearner) extractDomainPatterns(domain string, urls []string, patternType string) []*LearnedPattern {
	var patterns []*LearnedPattern

	// Extrai padrões de path
	pathPatterns := pl.extractPathPatterns(urls, patternType)
	for _, pattern := range pathPatterns {
		pattern.ID = fmt.Sprintf("%s_%s_%d", domain, patternType, time.Now().Unix())
		patterns = append(patterns, pattern)
	}

	// Extrai padrões de parâmetros
	paramPatterns := pl.extractParameterPatterns(urls, patternType)
	for _, pattern := range paramPatterns {
		pattern.ID = fmt.Sprintf("%s_%s_param_%d", domain, patternType, time.Now().Unix())
		patterns = append(patterns, pattern)
	}

	return patterns
}

// extractPathPatterns extrai padrões de path das URLs
func (pl *PatternLearner) extractPathPatterns(urls []string, patternType string) []*LearnedPattern {
	var patterns []*LearnedPattern
	pathCounts := make(map[string][]string)

	for _, rawURL := range urls {
		if parsedURL, err := url.Parse(rawURL); err == nil {
			path := parsedURL.Path

			// Normaliza o path para criar padrões
			normalizedPatterns := pl.normalizePath(path)

			for _, pattern := range normalizedPatterns {
				pathCounts[pattern] = append(pathCounts[pattern], rawURL)
			}
		}
	}

	// Cria padrões para paths que aparecem múltiplas vezes
	for pattern, examples := range pathCounts {
		if len(examples) >= 2 { // Pelo menos 2 exemplos
			confidence := pl.calculateConfidence(examples)

			patterns = append(patterns, &LearnedPattern{
				Pattern:    pattern,
				Type:       patternType,
				Confidence: confidence,
				Examples:   examples,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				MatchCount: 1,
			})
		}
	}

	return patterns
}

// extractParameterPatterns extrai padrões de parâmetros das URLs
func (pl *PatternLearner) extractParameterPatterns(urls []string, patternType string) []*LearnedPattern {
	var patterns []*LearnedPattern
	paramCounts := make(map[string][]string)

	for _, rawURL := range urls {
		if parsedURL, err := url.Parse(rawURL); err == nil {
			query := parsedURL.RawQuery
			if query != "" {
				// Extrai padrões de parâmetros
				paramPatterns := pl.extractQueryPatterns(query)

				for _, pattern := range paramPatterns {
					paramCounts[pattern] = append(paramCounts[pattern], rawURL)
				}
			}
		}
	}

	// Cria padrões para parâmetros que aparecem múltiplas vezes
	for pattern, examples := range paramCounts {
		if len(examples) >= 2 {
			confidence := pl.calculateConfidence(examples)

			patterns = append(patterns, &LearnedPattern{
				Pattern:    pattern,
				Type:       patternType,
				Confidence: confidence,
				Examples:   examples,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				MatchCount: 1,
			})
		}
	}

	return patterns
}

// normalizePath normaliza um path para criar padrões generalizados
func (pl *PatternLearner) normalizePath(path string) []string {
	var patterns []string

	// Remove trailing slash
	path = strings.TrimSuffix(path, "/")

	// Padrão exato
	patterns = append(patterns, path)

	// Substitui números por placeholder
	numberPattern := regexp.MustCompile(`\d+`)
	pathWithNumbers := numberPattern.ReplaceAllString(path, "{id}")
	if pathWithNumbers != path {
		patterns = append(patterns, pathWithNumbers)
	}

	// Padrões de segmentos
	segments := strings.Split(path, "/")
	if len(segments) > 2 {
		// Padrão dos primeiros 2 segmentos
		firstTwo := strings.Join(segments[:3], "/") // inclui o "" inicial
		patterns = append(patterns, firstTwo+"/*")

		// Padrão dos últimos segmentos
		if len(segments) > 3 {
			lastSegment := segments[len(segments)-1]
			if numberPattern.MatchString(lastSegment) {
				// Se o último segmento é número, cria padrão genérico
				genericPath := strings.Join(segments[:len(segments)-1], "/") + "/{id}"
				patterns = append(patterns, genericPath)
			}
		}
	}

	return patterns
}

// extractQueryPatterns extrai padrões de query string
func (pl *PatternLearner) extractQueryPatterns(query string) []string {
	var patterns []string

	// Padrões de parâmetros específicos
	if strings.Contains(query, "page=") || strings.Contains(query, "pagina=") {
		patterns = append(patterns, "has_pagination")
	}

	if strings.Contains(query, "filtro") || strings.Contains(query, "filter") {
		patterns = append(patterns, "has_filters")
	}

	if strings.Contains(query, "ordenar") || strings.Contains(query, "sort") {
		patterns = append(patterns, "has_sorting")
	}

	if strings.Contains(query, "busca") || strings.Contains(query, "search") || strings.Contains(query, "q=") {
		patterns = append(patterns, "has_search")
	}

	return patterns
}

// calculateConfidence calcula a confiança de um padrão baseado nos exemplos
func (pl *PatternLearner) calculateConfidence(examples []string) float64 {
	if len(examples) == 0 {
		return 0.0
	}

	// Confiança baseada no número de exemplos
	baseConfidence := float64(len(examples)) / 10.0 // Max 1.0 com 10+ exemplos
	if baseConfidence > 1.0 {
		baseConfidence = 1.0
	}

	return baseConfidence
}

// ClassifyURL classifica uma URL baseada nos padrões aprendidos
func (pl *PatternLearner) ClassifyURL(rawURL string) (string, float64) {
	pl.mutex.RLock()
	defer pl.mutex.RUnlock()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "unknown", 0.0
	}

	// Verifica padrões de catálogo
	catalogScore := pl.matchPatterns(rawURL, parsedURL, pl.catalogPatterns)

	// Verifica padrões de propriedade
	propertyScore := pl.matchPatterns(rawURL, parsedURL, pl.propertyPatterns)

	if catalogScore > propertyScore && catalogScore > 0.1 {
		return "catalog", catalogScore
	} else if propertyScore > catalogScore && propertyScore > 0.1 {
		return "property", propertyScore
	}

	return "unknown", 0.0
}

// matchPatterns verifica se uma URL corresponde aos padrões aprendidos
func (pl *PatternLearner) matchPatterns(rawURL string, parsedURL *url.URL, patterns map[string]*LearnedPattern) float64 {
	maxScore := 0.0

	path := parsedURL.Path
	query := parsedURL.RawQuery

	for _, pattern := range patterns {
		score := 0.0

		// Verifica padrões de path
		if strings.Contains(pattern.Pattern, "/") {
			if pl.matchPathPattern(path, pattern.Pattern) {
				score = pattern.Confidence
			}
		} else {
			// Verifica padrões de parâmetros
			if pl.matchQueryPattern(query, pattern.Pattern) {
				score = pattern.Confidence * 0.8 // Peso menor para parâmetros
			}
		}

		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore
}

// matchPathPattern verifica se um path corresponde a um padrão
func (pl *PatternLearner) matchPathPattern(path, pattern string) bool {
	// Padrão exato
	if path == pattern {
		return true
	}

	// Padrão com wildcard
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}

	// Padrão com placeholder {id}
	if strings.Contains(pattern, "{id}") {
		// Escapa caracteres especiais da regex ANTES de substituir {id}
		escapedPattern := regexp.QuoteMeta(pattern)
		// Substitui o {id} escapado por \d+
		regex := strings.ReplaceAll(escapedPattern, `\{id\}`, `\d+`)
		if matched, _ := regexp.MatchString("^"+regex+"$", path); matched {
			return true
		}
	}

	return false
}

// matchQueryPattern verifica se uma query corresponde a um padrão
func (pl *PatternLearner) matchQueryPattern(query, pattern string) bool {
	switch pattern {
	case "has_pagination":
		return strings.Contains(query, "page=") || strings.Contains(query, "pagina=")
	case "has_filters":
		return strings.Contains(query, "filtro") || strings.Contains(query, "filter")
	case "has_sorting":
		return strings.Contains(query, "ordenar") || strings.Contains(query, "sort")
	case "has_search":
		return strings.Contains(query, "busca") || strings.Contains(query, "search") || strings.Contains(query, "q=")
	}

	return false
}

// GetLearnedPatterns retorna todos os padrões aprendidos
func (pl *PatternLearner) GetLearnedPatterns() map[string][]*LearnedPattern {
	pl.mutex.RLock()
	defer pl.mutex.RUnlock()

	result := make(map[string][]*LearnedPattern)

	var catalogPatterns []*LearnedPattern
	for _, pattern := range pl.catalogPatterns {
		catalogPatterns = append(catalogPatterns, pattern)
	}
	result["catalog"] = catalogPatterns

	var propertyPatterns []*LearnedPattern
	for _, pattern := range pl.propertyPatterns {
		propertyPatterns = append(propertyPatterns, pattern)
	}
	result["property"] = propertyPatterns

	return result
}

// ExportPatterns exporta os padrões para JSON
func (pl *PatternLearner) ExportPatterns() ([]byte, error) {
	patterns := pl.GetLearnedPatterns()
	return json.MarshalIndent(patterns, "", "  ")
}

// ImportPatterns importa padrões de JSON
func (pl *PatternLearner) ImportPatterns(data []byte) error {
	pl.mutex.Lock()
	defer pl.mutex.Unlock()

	var patterns map[string][]*LearnedPattern
	if err := json.Unmarshal(data, &patterns); err != nil {
		return err
	}

	// Importa padrões de catálogo
	if catalogPatterns, exists := patterns["catalog"]; exists {
		for _, pattern := range catalogPatterns {
			pl.catalogPatterns[pattern.Pattern] = pattern
		}
	}

	// Importa padrões de propriedade
	if propertyPatterns, exists := patterns["property"]; exists {
		for _, pattern := range propertyPatterns {
			pl.propertyPatterns[pattern.Pattern] = pattern
		}
	}

	pl.logger.Info("Patterns imported successfully")
	return nil
}
