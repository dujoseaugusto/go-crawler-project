package crawler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

// ReferencePattern representa um padrão aprendido de páginas de referência
type ReferencePattern struct {
	ID          string                 `json:"id"`
	Domain      string                 `json:"domain"`
	URLPattern  string                 `json:"url_pattern"`
	Selectors   map[string][]string    `json:"selectors"`
	Features    map[string]interface{} `json:"features"`
	Confidence  float64                `json:"confidence"`
	Examples    []string               `json:"examples"`
	CreatedAt   time.Time              `json:"created_at"`
	LastTested  time.Time              `json:"last_tested"`
	SuccessRate float64                `json:"success_rate"`
}

// ReferencePatternTrainer treina padrões baseado em páginas de referência conhecidas
type ReferencePatternTrainer struct {
	patterns    map[string]*ReferencePattern
	mutex       sync.RWMutex
	logger      *logger.Logger
	collector   *colly.Collector
	testResults map[string][]bool // Para calcular taxa de sucesso
}

// NewReferencePatternTrainer cria um novo treinador baseado em referências
func NewReferencePatternTrainer() *ReferencePatternTrainer {
	c := colly.NewCollector(
		colly.MaxDepth(1),
		colly.Async(false),
	)

	// Configurações do collector
	extensions.RandomUserAgent(c)
	extensions.Referer(c)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	return &ReferencePatternTrainer{
		patterns:    make(map[string]*ReferencePattern),
		logger:      logger.NewLogger("reference_pattern_trainer"),
		collector:   c,
		testResults: make(map[string][]bool),
	}
}

// TrainFromReferenceFile treina padrões usando um arquivo de referência (como List-site.ini)
func (rpt *ReferencePatternTrainer) TrainFromReferenceFile(ctx context.Context, filePath string) error {
	rpt.logger.WithField("file", filePath).Info("Starting training from reference file")

	// Lê URLs do arquivo
	urls, err := rpt.loadURLsFromFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to load URLs from file: %w", err)
	}

	rpt.logger.WithField("url_count", len(urls)).Info("Loaded reference URLs")

	// Analisa cada URL
	for i, rawURL := range urls {
		if rawURL == "" {
			continue
		}

		rpt.logger.WithFields(map[string]interface{}{
			"url":      rawURL,
			"progress": fmt.Sprintf("%d/%d", i+1, len(urls)),
		}).Info("Analyzing reference URL")

		if err := rpt.analyzeReferenceURL(ctx, rawURL); err != nil {
			rpt.logger.WithError(err).WithField("url", rawURL).Warn("Failed to analyze reference URL")
			continue
		}

		// Pequena pausa entre requisições
		time.Sleep(1 * time.Second)
	}

	// Consolida padrões por domínio
	rpt.consolidatePatterns()

	rpt.logger.WithField("patterns_learned", len(rpt.patterns)).Info("Training completed")
	return nil
}

// loadURLsFromFile carrega URLs de um arquivo (suporta diferentes formatos)
func (rpt *ReferencePatternTrainer) loadURLsFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			// Remove possíveis prefixos de numeração
			line = regexp.MustCompile(`^\d+\|`).ReplaceAllString(line, "")
			if strings.HasPrefix(line, "http") {
				urls = append(urls, line)
			}
		}
	}

	return urls, scanner.Err()
}

// analyzeReferenceURL analisa uma URL de referência para extrair padrões
func (rpt *ReferencePatternTrainer) analyzeReferenceURL(ctx context.Context, rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	domain := parsedURL.Host
	var pageData *PageAnalysisData

	// Configura callback para capturar dados da página
	rpt.collector.OnHTML("html", func(e *colly.HTMLElement) {
		pageData = rpt.extractPageData(e, rawURL)
	})

	// Visita a página
	if err := rpt.collector.Visit(rawURL); err != nil {
		return fmt.Errorf("failed to visit URL: %w", err)
	}

	// Aguarda processamento
	rpt.collector.Wait()

	if pageData == nil {
		return fmt.Errorf("no data extracted from page")
	}

	// Cria ou atualiza padrão para o domínio
	rpt.updateDomainPattern(domain, rawURL, pageData)

	return nil
}

