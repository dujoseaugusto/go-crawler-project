package crawler

import (
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
)

// SelectorSet representa um conjunto de seletores para um tipo específico de dado
type SelectorSet struct {
	Primary   []string `json:"primary"`   // Seletores principais (alta prioridade)
	Secondary []string `json:"secondary"` // Seletores secundários (fallback)
	Fallback  []string `json:"fallback"`  // Seletores de último recurso
}

// DomainSelectorMap mapeia domínios para seus seletores específicos
type DomainSelectorMap map[string]map[string]*SelectorSet

// EnhancedExtractor extrator melhorado que usa padrões aprendidos das páginas de referência
type EnhancedExtractor struct {
	domainSelectors  DomainSelectorMap
	genericSelectors map[string]*SelectorSet
	referenceTrainer *ReferencePatternTrainer
	patternValidator *PatternValidator
	logger           *logger.Logger
	mutex            sync.RWMutex
	extractionStats  map[string]int // Estatísticas de sucesso por seletor
}

// NewEnhancedExtractor cria um novo extrator melhorado
func NewEnhancedExtractor(referenceTrainer *ReferencePatternTrainer, patternValidator *PatternValidator) *EnhancedExtractor {
	ee := &EnhancedExtractor{
		domainSelectors:  make(DomainSelectorMap),
		referenceTrainer: referenceTrainer,
		patternValidator: patternValidator,
		logger:           logger.NewLogger("enhanced_extractor"),
		extractionStats:  make(map[string]int),
	}

	// Inicializa seletores genéricos melhorados baseados na análise das páginas de referência
	ee.initializeGenericSelectors()

	// Carrega seletores específicos por domínio dos padrões aprendidos
	ee.loadDomainSpecificSelectors()

	return ee
}

// initializeGenericSelectors inicializa seletores genéricos melhorados
func (ee *EnhancedExtractor) initializeGenericSelectors() {
	ee.genericSelectors = map[string]*SelectorSet{
		"price": {
			Primary: []string{
				".preco-imovel", ".valor-imovel", ".property-price", ".listing-price",
				".price-value", ".valor-venda", ".preco-venda", ".imovel-preco",
			},
			Secondary: []string{
				".preco", ".price", ".valor", ".value", "[class*='preco']",
				"[class*='price']", "[class*='valor']", ".currency", ".amount",
			},
			Fallback: []string{
				"*[class*='R$']", "span:contains('R$')", "div:contains('R$')",
				"p:contains('R$')", ".money", ".cost",
			},
		},
		"address": {
			Primary: []string{
				".endereco-imovel", ".property-address", ".listing-address", ".imovel-endereco",
				".localizacao-imovel", ".property-location", ".address-info",
			},
			Secondary: []string{
				".endereco", ".address", ".localizacao", ".location", "[class*='endereco']",
				"[class*='address']", "[class*='localizacao']", ".street-address",
			},
			Fallback: []string{
				".locality", ".region", ".postal-code", "[itemprop='address']",
				"*[class*='rua']", "*[class*='avenida']", "*[class*='bairro']",
			},
		},
		"description": {
			Primary: []string{
				".descricao-imovel", ".property-description", ".listing-description",
				".imovel-descricao", ".property-details", ".detalhes-imovel",
			},
			Secondary: []string{
				".descricao", ".description", ".detalhes", ".details", "[class*='descricao']",
				"[class*='description']", "[class*='detalhes']", ".property-info",
			},
			Fallback: []string{
				".content", ".text", ".info", "[itemprop='description']",
				".summary", ".overview", ".about",
			},
		},
		"rooms": {
			Primary: []string{
				".quartos-imovel", ".dormitorios-imovel", ".property-bedrooms",
				".imovel-quartos", ".bedrooms-count", ".rooms-count",
			},
			Secondary: []string{
				".quartos", ".dormitorios", ".bedrooms", ".rooms", "[class*='quartos']",
				"[class*='dormitorios']", "[class*='bedrooms']", ".bed-count",
			},
			Fallback: []string{
				"*:contains('quartos')", "*:contains('dormitórios')", "*:contains('bedrooms')",
				".specifications li", ".features li", ".amenities li",
			},
		},
		"bathrooms": {
			Primary: []string{
				".banheiros-imovel", ".property-bathrooms", ".imovel-banheiros",
				".bathrooms-count", ".wc-count",
			},
			Secondary: []string{
				".banheiros", ".bathrooms", ".wc", "[class*='banheiros']",
				"[class*='bathrooms']", ".bath-count",
			},
			Fallback: []string{
				"*:contains('banheiros')", "*:contains('bathrooms')", "*:contains('wc')",
				".specifications li", ".features li",
			},
		},
		"area": {
			Primary: []string{
				".area-imovel", ".metragem-imovel", ".property-area", ".imovel-area",
				".area-total", ".area-construida", ".built-area",
			},
			Secondary: []string{
				".area", ".metragem", ".size", ".m2", "[class*='area']",
				"[class*='metragem']", "[class*='size']", ".square-meters",
			},
			Fallback: []string{
				"*:contains('m²')", "*:contains('m2')", "*:contains('metros')",
				".specifications li", ".features li",
			},
		},
		"features": {
			Primary: []string{
				".caracteristicas-imovel", ".property-features", ".imovel-caracteristicas",
				".property-amenities", ".listing-features", ".amenities-list",
			},
			Secondary: []string{
				".caracteristicas", ".features", ".amenities", ".comodidades",
				"[class*='caracteristicas']", "[class*='features']", ".specifications",
			},
			Fallback: []string{
				".extras", ".includes", ".highlights", ".benefits",
				"ul li", ".list-item", ".feature-item",
			},
		},
		"type": {
			Primary: []string{
				".tipo-imovel", ".property-type", ".imovel-tipo", ".listing-type",
				".property-category", ".category",
			},
			Secondary: []string{
				".tipo", ".type", "[class*='tipo']", "[class*='type']",
				".category-name", ".property-kind",
			},
			Fallback: []string{
				"h1", "h2", ".title", ".heading", ".property-title",
				".listing-title", "[itemprop='name']",
			},
		},
		"contact": {
			Primary: []string{
				".contato-imovel", ".property-contact", ".imovel-contato",
				".contact-info", ".agent-info", ".broker-info",
			},
			Secondary: []string{
				".contato", ".contact", ".telefone", ".phone", "[class*='contato']",
				"[class*='contact']", "[class*='telefone']", ".whatsapp",
			},
			Fallback: []string{
				".agent", ".broker", ".seller", ".owner",
				"*:contains('telefone')", "*:contains('whatsapp')",
			},
		},
	}
}

