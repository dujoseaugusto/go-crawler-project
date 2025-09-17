package crawler

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
)

// DataExtractor é responsável por extrair dados de elementos HTML
type DataExtractor struct {
	logger *logger.Logger
}

// NewDataExtractor cria um novo extrator de dados
func NewDataExtractor() *DataExtractor {
	return &DataExtractor{
		logger: logger.NewLogger("data_extractor"),
	}
}

// ExtractProperty extrai dados de propriedade de um elemento HTML
func (e *DataExtractor) ExtractProperty(element *colly.HTMLElement, url string) *repository.Property {
	e.logger.WithField("url", url).Debug("Extracting property data")

	property := &repository.Property{
		URL: url,
	}

	// Extrai endereço
	property.Endereco = e.extractAddress(element)

	// Extrai valor
	property.ValorTexto = e.extractPrice(element)
	property.Valor = e.extractNumericValue(property.ValorTexto)

	// Extrai descrição
	property.Descricao = e.extractDescription(element)

	// Extrai informações numéricas da descrição
	property.Quartos = e.extractRooms(property.Descricao)
	property.Banheiros = e.extractBathrooms(property.Descricao)
	property.AreaTotal = e.extractArea(property.Descricao)

	// Extrai tipo de imóvel
	property.TipoImovel = e.extractPropertyType(property.Descricao + " " + property.Endereco)

	// Extrai localização
	property.Cidade, property.Bairro = e.extractLocation(property.Endereco + " " + property.Descricao)

	// Extrai CEP
	property.CEP = e.extractCEP(property.Endereco)

	// Extrai características
	property.Caracteristicas = e.extractFeatures(element, property.Descricao)

	e.logger.WithFields(map[string]interface{}{
		"endereco": property.Endereco,
		"valor":    property.Valor,
		"tipo":     property.TipoImovel,
	}).Debug("Property data extracted")

	return property
}

// extractAddress extrai endereço usando múltiplos seletores
func (e *DataExtractor) extractAddress(element *colly.HTMLElement) string {
	// Seletores mais abrangentes e específicos para sites brasileiros
	selectors := []string{
		".endereco", ".address", ".property-address", "[itemprop=address]",
		"#endereco", "#address", ".street-address", ".location",
		".imovel-endereco", ".property-location", ".address-info",
		".property-title", ".listing-address", ".imovel-titulo",
		".endereco-imovel", ".local", ".localizacao", ".onde",
		"h1", "h2", "h3", // Títulos podem conter endereços
		".titulo", ".title", ".nome-imovel", ".property-name",
		".card-title", ".item-title", ".listing-title",
		"[data-address]", "[data-location]", "[data-endereco]",
	}

	for _, selector := range selectors {
		if text := strings.TrimSpace(element.ChildText(selector)); text != "" {
			// Verifica se parece com endereço
			if e.looksLikeAddress(text) {
				return e.cleanText(text)
			}
		}
	}

	// Busca em meta tags
	metaSelectors := []string{
		"meta[property='og:title']",
		"meta[name='description']",
		"meta[property='og:description']",
	}
	
	for _, selector := range metaSelectors {
		element.ForEach(selector, func(_ int, el *colly.HTMLElement) {
			if content := el.Attr("content"); content != "" {
				if e.looksLikeAddress(content) {
					return
				}
			}
		})
	}

	// Fallback: procura por padrões de endereço no texto completo
	fullText := element.Text
	
	// Padrões mais abrangentes para endereços brasileiros
	addressPatterns := []string{
		`(?i)(R(?:ua|\.)\s+[^,\n]+,\s*\d+)`,
		`(?i)(Av(?:enida|\.)\s+[^,\n]+,\s*\d+)`,
		`(?i)(Pra(?:ça|\.)\s+[^,\n]+,\s*\d+)`,
		`(?i)(Est(?:rada|\.)\s+[^,\n]+,\s*\d+)`,
		`(?i)(Rod(?:ovia|\.)\s+[^,\n]+,\s*\d+)`,
		`(?i)([A-Z][^,\n]+ - [A-Z]{2})`, // Cidade - Estado
	}
	
	for _, pattern := range addressPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(fullText); match != "" {
			return e.cleanText(match)
		}
	}

	return ""
}