// PageAnalysisData contém dados extraídos de uma página para análise
type PageAnalysisData struct {
	URL            string                 `json:"url"`
	Title          string                 `json:"title"`
	TextContent    string                 `json:"text_content"`
	Selectors      map[string][]string    `json:"selectors"`
	Features       map[string]interface{} `json:"features"`
	StructuralInfo map[string]interface{} `json:"structural_info"`
}

// extractPageData extrai dados detalhados de uma página
func (rpt *ReferencePatternTrainer) extractPageData(e *colly.HTMLElement, rawURL string) *PageAnalysisData {
	data := &PageAnalysisData{
		URL:            rawURL,
		Title:          e.ChildText("title"),
		TextContent:    e.Text,
		Selectors:      make(map[string][]string),
		Features:       make(map[string]interface{}),
		StructuralInfo: make(map[string]interface{}),
	}

	// Extrai seletores que contêm informações de imóveis
	rpt.extractPropertySelectors(e, data)

	// Extrai características da página
	rpt.extractPageFeatures(e, data)

	// Extrai informações estruturais
	rpt.extractStructuralInfo(e, data)

	return data
}

// extractPropertySelectors identifica seletores que contêm dados de imóveis
func (rpt *ReferencePatternTrainer) extractPropertySelectors(e *colly.HTMLElement, data *PageAnalysisData) {
	// Seletores candidatos para diferentes tipos de dados
	selectorCandidates := map[string][]string{
		"price": {
			".preco", ".price", ".valor", ".value", ".property-price", ".imovel-preco",
			"[class*='preco']", "[class*='price']", "[class*='valor']",
		},
		"address": {
			".endereco", ".address", ".location", ".localizacao", ".property-address",
			"[class*='endereco']", "[class*='address']", "[class*='location']",
		},
		"description": {
			".descricao", ".description", ".detalhes", ".details", ".property-description",
			"[class*='descricao']", "[class*='description']", "[class*='detalhes']",
		},
		"features": {
			".caracteristicas", ".features", ".amenities", ".comodidades",
			"[class*='caracteristicas']", "[class*='features']",
		},
		"rooms": {
			".quartos", ".dormitorios", ".bedrooms", ".rooms",
			"[class*='quartos']", "[class*='dormitorios']",
		},
		"area": {
			".area", ".metragem", ".size", ".m2",
			"[class*='area']", "[class*='metragem']",
		},
	}

	// Testa cada seletor e armazena os que retornam conteúdo válido
	for dataType, selectors := range selectorCandidates {
		var validSelectors []string

		for _, selector := range selectors {
			content := strings.TrimSpace(e.ChildText(selector))
			if content != "" && rpt.isValidContent(content, dataType) {
				validSelectors = append(validSelectors, selector)
			}
		}

		if len(validSelectors) > 0 {
			data.Selectors[dataType] = validSelectors
		}
	}
}

// isValidContent verifica se o conteúdo é válido para o tipo de dado
func (rpt *ReferencePatternTrainer) isValidContent(content, dataType string) bool {
	content = strings.ToLower(content)

	switch dataType {
	case "price":
		return strings.Contains(content, "r$") ||
			regexp.MustCompile(`\d+[.,]\d+`).MatchString(content)
	case "address":
		return len(content) > 10 && len(content) < 200 &&
			(strings.Contains(content, "rua") ||
				strings.Contains(content, "av") ||
				strings.Contains(content, "jardim") ||
				strings.Contains(content, "vila"))
	case "description":
		return len(content) > 20 && len(content) < 1000
	case "rooms":
		return strings.Contains(content, "quarto") ||
			strings.Contains(content, "dormitório") ||
			regexp.MustCompile(`\d+\s*(quarto|dormitório)`).MatchString(content)
	case "area":
		return strings.Contains(content, "m²") || strings.Contains(content, "m2") ||
			regexp.MustCompile(`\d+\s*m[²2]`).MatchString(content)
	default:
		return len(content) > 5 && len(content) < 500
	}
}