// loadDomainSpecificSelectors carrega seletores específicos por domínio dos padrões aprendidos
func (ee *EnhancedExtractor) loadDomainSpecificSelectors() {
	if ee.referenceTrainer == nil {
		return
	}

	patterns := ee.referenceTrainer.GetLearnedPatterns()

	for _, pattern := range patterns {
		domain := pattern.Domain
		if ee.domainSelectors[domain] == nil {
			ee.domainSelectors[domain] = make(map[string]*SelectorSet)
		}

		// Converte seletores do padrão para SelectorSet
		for dataType, selectors := range pattern.Selectors {
			if len(selectors) > 0 {
				selectorSet := &SelectorSet{
					Primary: selectors[:min(len(selectors), 3)], // Primeiros 3 como principais
				}

				if len(selectors) > 3 {
					selectorSet.Secondary = selectors[3:min(len(selectors), 6)] // Próximos 3 como secundários
				}

				if len(selectors) > 6 {
					selectorSet.Fallback = selectors[6:] // Resto como fallback
				}

				ee.domainSelectors[domain][dataType] = selectorSet
			}
		}
	}

	ee.logger.WithField("domains_loaded", len(ee.domainSelectors)).Info("Domain-specific selectors loaded")
}

// ExtractPropertyData extrai dados de propriedade usando seletores melhorados
func (ee *EnhancedExtractor) ExtractPropertyData(e *colly.HTMLElement, url string) repository.Property {
	domain := e.Request.URL.Host

	ee.logger.WithFields(map[string]interface{}{
		"url":    url,
		"domain": domain,
	}).Debug("Extracting property data with enhanced selectors")

	property := repository.Property{
		URL: url,
	}

	// Extrai cada tipo de dado usando seletores apropriados
	property.ValorTexto = ee.extractWithSelectors(e, domain, "price")
	property.Valor = extractValue(property.ValorTexto)

	property.Endereco = ee.extractWithSelectors(e, domain, "address")
	property.Descricao = ee.extractWithSelectors(e, domain, "description")

	// Extrai informações numéricas
	roomsText := ee.extractWithSelectors(e, domain, "rooms")
	property.Quartos = ee.extractNumericValue(roomsText, "quartos")

	bathroomsText := ee.extractWithSelectors(e, domain, "bathrooms")
	property.Banheiros = ee.extractNumericValue(bathroomsText, "banheiros")

	areaText := ee.extractWithSelectors(e, domain, "area")
	property.AreaTotal = ee.extractAreaValue(areaText)

	// Extrai tipo do imóvel
	typeText := ee.extractWithSelectors(e, domain, "type")
	if typeText == "" {
		typeText = property.Descricao + " " + property.Endereco
	}
	property.TipoImovel = extractPropertyType(typeText)

	// Extrai características
	featuresText := ee.extractWithSelectors(e, domain, "features")
	property.Caracteristicas = ee.extractFeaturesList(e, featuresText)

	// Extrai informações de localização
	property.Cidade, property.Bairro = ee.extractLocationInfo(property.Endereco, e.Text)

	// Extrai CEP se disponível
	property.CEP = extractCEP(property.Endereco + " " + e.Text)

	// Limpa e valida dados extraídos
	ee.cleanAndValidateProperty(&property)

	ee.logger.WithFields(map[string]interface{}{
		"url":             url,
		"has_price":       property.Valor > 0,
		"has_address":     property.Endereco != "",
		"has_description": property.Descricao != "",
		"property_type":   property.TipoImovel,
	}).Debug("Property data extraction completed")

	return property
}