// looksLikeAddress verifica se um texto parece ser um endereço
func (e *DataExtractor) looksLikeAddress(text string) bool {
	if len(text) < 5 || len(text) > 200 {
		return false
	}
	
	text = strings.ToLower(text)
	
	// Palavras que indicam endereço
	addressWords := []string{
		"rua", "avenida", "praça", "estrada", "rodovia", "alameda",
		"travessa", "largo", "beco", "vila", "jardim", "centro",
		"bairro", "distrito", "setor", "quadra", "lote",
	}
	
	// Padrões que indicam endereço
	addressPatterns := []string{
		`\d+`, // Números
		`-\s*mg|minas\s+gerais`, // Estado
		`cep\s*:?\s*\d`, // CEP
	}
	
	wordCount := 0
	for _, word := range addressWords {
		if strings.Contains(text, word) {
			wordCount++
		}
	}
	
	patternCount := 0
	for _, pattern := range addressPatterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			patternCount++
		}
	}
	
	// Considera endereço se tem pelo menos 1 palavra indicativa OU 1 padrão
	return wordCount >= 1 || patternCount >= 1
}

// extractPrice extrai preço usando múltiplos seletores
func (e *DataExtractor) extractPrice(element *colly.HTMLElement) string {
	// Seletores mais abrangentes para preços
	selectors := []string{
		".valor", ".price", ".property-price", "[itemprop=price]",
		"#valor", "#price", ".value", ".amount", ".property-value",
		".imovel-preco", ".valor-venda", ".valor-imovel",
		".preco", ".preco-venda", ".valor-total", ".price-value",
		".money", ".currency", ".cost", ".sale-price",
		"[data-price]", "[data-valor]", "[data-preco]",
		".price-container", ".valor-container", ".preco-container",
		"strong", "b", // Preços geralmente estão em negrito
	}

	for _, selector := range selectors {
		if text := strings.TrimSpace(element.ChildText(selector)); text != "" {
			if e.looksLikePrice(text) {
				return e.cleanText(text)
			}
		}
	}

	// Busca em atributos específicos
	priceAttributes := []string{"data-price", "data-valor", "data-preco", "value"}
	for _, attr := range priceAttributes {
		element.ForEach("*[" + attr + "]", func(_ int, el *colly.HTMLElement) {
			if value := el.Attr(attr); value != "" && e.looksLikePrice(value) {
				// Encontrou preço em atributo, mas continua procurando
			}
		})
	}

	// Fallback: procura por padrões de preço no texto completo
	fullText := element.Text
	
	// Padrões mais específicos para valores brasileiros
	pricePatterns := []string{
		`(?i)R\$\s*[\d.,]+(?:\s*mil)?(?:\s*milhões?)?`,
		`(?i)(?:valor|preço|por)\s*:?\s*R\$\s*[\d.,]+`,
		`(?i)R\$\s*\d{1,3}(?:\.\d{3})*(?:,\d{2})?`,
		`(?i)\d{1,3}(?:\.\d{3})*(?:,\d{2})?\s*reais?`,
	}
	
	for _, pattern := range pricePatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(fullText); match != "" {
			return e.cleanText(match)
		}
	}

	return ""
}

// looksLikePrice verifica se um texto parece ser um preço
func (e *DataExtractor) looksLikePrice(text string) bool {
	if len(text) < 2 || len(text) > 50 {
		return false
	}
	
	text = strings.ToLower(text)
	
	// Padrões que indicam preço
	priceIndicators := []string{
		"r$", "real", "reais", "mil", "milhão", "milhões",
		"valor", "preço", "preco", "venda", "à vista",
	}
	
	// Deve conter números
	hasNumbers := regexp.MustCompile(`\d`).MatchString(text)
	if !hasNumbers {
		return false
	}
	
	// Deve conter pelo menos um indicador de preço
	for _, indicator := range priceIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}
	
	// Ou seguir padrão de valor monetário
	monetaryPattern := regexp.MustCompile(`^\s*R?\$?\s*\d`)
	return monetaryPattern.MatchString(text)
}