// extractPageFeatures extrai características da página
func (rpt *ReferencePatternTrainer) extractPageFeatures(e *colly.HTMLElement, data *PageAnalysisData) {
	text := strings.ToLower(e.Text)

	// Características básicas
	data.Features["text_length"] = len(e.Text)
	data.Features["title_length"] = len(data.Title)

	// Contagem de elementos importantes
	data.Features["price_count"] = len(regexp.MustCompile(`R\$\s*[\d.,]+`).FindAllString(e.Text, -1))
	data.Features["phone_count"] = len(regexp.MustCompile(`\(\d{2}\)\s*\d{4,5}-?\d{4}`).FindAllString(e.Text, -1))
	data.Features["email_count"] = len(regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`).FindAllString(e.Text, -1))

	// Palavras-chave específicas de imóveis
	propertyKeywords := []string{
		"quartos", "dormitórios", "banheiros", "suítes", "vagas",
		"área", "m²", "condomínio", "iptu", "financiamento",
	}

	keywordCount := 0
	for _, keyword := range propertyKeywords {
		keywordCount += strings.Count(text, keyword)
	}
	data.Features["property_keywords"] = keywordCount

	// Indicadores de página individual vs catálogo
	data.Features["has_gallery"] = len(e.ChildAttrs("img", "src")) > 5
	data.Features["has_contact_form"] = e.ChildText("form") != ""
	data.Features["has_pagination"] = strings.Contains(text, "próxima") || strings.Contains(text, "anterior")
	data.Features["has_filters"] = strings.Contains(text, "filtrar") || strings.Contains(text, "ordenar")
}

// extractStructuralInfo extrai informações estruturais da página
func (rpt *ReferencePatternTrainer) extractStructuralInfo(e *colly.HTMLElement, data *PageAnalysisData) {
	// Contagem de elementos estruturais
	data.StructuralInfo["div_count"] = len(e.ChildAttrs("div", "class"))
	data.StructuralInfo["img_count"] = len(e.ChildAttrs("img", "src"))
	data.StructuralInfo["link_count"] = len(e.ChildAttrs("a", "href"))
	data.StructuralInfo["form_count"] = len(e.ChildAttrs("form", "action"))

	// Classes CSS mais comuns (para identificar padrões)
	classNames := e.ChildAttrs("*", "class")
	classFreq := make(map[string]int)

	for _, className := range classNames {
		classes := strings.Fields(className)
		for _, class := range classes {
			if len(class) > 3 { // Ignora classes muito curtas
				classFreq[class]++
			}
		}
	}

	// Armazena as 10 classes mais comuns
	var topClasses []string
	for class, freq := range classFreq {
		if freq > 1 && len(topClasses) < 10 {
			topClasses = append(topClasses, class)
		}
	}
	data.StructuralInfo["common_classes"] = topClasses
}

// updateDomainPattern atualiza ou cria padrão para um domínio
func (rpt *ReferencePatternTrainer) updateDomainPattern(domain, rawURL string, pageData *PageAnalysisData) {
	rpt.mutex.Lock()
	defer rpt.mutex.Unlock()

	patternID := fmt.Sprintf("ref_%s", strings.ReplaceAll(domain, ".", "_"))

	existing, exists := rpt.patterns[patternID]
	if !exists {
		// Cria novo padrão
		rpt.patterns[patternID] = &ReferencePattern{
			ID:         patternID,
			Domain:     domain,
			URLPattern: rpt.extractURLPattern(rawURL),
			Selectors:  pageData.Selectors,
			Features:   pageData.Features,
			Confidence: 0.5, // Confiança inicial
			Examples:   []string{rawURL},
			CreatedAt:  time.Now(),
			LastTested: time.Now(),
		}
	} else {
		// Atualiza padrão existente
		existing.Examples = append(existing.Examples, rawURL)
		existing.LastTested = time.Now()

		// Mescla seletores (mantém os que funcionam consistentemente)
		rpt.mergeSelectors(existing.Selectors, pageData.Selectors)

		// Atualiza características (média ponderada)
		rpt.updateFeatures(existing.Features, pageData.Features)

		// Recalcula confiança baseado no número de exemplos
		existing.Confidence = rpt.calculateConfidence(len(existing.Examples))
	}
}

// extractURLPattern extrai padrão da URL para matching futuro
func (rpt *ReferencePatternTrainer) extractURLPattern(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	path := parsedURL.Path

	// Substitui números por placeholder para criar padrão genérico
	numberRegex := regexp.MustCompile(`\d+`)
	pattern := numberRegex.ReplaceAllString(path, "{id}")

	return pattern
}

// mergeSelectors mescla seletores de diferentes exemplos
func (rpt *ReferencePatternTrainer) mergeSelectors(existing, new map[string][]string) {
	for dataType, newSelectors := range new {
		if existingSelectors, ok := existing[dataType]; ok {
			// Adiciona novos seletores que não existem
			for _, newSelector := range newSelectors {
				found := false
				for _, existingSelector := range existingSelectors {
					if existingSelector == newSelector {
						found = true
						break
					}
				}
				if !found {
					existing[dataType] = append(existing[dataType], newSelector)
				}
			}
		} else {
			existing[dataType] = newSelectors
		}
	}
}

// updateFeatures atualiza características com média ponderada
func (rpt *ReferencePatternTrainer) updateFeatures(existing, new map[string]interface{}) {
	for key, newValue := range new {
		if existingValue, ok := existing[key]; ok {
			// Para valores numéricos, calcula média
			if existingFloat, ok := existingValue.(float64); ok {
				if newFloat, ok := newValue.(float64); ok {
					existing[key] = (existingFloat + newFloat) / 2.0
				}
			} else if existingInt, ok := existingValue.(int); ok {
				if newInt, ok := newValue.(int); ok {
					existing[key] = (existingInt + newInt) / 2
				}
			}
			// Para valores booleanos, mantém true se pelo menos um for true
			if existingBool, ok := existingValue.(bool); ok {
				if newBool, ok := newValue.(bool); ok {
					existing[key] = existingBool || newBool
				}
			}
		} else {
			existing[key] = newValue
		}
	}
}

// calculateConfidence calcula confiança baseado no número de exemplos
func (rpt *ReferencePatternTrainer) calculateConfidence(exampleCount int) float64 {
	// Confiança aumenta com mais exemplos, máximo 0.95
	confidence := 0.3 + (float64(exampleCount-1) * 0.1)
	if confidence > 0.95 {
		confidence = 0.95
	}
	return confidence
}

// consolidatePatterns consolida padrões similares
func (rpt *ReferencePatternTrainer) consolidatePatterns() {
	rpt.mutex.Lock()
	defer rpt.mutex.Unlock()

	// Agrupa padrões por domínio base (remove subdomínios)
	domainGroups := make(map[string][]*ReferencePattern)

	for _, pattern := range rpt.patterns {
		baseDomain := rpt.getBaseDomain(pattern.Domain)
		domainGroups[baseDomain] = append(domainGroups[baseDomain], pattern)
	}

	// Para cada grupo com múltiplos padrões, consolida
	for baseDomain, patterns := range domainGroups {
		if len(patterns) > 1 {
			consolidated := rpt.consolidateDomainPatterns(patterns)
			consolidated.ID = fmt.Sprintf("consolidated_%s", strings.ReplaceAll(baseDomain, ".", "_"))

			// Remove padrões individuais e adiciona consolidado
			for _, pattern := range patterns {
				delete(rpt.patterns, pattern.ID)
			}
			rpt.patterns[consolidated.ID] = consolidated
		}
	}
}

// getBaseDomain extrai domínio base (remove www, subdomínios)
func (rpt *ReferencePatternTrainer) getBaseDomain(domain string) string {
	// Remove www.
	domain = strings.TrimPrefix(domain, "www.")

	// Para domínios brasileiros, mantém apenas domínio.com.br
	parts := strings.Split(domain, ".")
	if len(parts) >= 3 && parts[len(parts)-1] == "br" {
		return strings.Join(parts[len(parts)-3:], ".")
	} else if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}

	return domain
}

// consolidateDomainPatterns consolida múltiplos padrões do mesmo domínio
func (rpt *ReferencePatternTrainer) consolidateDomainPatterns(patterns []*ReferencePattern) *ReferencePattern {
	if len(patterns) == 0 {
		return nil
	}

	consolidated := &ReferencePattern{
		Domain:     patterns[0].Domain,
		Selectors:  make(map[string][]string),
		Features:   make(map[string]interface{}),
		Examples:   []string{},
		CreatedAt:  time.Now(),
		LastTested: time.Now(),
	}

	// Consolida exemplos
	for _, pattern := range patterns {
		consolidated.Examples = append(consolidated.Examples, pattern.Examples...)
	}

	// Consolida seletores (intersecção dos que funcionam em todos)
	if len(patterns) > 0 {
		consolidated.Selectors = patterns[0].Selectors
		for _, pattern := range patterns[1:] {
			rpt.mergeSelectors(consolidated.Selectors, pattern.Selectors)
		}
	}

	// Consolida características (média)
	for _, pattern := range patterns {
		rpt.updateFeatures(consolidated.Features, pattern.Features)
	}

	// Confiança baseada no número total de exemplos
	consolidated.Confidence = rpt.calculateConfidence(len(consolidated.Examples))

	return consolidated
}

// GetLearnedPatterns retorna todos os padrões aprendidos
func (rpt *ReferencePatternTrainer) GetLearnedPatterns() map[string]*ReferencePattern {
	rpt.mutex.RLock()
	defer rpt.mutex.RUnlock()

	result := make(map[string]*ReferencePattern)
	for id, pattern := range rpt.patterns {
		result[id] = pattern
	}
	return result
}

// ExportPatterns exporta padrões para JSON
func (rpt *ReferencePatternTrainer) ExportPatterns() ([]byte, error) {
	patterns := rpt.GetLearnedPatterns()
	return json.MarshalIndent(patterns, "", "  ")
}

// ImportPatterns importa padrões de JSON
func (rpt *ReferencePatternTrainer) ImportPatterns(data []byte) error {
	rpt.mutex.Lock()
	defer rpt.mutex.Unlock()

	var patterns map[string]*ReferencePattern
	if err := json.Unmarshal(data, &patterns); err != nil {
		return fmt.Errorf("failed to unmarshal patterns: %w", err)
	}

	rpt.patterns = patterns
	rpt.logger.WithField("patterns_count", len(patterns)).Info("Reference patterns imported successfully")
	return nil
}

// MatchURL verifica se uma URL corresponde aos padrões aprendidos
func (rpt *ReferencePatternTrainer) MatchURL(rawURL string) (*ReferencePattern, float64) {
	rpt.mutex.RLock()
	defer rpt.mutex.RUnlock()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0.0
	}

	domain := parsedURL.Host
	baseDomain := rpt.getBaseDomain(domain)

	var bestMatch *ReferencePattern
	var bestScore float64

	for _, pattern := range rpt.patterns {
		score := rpt.calculateURLMatch(rawURL, domain, baseDomain, pattern)
		if score > bestScore {
			bestScore = score
			bestMatch = pattern
		}
	}

	return bestMatch, bestScore
}

// calculateURLMatch calcula score de match entre URL e padrão
func (rpt *ReferencePatternTrainer) calculateURLMatch(rawURL, domain, baseDomain string, pattern *ReferencePattern) float64 {
	score := 0.0

	// Match por domínio (peso 40%)
	patternBaseDomain := rpt.getBaseDomain(pattern.Domain)
	if baseDomain == patternBaseDomain {
		score += 0.4
	} else if strings.Contains(domain, patternBaseDomain) || strings.Contains(patternBaseDomain, baseDomain) {
		score += 0.2
	}

	// Match por padrão de URL (peso 30%)
	parsedURL, err := url.Parse(rawURL)
	if err == nil {
		path := parsedURL.Path
		if rpt.matchURLPattern(path, pattern.URLPattern) {
			score += 0.3
		}
	}

	// Confiança do padrão (peso 30%)
	score += pattern.Confidence * 0.3

	return score
}

// matchURLPattern verifica se path corresponde ao padrão
func (rpt *ReferencePatternTrainer) matchURLPattern(path, pattern string) bool {
	// Substitui {id} por regex para números
	regexPattern := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\{id\}`, `\d+`)
	matched, _ := regexp.MatchString("^"+regexPattern+"$", path)
	return matched
}