// extractWithSelectors extrai conteúdo usando hierarquia de seletores
func (ee *EnhancedExtractor) extractWithSelectors(e *colly.HTMLElement, domain, dataType string) string {
	// 1. Tenta seletores específicos do domínio primeiro
	if domainSelectors, exists := ee.domainSelectors[domain]; exists {
		if selectorSet, exists := domainSelectors[dataType]; exists {
			if content := ee.trySelectorsInOrder(e, selectorSet, dataType); content != "" {
				ee.recordSelectorSuccess(domain, dataType, "domain_specific")
				return content
			}
		}
	}

	// 2. Usa seletores genéricos
	if selectorSet, exists := ee.genericSelectors[dataType]; exists {
		if content := ee.trySelectorsInOrder(e, selectorSet, dataType); content != "" {
			ee.recordSelectorSuccess("generic", dataType, "generic")
			return content
		}
	}

	// 3. Fallback para extração por regex no texto completo
	return ee.extractByRegex(e.Text, dataType)
}

// trySelectorsInOrder tenta seletores em ordem de prioridade
func (ee *EnhancedExtractor) trySelectorsInOrder(e *colly.HTMLElement, selectorSet *SelectorSet, dataType string) string {
	// Tenta seletores primários
	for _, selector := range selectorSet.Primary {
		if content := ee.extractAndValidate(e, selector, dataType); content != "" {
			return content
		}
	}

	// Tenta seletores secundários
	for _, selector := range selectorSet.Secondary {
		if content := ee.extractAndValidate(e, selector, dataType); content != "" {
			return content
		}
	}

	// Tenta seletores de fallback
	for _, selector := range selectorSet.Fallback {
		if content := ee.extractAndValidate(e, selector, dataType); content != "" {
			return content
		}
	}

	return ""
}

// extractAndValidate extrai conteúdo e valida se é apropriado para o tipo de dado
func (ee *EnhancedExtractor) extractAndValidate(e *colly.HTMLElement, selector, dataType string) string {
	content := strings.TrimSpace(e.ChildText(selector))
	if content == "" {
		return ""
	}

	// Valida se o conteúdo é apropriado para o tipo de dado
	if ee.isValidContentForType(content, dataType) {
		return content
	}

	return ""
}

