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
	selectors := []string{
		".endereco", ".address", ".property-address", "[itemprop=address]",
		"#endereco", "#address", ".street-address", ".location",
		".imovel-endereco", ".property-location", ".address-info",
		".property-title", ".listing-address", ".imovel-titulo",
	}

	for _, selector := range selectors {
		if text := strings.TrimSpace(element.ChildText(selector)); text != "" {
			return e.cleanText(text)
		}
	}

	// Fallback: procura por padrões de endereço no texto
	fullText := element.Text
	addressRegex := regexp.MustCompile(`(?i)(R(?:ua|\.)\s+[\w\s]+,\s*\d+|Av(?:enida|\.)\s+[\w\s]+,\s*\d+)`)
	if match := addressRegex.FindString(fullText); match != "" {
		return e.cleanText(match)
	}

	return ""
}

// extractPrice extrai preço usando múltiplos seletores
func (e *DataExtractor) extractPrice(element *colly.HTMLElement) string {
	selectors := []string{
		".valor", ".price", ".property-price", "[itemprop=price]",
		"#valor", "#price", ".value", ".amount", ".property-value",
		".imovel-preco", ".valor-venda", ".valor-imovel",
	}

	for _, selector := range selectors {
		if text := strings.TrimSpace(element.ChildText(selector)); text != "" {
			return e.cleanText(text)
		}
	}

	// Fallback: procura por padrões de preço no texto
	fullText := element.Text
	priceRegex := regexp.MustCompile(`(?i)R\$\s*[\d.,]+`)
	if match := priceRegex.FindString(fullText); match != "" {
		return e.cleanText(match)
	}

	return ""
}

// extractDescription extrai descrição usando múltiplos seletores
func (e *DataExtractor) extractDescription(element *colly.HTMLElement) string {
	selectors := []string{
		".descricao", ".description", ".property-description", "[itemprop=description]",
		"#descricao", "#description", ".details", ".features", ".property-details",
		".imovel-descricao", ".property-text", ".property-info",
	}

	for _, selector := range selectors {
		if text := strings.TrimSpace(element.ChildText(selector)); text != "" {
			return e.cleanText(text)
		}
	}

	// Fallback: coleta parágrafos que parecem ser descrição
	var description strings.Builder
	element.ForEach("p", func(_ int, el *colly.HTMLElement) {
		text := strings.TrimSpace(el.Text)
		if len(text) > 30 && len(text) < 2000 { // Aumentado de 500 para 2000 caracteres
			description.WriteString(text + " ")
		}
	})

	return e.cleanText(description.String())
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