// extractDescription extrai descrição usando múltiplos seletores
func (e *DataExtractor) extractDescription(element *colly.HTMLElement) string {
	// Seletores mais abrangentes para descrições
	selectors := []string{
		".descricao", ".description", ".property-description", "[itemprop=description]",
		"#descricao", "#description", ".details", ".features", ".property-details",
		".imovel-descricao", ".property-text", ".property-info",
		".content", ".texto", ".info", ".informacoes", ".detalhes",
		".property-content", ".listing-content", ".item-content",
		".resumo", ".summary", ".sobre", ".about",
		"[data-description]", "[data-descricao]", "[data-content]",
		".card-body", ".item-body", ".property-body",
	}

	// Primeiro tenta seletores específicos
	for _, selector := range selectors {
		if text := strings.TrimSpace(element.ChildText(selector)); text != "" {
			if e.looksLikeDescription(text) {
				return e.cleanText(text)
			}
		}
	}

	// Busca em meta tags
	metaDescription := element.ChildAttr("meta[name='description']", "content")
	if metaDescription != "" && e.looksLikeDescription(metaDescription) {
		return e.cleanText(metaDescription)
	}

	// Fallback: coleta parágrafos e divs que parecem ser descrição
	var description strings.Builder
	
	// Primeiro tenta parágrafos
	element.ForEach("p", func(_ int, el *colly.HTMLElement) {
		text := strings.TrimSpace(el.Text)
		if e.looksLikeDescription(text) {
			description.WriteString(text + " ")
		}
	})

	// Se não encontrou descrição suficiente, tenta divs
	if description.Len() < 50 {
		element.ForEach("div", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if e.looksLikeDescription(text) && len(text) > 30 && len(text) < 1000 {
				description.WriteString(text + " ")
			}
		})
	}

	// Se ainda não tem descrição, pega texto de elementos com mais conteúdo
	if description.Len() < 30 {
		element.ForEach("span, td, li", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if e.looksLikeDescription(text) && len(text) > 20 && len(text) < 500 {
				description.WriteString(text + " ")
			}
		})
	}

	result := e.cleanText(description.String())
	
	// Limita o tamanho da descrição
	if len(result) > 3000 {
		result = result[:3000] + "..."
	}

	return result
}

// looksLikeDescription verifica se um texto parece ser uma descrição válida
func (e *DataExtractor) looksLikeDescription(text string) bool {
	if len(text) < 15 || len(text) > 5000 {
		return false
	}
	
	text = strings.ToLower(text)
	
	// Palavras que indicam descrição de imóvel
	descriptiveWords := []string{
		"casa", "apartamento", "imóvel", "propriedade", "residência",
		"quartos", "dormitórios", "banheiros", "sala", "cozinha",
		"garagem", "jardim", "área", "localização", "próximo",
		"amplo", "espaçoso", "confortável", "moderno", "reformado",
		"venda", "aluguel", "locação", "oportunidade",
	}
	
	// Padrões que NÃO devem estar em descrições
	excludePatterns := []string{
		"copyright", "todos os direitos", "política de privacidade",
		"termos de uso", "desenvolvido por", "powered by",
		"javascript", "css", "html", "error", "404",
		"menu", "navegação", "footer", "header", "sidebar",
	}
	
	// Verifica se contém palavras excludentes
	for _, pattern := range excludePatterns {
		if strings.Contains(text, pattern) {
			return false
		}
	}
	
	// Conta palavras descritivas
	wordCount := 0
	for _, word := range descriptiveWords {
		if strings.Contains(text, word) {
			wordCount++
		}
	}
	
	// Verifica se tem estrutura de frase (espaços e pontuação)
	hasSpaces := strings.Contains(text, " ")
	hasPunctuation := regexp.MustCompile(`[.,;:!?]`).MatchString(text)
	
	// Considera descrição se tem pelo menos 2 palavras descritivas OU estrutura de frase
	return (wordCount >= 2) || (hasSpaces && hasPunctuation && wordCount >= 1)
}

// extractFeatures extrai características do imóvel
func (e *DataExtractor) extractFeatures(element *colly.HTMLElement, description string) []string {
	var features []string

	// Lista de características comuns
	commonFeatures := []string{
		"piscina", "churrasqueira", "garagem", "jardim", "varanda",
		"sacada", "área gourmet", "playground", "portaria", "elevador",
	}

	descLower := strings.ToLower(description)
	for _, feature := range commonFeatures {
		if strings.Contains(descLower, feature) {
			features = append(features, feature)
		}
	}

	// Extrai de elementos específicos
	element.ForEach(".caracteristicas li, .amenities li, .features li", func(_ int, el *colly.HTMLElement) {
		feature := strings.TrimSpace(el.Text)
		if len(feature) > 3 && len(feature) < 50 {
			features = append(features, feature)
		}
	})

	return features
}