// isValidContentForType verifica se o conteúdo é válido para o tipo de dado
func (ee *EnhancedExtractor) isValidContentForType(content, dataType string) bool {
	content = strings.ToLower(content)

	switch dataType {
	case "price":
		return strings.Contains(content, "r$") ||
			regexp.MustCompile(`\d+[.,]\d+`).MatchString(content)
	case "address":
		return len(content) > 10 && len(content) < 300 &&
			!strings.Contains(content, "javascript") &&
			!strings.Contains(content, "cookie") &&
			!strings.Contains(content, "todos os direitos")
	case "description":
		return len(content) > 20 && len(content) < 2000 &&
			!strings.Contains(content, "javascript") &&
			!strings.Contains(content, "function")
	case "rooms", "bathrooms":
		return len(content) < 50 &&
			(regexp.MustCompile(`\d+`).MatchString(content) ||
				strings.Contains(content, "quarto") ||
				strings.Contains(content, "banheiro"))
	case "area":
		return len(content) < 100 &&
			(strings.Contains(content, "m²") ||
				strings.Contains(content, "m2") ||
				regexp.MustCompile(`\d+\s*m`).MatchString(content))
	case "features":
		return len(content) > 5 && len(content) < 1000 &&
			!strings.Contains(content, "javascript")
	default:
		return len(content) > 3 && len(content) < 500
	}
}

// extractByRegex extrai dados usando regex como último recurso
func (ee *EnhancedExtractor) extractByRegex(text, dataType string) string {
	switch dataType {
	case "price":
		re := regexp.MustCompile(`R\$\s*[\d.,]+`)
		if match := re.FindString(text); match != "" {
			return match
		}
	case "address":
		re := regexp.MustCompile(`(?i)(R(?:ua|\.)\s+[^,\n]+|Av(?:enida|\.)\s+[^,\n]+|VILA\s+[^,\n]+|JARDIM\s+[^,\n]+)`)
		if match := re.FindString(text); match != "" {
			return strings.TrimSpace(match)
		}
	case "rooms":
		re := regexp.MustCompile(`(\d+)\s*(?:quartos?|dormitórios?)`)
		if match := re.FindStringSubmatch(text); len(match) > 1 {
			return match[0]
		}
	case "bathrooms":
		re := regexp.MustCompile(`(\d+)\s*(?:banheiros?)`)
		if match := re.FindStringSubmatch(text); len(match) > 1 {
			return match[0]
		}
	case "area":
		re := regexp.MustCompile(`(\d+(?:[,.]\d+)?)\s*m[²2]`)
		if match := re.FindStringSubmatch(text); len(match) > 1 {
			return match[0]
		}
	}

	return ""
}

// extractNumericValue extrai valor numérico de texto
func (ee *EnhancedExtractor) extractNumericValue(text, context string) int {
	if text == "" {
		return 0
	}

	// Procura por números no texto
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindAllString(text, -1)

	if len(matches) > 0 {
		if value, err := strconv.Atoi(matches[0]); err == nil {
			// Valida se o valor é razoável para o contexto
			if context == "quartos" && value > 0 && value <= 20 {
				return value
			} else if context == "banheiros" && value > 0 && value <= 15 {
				return value
			}
		}
	}

	return 0
}

// extractAreaValue extrai valor de área
func (ee *EnhancedExtractor) extractAreaValue(text string) float64 {
	if text == "" {
		return 0
	}

	re := regexp.MustCompile(`(\d+(?:[,.]\d+)?)\s*m[²2]`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		areaStr := strings.Replace(match[1], ",", ".", 1)
		if area, err := strconv.ParseFloat(areaStr, 64); err == nil {
			// Valida se a área é razoável (entre 10 e 10000 m²)
			if area >= 10 && area <= 10000 {
				return area
			}
		}
	}

	return 0
}

// extractFeaturesList extrai lista de características
func (ee *EnhancedExtractor) extractFeaturesList(e *colly.HTMLElement, featuresText string) []string {
	var features []string

	// Tenta extrair de listas estruturadas primeiro
	e.ForEach(".caracteristicas li, .features li, .amenities li, .specifications li", func(_ int, el *colly.HTMLElement) {
		feature := strings.TrimSpace(el.Text)
		if ee.isValidFeature(feature) {
			features = append(features, feature)
		}
	})

	// Se não encontrou em listas, processa texto das características
	if len(features) == 0 && featuresText != "" {
		lines := strings.Split(featuresText, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if ee.isValidFeature(line) {
				features = append(features, line)
			}
		}
	}

	// Remove duplicatas e limita quantidade
	return ee.cleanFeaturesList(features)
}

// isValidFeature verifica se uma característica é válida
func (ee *EnhancedExtractor) isValidFeature(feature string) bool {
	feature = strings.TrimSpace(feature)
	featureUpper := strings.ToUpper(feature)

	// Lista de características inválidas
	invalidFeatures := map[string]bool{
		"INÍCIO": true, "INICIO": true, "SOBRE NÓS": true, "CONTATO": true,
		"IMÓVEIS URBANOS": true, "IMÓVEIS RURAIS": true, "WHATSAPP": true,
		"E-MAIL": true, "FACEBOOK": true, "JAVASCRIPT": true, "FUNCTION": true,
	}

	return len(feature) > 3 && len(feature) < 100 &&
		!invalidFeatures[featureUpper] &&
		!strings.Contains(featureUpper, "HTTP") &&
		!strings.Contains(featureUpper, "WWW.") &&
		!strings.Contains(featureUpper, "R$")
}

// cleanFeaturesList limpa e organiza lista de características
func (ee *EnhancedExtractor) cleanFeaturesList(features []string) []string {
	seen := make(map[string]bool)
	var cleaned []string

	for _, feature := range features {
		feature = strings.TrimSpace(feature)
		featureUpper := strings.ToUpper(feature)

		if !seen[featureUpper] && len(cleaned) < 20 { // Limita a 20 características
			seen[featureUpper] = true
			cleaned = append(cleaned, feature)
		}
	}

	return cleaned
}

// extractLocationInfo extrai informações de localização melhoradas
func (ee *EnhancedExtractor) extractLocationInfo(endereco, fullText string) (cidade, bairro string) {
	// Usa função existente como base
	cidade, bairro = extractLocationInfo(endereco)

	// Se não encontrou cidade no endereço, procura no texto completo
	if cidade == "" {
		cidade = extractCityFromText(fullText)
	}

	// Se não encontrou bairro no endereço, procura no texto completo
	if bairro == "" {
		bairro = extractNeighborhoodFromText(fullText)
	}

	return cidade, bairro
}

// cleanAndValidateProperty limpa e valida dados da propriedade
func (ee *EnhancedExtractor) cleanAndValidateProperty(property *repository.Property) {
	// Limpa textos
	property.Endereco = cleanText(property.Endereco)
	property.Descricao = cleanText(property.Descricao)
	property.ValorTexto = cleanText(property.ValorTexto)
	property.Cidade = cleanText(property.Cidade)
	property.Bairro = cleanText(property.Bairro)

	// Valida e corrige valores numéricos
	if property.Valor < 0 {
		property.Valor = 0
	}
	if property.Quartos < 0 || property.Quartos > 20 {
		property.Quartos = 0
	}
	if property.Banheiros < 0 || property.Banheiros > 15 {
		property.Banheiros = 0
	}
	if property.AreaTotal < 0 || property.AreaTotal > 10000 {
		property.AreaTotal = 0
	}

	// Define tipo padrão se não foi determinado
	if property.TipoImovel == "" {
		property.TipoImovel = "Outro"
	}
}

// recordSelectorSuccess registra sucesso de um seletor para estatísticas
func (ee *EnhancedExtractor) recordSelectorSuccess(domain, dataType, selectorType string) {
	ee.mutex.Lock()
	defer ee.mutex.Unlock()

	key := domain + "_" + dataType + "_" + selectorType
	ee.extractionStats[key]++
}

// GetExtractionStats retorna estatísticas de extração
func (ee *EnhancedExtractor) GetExtractionStats() map[string]int {
	ee.mutex.RLock()
	defer ee.mutex.RUnlock()

	stats := make(map[string]int)
	for key, count := range ee.extractionStats {
		stats[key] = count
	}
	return stats
}

// UpdateDomainSelectors atualiza seletores para um domínio específico
func (ee *EnhancedExtractor) UpdateDomainSelectors(domain string, selectors map[string]*SelectorSet) {
	ee.mutex.Lock()
	defer ee.mutex.Unlock()

	if ee.domainSelectors[domain] == nil {
		ee.domainSelectors[domain] = make(map[string]*SelectorSet)
	}

	for dataType, selectorSet := range selectors {
		ee.domainSelectors[domain][dataType] = selectorSet
	}

	ee.logger.WithFields(map[string]interface{}{
		"domain":     domain,
		"data_types": len(selectors),
	}).Info("Domain selectors updated")
}

// Função auxiliar min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