// extractNumericValue extrai valor numérico de uma string
func (e *DataExtractor) extractNumericValue(valueStr string) float64 {
	if valueStr == "" {
		return 0
	}

	// Remove caracteres não numéricos exceto ponto e vírgula
	re := regexp.MustCompile(`[^\d,.]`)
	cleaned := re.ReplaceAllString(valueStr, "")

	// Procura por padrões de valores monetários
	reValue := regexp.MustCompile(`(\d{1,3}(?:\.\d{3})*(?:,\d{1,2})?)|\d+`)
	matches := reValue.FindString(cleaned)

	if matches == "" {
		return 0
	}

	// Converte vírgula para ponto (padrão brasileiro para internacional)
	matches = strings.Replace(matches, ".", "", -1)
	matches = strings.Replace(matches, ",", ".", 1)

	value, err := strconv.ParseFloat(matches, 64)
	if err != nil {
		return 0
	}

	return value
}

// extractRooms extrai número de quartos
func (e *DataExtractor) extractRooms(text string) int {
	re := regexp.MustCompile(`(\d+)\s*(?:quartos?|dormitórios?|dorms?|suítes?)`)
	match := re.FindStringSubmatch(strings.ToLower(text))
	if len(match) > 1 {
		if rooms, err := strconv.Atoi(match[1]); err == nil {
			return rooms
		}
	}
	return 0
}

// extractBathrooms extrai número de banheiros
func (e *DataExtractor) extractBathrooms(text string) int {
	re := regexp.MustCompile(`(\d+)\s*(?:banheiros?|lavabos?)`)
	match := re.FindStringSubmatch(strings.ToLower(text))
	if len(match) > 1 {
		if bathrooms, err := strconv.Atoi(match[1]); err == nil {
			return bathrooms
		}
	}
	return 0
}

// extractArea extrai área em m²
func (e *DataExtractor) extractArea(text string) float64 {
	re := regexp.MustCompile(`(\d+(?:[,.]\d+)?)\s*m²`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		areaStr := strings.Replace(match[1], ",", ".", 1)
		if area, err := strconv.ParseFloat(areaStr, 64); err == nil {
			return area
		}
	}
	return 0
}

// extractCEP extrai CEP
func (e *DataExtractor) extractCEP(text string) string {
	re := regexp.MustCompile(`\d{5}-?\d{3}`)
	return re.FindString(text)
}

// extractPropertyType extrai tipo de imóvel
func (e *DataExtractor) extractPropertyType(text string) string {
	textLower := strings.ToLower(text)

	if strings.Contains(textLower, "casa") {
		return "Casa"
	} else if strings.Contains(textLower, "apartamento") || strings.Contains(textLower, "apto") {
		return "Apartamento"
	} else if strings.Contains(textLower, "terreno") || strings.Contains(textLower, "lote") {
		return "Terreno"
	} else if strings.Contains(textLower, "comercial") || strings.Contains(textLower, "sala") {
		return "Comercial"
	} else if strings.Contains(textLower, "rural") || strings.Contains(textLower, "fazenda") {
		return "Rural"
	}

	return "Outro"
}

// extractLocation extrai cidade e bairro
func (e *DataExtractor) extractLocation(text string) (cidade, bairro string) {
	// Lista de cidades conhecidas
	cities := []string{
		"MUZAMBINHO", "ALFENAS", "GUAXUPÉ", "JURUAIA", "MONTE BELO",
		"POÇOS DE CALDAS", "VARGINHA", "TRÊS PONTAS", "MACHADO",
	}

	textUpper := strings.ToUpper(text)

	// Procura por cidades
	for _, city := range cities {
		if strings.Contains(textUpper, city) {
			cidade = strings.Title(strings.ToLower(city))
			break
		}
	}

	// Tenta extrair bairro usando padrões
	re := regexp.MustCompile(`([^,]+),\s*([^,]+)(?:,\s*[^,]+)?$`)
	matches := re.FindStringSubmatch(text)

	if len(matches) >= 3 {
		possibleBairro := strings.TrimSpace(matches[1])
		possibleCidade := strings.TrimSpace(matches[2])

		// Se não encontrou cidade ainda, usa a do padrão
		if cidade == "" {
			cidade = possibleCidade
		}
		bairro = possibleBairro
	}

	return
}

// cleanText limpa e normaliza texto
func (e *DataExtractor) cleanText(text string) string {
	// Remove espaços extras
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	// Remove quebras de linha
	text = strings.ReplaceAll(text, "\n", " ")

	return strings.TrimSpace(text)
}
