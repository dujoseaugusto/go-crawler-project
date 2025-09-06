package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

type Crawler struct {
	config    *config.Config
	repo      repository.PropertyRepository
	aiService *ai.GeminiService
}

func NewCrawler(ctx context.Context, cfg *config.Config) *Crawler {
	repo, err := repository.NewMongoRepository(cfg.MongoURI, "crawler", "properties")
	if err != nil {
		log.Fatalf("Failed to create MongoDB repository: %v", err)
	}

	// Inicializar o serviço de IA
	aiService, err := ai.NewGeminiService(ctx)
	if err != nil {
		log.Printf("Warning: Failed to create Gemini service: %v", err)
		// Continua sem o serviço de IA se houver erro
		return &Crawler{
			config: cfg,
			repo:   repo,
		}
	}

	return &Crawler{
		config:    cfg,
		repo:      repo,
		aiService: aiService,
	}
}

func (c *Crawler) StartCrawling(ctx context.Context) error {
	urls, err := LoadURLsFromFile(c.config.SitesFile)
	if err != nil {
		return fmt.Errorf("failed to load URLs: %v", err)
	}

	StartCrawling(ctx, c.repo, urls, c.aiService)
	return nil
}

// Extrai valor numérico de uma string (ex: "R$ 500.000,00" -> 500000.00)
func extractValue(valorStr string) float64 {
	// Remove caracteres não numéricos exceto ponto e vírgula
	re := regexp.MustCompile(`[^\d,.]`)
	cleaned := re.ReplaceAllString(valorStr, "")

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

// Extrai CEP de uma string
func extractCEP(str string) string {
	re := regexp.MustCompile(`\d{5}-?\d{3}`)
	match := re.FindString(str)
	return match
}

// Extrai número de quartos de uma string
func extractRooms(str string) int {
	re := regexp.MustCompile(`(\d+)\s*(?:quartos?|dormitórios?|dorms?|suítes?)`)
	match := re.FindStringSubmatch(str)
	if len(match) > 1 {
		rooms, err := strconv.Atoi(match[1])
		if err == nil {
			return rooms
		}
	}
	return 0
}

// Extrai número de banheiros de uma string
func extractBathrooms(str string) int {
	re := regexp.MustCompile(`(\d+)\s*(?:banheiros?|lavabos?)`)
	match := re.FindStringSubmatch(str)
	if len(match) > 1 {
		bathrooms, err := strconv.Atoi(match[1])
		if err == nil {
			return bathrooms
		}
	}
	return 0
}

// Extrai área de uma string (ex: "150m²" -> 150.0)
func extractArea(str string) float64 {
	re := regexp.MustCompile(`(\d+(?:[,.]\d+)?)\s*m²`)
	match := re.FindStringSubmatch(str)
	if len(match) > 1 {
		// Converte vírgula para ponto
		areaStr := strings.Replace(match[1], ",", ".", 1)
		area, err := strconv.ParseFloat(areaStr, 64)
		if err == nil {
			return area
		}
	}
	return 0
}

// Extrai o tipo de imóvel de uma string
func extractPropertyType(str string) string {
	str = strings.ToLower(str)

	if strings.Contains(str, "casa") {
		return "Casa"
	} else if strings.Contains(str, "apartamento") || strings.Contains(str, "apto") {
		return "Apartamento"
	} else if strings.Contains(str, "terreno") || strings.Contains(str, "lote") {
		return "Terreno"
	} else if strings.Contains(str, "comercial") || strings.Contains(str, "sala") || strings.Contains(str, "escritório") {
		return "Comercial"
	} else if strings.Contains(str, "rural") || strings.Contains(str, "fazenda") || strings.Contains(str, "sítio") || strings.Contains(str, "chácara") {
		return "Rural"
	}

	return "Outro"
}

// Limpa texto removendo espaços extras, quebras de linha, etc.
func cleanText(str string) string {
	// Remove espaços extras
	re := regexp.MustCompile(`\s+`)
	str = re.ReplaceAllString(str, " ")

	// Remove quebras de linha
	str = strings.ReplaceAll(str, "\n", " ")

	return strings.TrimSpace(str)
}

// Extrai cidade e bairro do endereço
func extractLocationInfo(endereco string) (cidade, bairro string) {
	// Tenta encontrar padrão "Bairro, Cidade"
	re := regexp.MustCompile(`([^,]+),\s*([^,]+)(?:,\s*[^,]+)?$`)
	matches := re.FindStringSubmatch(endereco)

	if len(matches) >= 3 {
		bairro = strings.TrimSpace(matches[1])
		cidade = strings.TrimSpace(matches[2])
	} else {
		// Tenta encontrar cidade comum
		for _, cityName := range []string{"Muzambinho", "Alfenas", "Guaxupé", "Juruaia", "Monte Belo", "São Paulo", "Rio de Janeiro", "Belo Horizonte"} {
			if strings.Contains(endereco, cityName) {
				cidade = cityName
				break
			}
		}
	}

	return
}

func mapToProperty(endereco, cidade, descricao, valorStr, url string) repository.Property {
	// Limpa os textos
	endereco = cleanText(endereco)
	cidade = cleanText(cidade)
	descricao = cleanText(descricao)
	valorStr = cleanText(valorStr)

	// Extrai informações adicionais
	valor := extractValue(valorStr)
	cep := extractCEP(endereco)
	quartos := extractRooms(descricao)
	banheiros := extractBathrooms(descricao)
	areaTotal := extractArea(descricao)
	tipoImovel := extractPropertyType(descricao + " " + endereco)

	// Se a cidade não foi encontrada, tenta extrair do endereço
	cidadeExtraida, bairro := extractLocationInfo(endereco)
	if cidade == "" {
		cidade = cidadeExtraida
	}

	// Extrai características do imóvel
	caracteristicas := []string{}
	for _, feature := range []string{"piscina", "churrasqueira", "garagem", "jardim", "varanda", "sacada"} {
		if strings.Contains(strings.ToLower(descricao), feature) {
			caracteristicas = append(caracteristicas, feature)
		}
	}

	return repository.Property{
		Endereco:        endereco,
		Cidade:          cidade,
		Bairro:          bairro,
		CEP:             cep,
		Descricao:       descricao,
		Valor:           valor,
		ValorTexto:      valorStr,
		Quartos:         quartos,
		Banheiros:       banheiros,
		AreaTotal:       areaTotal,
		TipoImovel:      tipoImovel,
		URL:             url,
		Caracteristicas: caracteristicas,
	}
}

// extractIndividualPropertiesFromCatalog extrai dados individuais de uma página de catálogo
func extractIndividualPropertiesFromCatalog(e *colly.HTMLElement, ctx context.Context, repo repository.PropertyRepository, aiService *ai.GeminiService) {
	url := e.Request.URL.String()

	// Tenta encontrar elementos que representam imóveis individuais no catálogo
	var properties []repository.Property

	// Procura por cards/containers de imóveis com seletores mais específicos
	e.ForEach(".imovel, .property, .card, .item, .listing, .property-card, .real-estate-item, [class*='imovel'], [class*='property'], [class*='listing']", func(i int, el *colly.HTMLElement) {
		// Extrai dados mais detalhados de cada elemento
		endereco := getFirstNonEmpty(el, ".endereco", ".address", ".location", ".street")
		if endereco == "" {
			// Tenta extrair endereço do texto do elemento
			text := el.Text
			addressRegex := regexp.MustCompile(`(?i)(R(?:ua|\.)\s+[^,\n]+|Av(?:enida|\.)\s+[^,\n]+|VILA\s+[^,\n]+|JARDIM\s+[^,\n]+)`)
			if match := addressRegex.FindString(text); match != "" {
				endereco = strings.TrimSpace(match)
			}
		}

		// Extrai preço
		valor := getFirstNonEmpty(el, ".preco", ".price", ".valor", ".value")
		if valor == "" {
			priceRegex := regexp.MustCompile(`R\$\s*[\d.,]+`)
			if match := priceRegex.FindString(el.Text); match != "" {
				valor = match
			}
		}

		// Extrai descrição
		descricao := getFirstNonEmpty(el, ".descricao", ".description", ".details", ".info")
		if descricao == "" {
			// Pega um resumo do texto do elemento
			text := strings.TrimSpace(el.Text)
			if len(text) > 50 && len(text) < 500 {
				descricao = text
			}
		}

		// Extrai características
		var caracteristicas []string
		el.ForEach(".caracteristica, .feature, .amenity", func(_ int, feat *colly.HTMLElement) {
			if feature := strings.TrimSpace(feat.Text); feature != "" {
				caracteristicas = append(caracteristicas, feature)
			}
		})

		if endereco != "" || valor != "" {
			property := repository.Property{
				Endereco:        endereco,
				Cidade:          extractCityFromText(endereco + " " + descricao),
				Descricao:       descricao,
				ValorTexto:      valor,
				Valor:           extractValue(valor),
				URL:             url,
				TipoImovel:      extractPropertyType(descricao + " " + endereco),
				Quartos:         extractRooms(descricao + " " + el.Text),
				Banheiros:       extractBathrooms(descricao + " " + el.Text),
				AreaTotal:       extractArea(descricao + " " + el.Text),
				Caracteristicas: caracteristicas,
			}

			// Processa com IA se disponível
			if aiService != nil {
				processedProperty, err := aiService.ProcessPropertyData(ctx, property)
				if err == nil {
					property = processedProperty
				}
			}

			properties = append(properties, property)
		}
	})

	// Se não encontrou elementos específicos, usa método de fallback mais inteligente
	if len(properties) == 0 {
		log.Printf("Nenhum elemento específico encontrado, usando fallback inteligente")

		// Divide o texto em blocos que podem representar imóveis individuais
		fullText := e.Text

		// Procura por padrões que indicam início de um novo imóvel
		blocks := regexp.MustCompile(`(?i)(R\$\s*[\d.,]+.*?(?=R\$|$))`).FindAllString(fullText, -1)

		for i, block := range blocks {
			if i >= 15 { // Limita para evitar spam
				break
			}

			block = strings.TrimSpace(block)
			if len(block) < 10 { // Ignora blocos muito pequenos
				continue
			}

			// Extrai informações do bloco
			endereco := extractAddressFromText(block)
			valor := ""
			if priceMatch := regexp.MustCompile(`R\$\s*[\d.,]+`).FindString(block); priceMatch != "" {
				valor = priceMatch
			}

			property := repository.Property{
				Endereco:   endereco,
				Cidade:     extractCityFromText(block),
				Descricao:  cleanText(block),
				ValorTexto: valor,
				Valor:      extractValue(valor),
				URL:        url,
				TipoImovel: extractPropertyType(block),
				Quartos:    extractRooms(block),
				Banheiros:  extractBathrooms(block),
				AreaTotal:  extractArea(block),
			}

			// Processa com IA se disponível
			if aiService != nil {
				processedProperty, err := aiService.ProcessPropertyData(ctx, property)
				if err == nil {
					property = processedProperty
				}
			}

			properties = append(properties, property)
		}
	}

	// Salva cada propriedade individualmente
	for _, property := range properties {
		if property.Endereco != "" || property.Valor > 0 {
			log.Printf("Salvando imóvel do catálogo: %s - Valor: %.2f - Tipo: %s", property.Endereco, property.Valor, property.TipoImovel)
			if err := repo.Save(ctx, property); err != nil {
				log.Printf("Erro ao salvar imóvel do catálogo: %v", err)
			}
		}
	}

	log.Printf("Extraídos %d imóveis individuais do catálogo: %s", len(properties), url)
}

// extractAddressFromText extrai endereço de um texto
func extractAddressFromText(text string) string {
	// Remove preços e outros ruídos
	text = regexp.MustCompile(`R\$\s*[\d.,]+`).ReplaceAllString(text, "")

	// Procura por padrões de endereço
	addressRegex := regexp.MustCompile(`(?i)(VILA|JARDIM|CENTRO|BAIRRO|RUA|AV)[A-ZÁÊÇÕ\s,]+`)
	if match := addressRegex.FindString(text); match != "" {
		return strings.TrimSpace(match)
	}

	// Se não encontrou padrão específico, pega as primeiras palavras
	words := strings.Fields(text)
	if len(words) > 0 && len(words) <= 5 {
		return strings.Join(words, " ")
	}

	return ""
}

// extractCityFromText extrai cidade de um texto
func extractCityFromText(text string) string {
	cities := []string{
		"MUZAMBINHO", "ALFENAS", "GUAXUPÉ", "JURUAIA", "MONTE BELO",
		"POÇOS DE CALDAS", "VARGINHA", "TRÊS PONTAS", "MACHADO", "PARAGUAÇU",
		"CABO VERDE", "CAMPANHA", "TRÊS CORAÇÕES", "ELÓI MENDES", "NEPOMUCENO",
		"CARMO DE MINAS", "CONCEIÇÃO DO RIO VERDE", "AREADO", "FAMA", "NOVA RESENDE",
		"SÃO SEBASTIÃO DO PARAÍSO", "JACUTINGA", "MONTE SANTO DE MINAS", "BOTELHOS",
		"BANDEIRA DO SUL", "SERRANIA", "GUARANÉSIA", "BORDA DA MATA", "OURO FINO",
		"CANAÃ", // Adicionando CANAÃ como cidade válida
	}

	// Lista de bairros conhecidos que NÃO são cidades
	neighborhoods := []string{
		"JARDIM POR DO SOL", "JARDIM EUROPA", "JARDIM MIRIAM", "VILA BUENO",
		"CENTRO", "JARDIM NOVO HORIZONTE", "PINHAL", "PARQUE DA COLINA",
		"JARDIM SÃO LUCAS", "JARDIM AGAPE", "VILA DORO", "ALTO DO ANJO",
		"PONTE PRETA", "JD. EUROPA II", "BAIRRO PINHAL", "BAIRRO DA LAJE",
		"BAIRRO CENTRO", "VILA NOVA", "JARDIM CALIFORNIA", "VILA ESPERANÇA",
	}

	textUpper := strings.ToUpper(text)

	// Primeiro verifica se é um bairro conhecido (retorna vazio para cidade)
	for _, neighborhood := range neighborhoods {
		if strings.Contains(textUpper, neighborhood) {
			return "" // É um bairro, não uma cidade
		}
	}

	// Depois verifica se é uma cidade válida
	for _, city := range cities {
		if strings.Contains(textUpper, city) {
			return strings.Title(strings.ToLower(city))
		}
	}

	// Procura por padrões como "Muzambinho-MG", "cidade de X", etc.
	cityPatterns := []string{
		"MUZAMBINHO-MG", "ALFENAS-MG", "GUAXUPÉ-MG", "JURUAIA-MG",
		"POÇOS DE CALDAS-MG", "VARGINHA-MG", "TRÊS PONTAS-MG",
	}

	for _, pattern := range cityPatterns {
		if strings.Contains(textUpper, pattern) {
			cityName := strings.Split(pattern, "-")[0]
			return strings.Title(strings.ToLower(cityName))
		}
	}

	return ""
}

// extractNeighborhoodFromText extrai bairro de um texto
func extractNeighborhoodFromText(text string) string {
	neighborhoods := []string{
		"JARDIM POR DO SOL", "JARDIM EUROPA", "JARDIM MIRIAM", "VILA BUENO",
		"CENTRO", "JARDIM NOVO HORIZONTE", "PINHAL", "PARQUE DA COLINA",
		"JARDIM SÃO LUCAS", "JARDIM AGAPE", "VILA DORO", "ALTO DO ANJO",
		"PONTE PRETA", "JD. EUROPA II", "CANAÃ", "BAIRRO PINHAL", "BAIRRO DA LAJE",
		"BAIRRO CENTRO", "VILA NOVA", "JARDIM CALIFORNIA", "VILA ESPERANÇA",
	}

	textUpper := strings.ToUpper(text)

	for _, neighborhood := range neighborhoods {
		if strings.Contains(textUpper, neighborhood) {
			return strings.Title(strings.ToLower(neighborhood))
		}
	}

	return ""
}

func StartCrawling(ctx context.Context, repo repository.PropertyRepository, urls []string, aiService *ai.GeminiService) {
	// Coletor principal para navegar nas páginas iniciais
	c := colly.NewCollector(
		colly.MaxDepth(10), // Limite de profundidade de links
		colly.Async(true),  // Permite coleta assíncrona
	)

	// Adiciona extensões úteis como User-Agent aleatório
	extensions.RandomUserAgent(c)
	extensions.Referer(c)

	// Coletor para páginas de detalhes de imóveis
	detailCollector := c.Clone()

	// Controle de concorrência
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1,
	})

	// Controle para evitar visitar a mesma URL várias vezes
	visitedURLs := make(map[string]bool)
	var visitedMutex sync.Mutex

	// Função para verificar se uma URL já foi visitada
	isVisited := func(url string) bool {
		visitedMutex.Lock()
		defer visitedMutex.Unlock()
		return visitedURLs[url]
	}

	// Função para marcar uma URL como visitada
	markVisited := func(url string) {
		visitedMutex.Lock()
		visitedURLs[url] = true
		visitedMutex.Unlock()
	}

	// Procura por links para páginas de detalhes de imóveis
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteLink := e.Request.AbsoluteURL(link)

		// Ignora links vazios ou externos
		if absoluteLink == "" || !strings.Contains(absoluteLink, e.Request.URL.Host) {
			return
		}

		// Verifica se parece ser um link para detalhes de imóvel (mais específico)
		if (strings.Contains(absoluteLink, "imovel") && !strings.Contains(absoluteLink, "imoveis/?")) ||
			strings.Contains(absoluteLink, "propriedade/") ||
			strings.Contains(absoluteLink, "/casa/") ||
			strings.Contains(absoluteLink, "/apartamento/") ||
			strings.Contains(absoluteLink, "/detalhes/") ||
			strings.Contains(absoluteLink, "/property/") ||
			strings.Contains(absoluteLink, "/listing/") ||
			(strings.Contains(absoluteLink, "/") && regexp.MustCompile(`/\d+/?$`).MatchString(absoluteLink)) {

			if !isVisited(absoluteLink) {
				markVisited(absoluteLink)
				log.Printf("Encontrou link de imóvel: %s", absoluteLink)
				detailCollector.Visit(absoluteLink)
			}
		}
	})

	// Função para detectar se é uma página de catálogo/listagem
	isCatalogPage := func(e *colly.HTMLElement) bool {
		url := e.Request.URL.String()
		text := strings.ToLower(e.Text)

		// Indicadores específicos de página de catálogo (mais restritivos)
		catalogIndicators := []string{
			"imoveis/?", "busca", "listagem", "catalogo", "pesquisa",
			"resultados", "filtros", "ordenar", "pagina=", "page=",
		}

		catalogCount := 0
		for _, indicator := range catalogIndicators {
			if strings.Contains(url, indicator) {
				catalogCount++
			}
		}

		// Só considera catálogo se tiver indicadores claros E múltiplos valores
		if catalogCount > 0 && (strings.Count(text, "r$") > 8 || strings.Count(text, "quartos") > 5) {
			return true
		}

		// URLs específicas que sabemos que são catálogos
		if strings.Contains(url, "imoveis/?operacao=") {
			return true
		}

		return false
	}

	// Função para extrair links de imóveis individuais de uma página de catálogo
	extractPropertyLinks := func(e *colly.HTMLElement) []string {
		var links []string

		// Procura por links que parecem ser de imóveis individuais
		e.ForEach("a[href]", func(_ int, el *colly.HTMLElement) {
			href := el.Attr("href")
			absoluteLink := e.Request.AbsoluteURL(href)

			// Ignora links externos ou inválidos
			if strings.Contains(absoluteLink, "wa.me") ||
				strings.Contains(absoluteLink, "whatsapp") ||
				strings.Contains(absoluteLink, "mailto:") ||
				strings.Contains(absoluteLink, "tel:") ||
				!strings.Contains(absoluteLink, e.Request.URL.Host) {
				return
			}

			// Verifica se o link parece ser de um imóvel específico
			if strings.Contains(absoluteLink, "detalhes") ||
				strings.Contains(absoluteLink, "imovel/") ||
				strings.Contains(absoluteLink, "propriedade/") ||
				strings.Contains(absoluteLink, "anuncio/") ||
				strings.Contains(absoluteLink, "/VENDAS/") ||
				strings.Contains(absoluteLink, "/ALUGUEIS/") ||
				regexp.MustCompile(`/\d+/?$`).MatchString(absoluteLink) {

				if !isVisited(absoluteLink) {
					links = append(links, absoluteLink)
				}
			}
		})

		return links
	}

	// Procura por dados de imóveis nas páginas de detalhes
	detailCollector.OnHTML("body", func(e *colly.HTMLElement) {
		// Pega a URL da página
		url := e.Request.URL.String()

		// Verifica se é uma página de catálogo
		if isCatalogPage(e) {
			log.Printf("Detectada página de catálogo: %s", url)

			// Extrai links individuais dos imóveis
			propertyLinks := extractPropertyLinks(e)
			log.Printf("Encontrados %d links de imóveis no catálogo", len(propertyLinks))

			// Se encontrou links, visita cada um
			if len(propertyLinks) > 0 {
				for _, link := range propertyLinks {
					if len(propertyLinks) <= 20 { // Limita para evitar sobrecarga
						markVisited(link)
						log.Printf("Visitando imóvel individual: %s", link)
						detailCollector.Visit(link)
					}
				}
				// Não processa dados do catálogo como se fosse um imóvel individual
				return
			} else {
				// Se não encontrou links no catálogo, extrai dados individuais do catálogo
				log.Printf("Nenhum link encontrado no catálogo, extraindo dados individuais: %s", url)
				extractIndividualPropertiesFromCatalog(e, ctx, repo, aiService)
				return
			}
		}

		// Tenta vários padrões comuns de classes/IDs para dados de imóveis (seletores específicos dos sites)
		endereco := getFirstNonEmpty(e,
			".endereco", ".address", ".property-address", "[itemprop=address]",
			"#endereco", "#address", ".street-address", ".location",
			".imovel-endereco", ".property-location", ".address-info",
			".property-title", ".listing-address", ".imovel-titulo",
			".property-info h3", ".property-info h4", ".card-title",
			".property-card-address", ".real-estate-address")

		// Se não encontrou endereço, tenta extrair do conteúdo geral da página
		if endereco == "" {
			// Procura por padrões de endereço no texto completo
			fullText := e.Text
			reAddress := regexp.MustCompile(`(?i)(R(?:ua|\.)\s+[\w\s]+,\s*\d+|Av(?:enida|\.)\s+[\w\s]+,\s*\d+)`)
			if match := reAddress.FindString(fullText); match != "" {
				endereco = match
			}
		}

		// Primeiro tenta extrair cidade do texto completo (mais confiável)
		fullText := endereco + " " + e.Text
		cidade := extractCityFromText(fullText)

		// Se não encontrou cidade no texto, tenta nos elementos HTML
		if cidade == "" {
			cidadeElement := getFirstNonEmpty(e,
				".cidade", ".city", ".property-city", "[itemprop=addressLocality]",
				"#cidade", "#city", ".city-name",
				".property-location", ".listing-location", ".imovel-cidade",
				".property-info .city", ".card-location", ".address-city")

			// Valida se o elemento encontrado é realmente uma cidade
			if cidadeElement != "" {
				cidadeValidated := extractCityFromText(cidadeElement)
				if cidadeValidated != "" {
					cidade = cidadeValidated
				}
			}
		}

		// Extrai bairro do endereço ou texto da página
		var bairro string
		bairroElement := getFirstNonEmpty(e,
			".bairro", ".neighborhood", ".district", ".property-neighborhood",
			"#bairro", "#neighborhood", ".locality", ".area")

		if bairroElement != "" {
			bairro = bairroElement
		} else {
			// Tenta extrair bairro do endereço ou texto geral
			fullText := endereco + " " + e.Text
			bairro = extractNeighborhoodFromText(fullText)
		}

		descricao := getFirstNonEmpty(e,
			".descricao", ".description", ".property-description", "[itemprop=description]",
			"#descricao", "#description", ".details", ".features", ".property-details",
			".imovel-descricao", ".property-text", ".property-info",
			".property-summary", ".listing-description", ".imovel-detalhes",
			".property-content", ".card-description", ".real-estate-description",
			".property-body", ".listing-details", ".imovel-info")

		// Se a descrição estiver vazia, tenta pegar outros elementos que possam conter informações úteis
		if descricao == "" {
			// Tenta pegar qualquer texto que pareça ser uma descrição
			descricaoTemp := ""
			e.ForEach("p", func(_ int, el *colly.HTMLElement) {
				text := strings.TrimSpace(el.Text)
				if len(text) > 30 { // Se o parágrafo tiver pelo menos 30 caracteres
					descricaoTemp += text + " "
				}
			})

			if descricaoTemp != "" {
				descricao = descricaoTemp
			}
		}

		valor := getFirstNonEmpty(e,
			".valor", ".price", ".property-price", "[itemprop=price]",
			"#valor", "#price", ".value", ".amount", ".property-value",
			".imovel-preco", ".valor-venda", ".valor-imovel",
			".listing-price", ".property-cost", ".card-price",
			".real-estate-price", ".property-amount", ".imovel-valor",
			".price-value", ".listing-value", ".property-price-value")

		// Se não encontrou valor, tenta extrair do conteúdo geral da página
		if valor == "" {
			fullText := e.Text
			rePrice := regexp.MustCompile(`(?i)R\$\s*[\d.,]+`)
			if match := rePrice.FindString(fullText); match != "" {
				valor = match
			}
		}

		// Tenta obter mais informações do imóvel
		// Procura por elementos que possam conter características
		var extraInfo string
		var caracteristicas []string

		// Lista de palavras que devem ser ignoradas nas características
		ignoredFeatures := map[string]bool{
			"INÍCIO": true, "INICIO": true, "SOBRE NÓS": true, "CONTATO": true,
			"IMÓVEIS URBANOS": true, "IMÓVEIS RURAIS": true, "DOCUMENTOS NECESSÁRIOS": true,
			"CADASTRE SEU IMÓVEL": true, "ALUGUEL": true, "VENDA": true, "VENDAS": true,
			"WHATSAPP": true, "E-MAIL": true, "FACEBOOK": true, "BANCO DO BRASIL": true,
			"CAIXA ECONÔMICA": true, "SANTANDER": true, "BRADESCO": true, "BANCO ITAÚ": true,
			"TIPO DE ATENDIMENTO": true, "ATENDIMENTO GERAL": true, "APARTAMENTO": true,
			"ÁREA": true, "BARRACÃO": true, "CASA": true, "CHÁCARA": true, "COMERCIAL": true,
			"LOTE": true, "SITIO": true, "SÍTIO": true, "SOBRADO": true, "TERRENO": true,
		}

		e.ForEach(".caracteristicas, .amenities, .features, .detalhes, .specifications, .property-features, .listing-features, .imovel-caracteristicas, .property-amenities, .real-estate-features", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" {
				extraInfo += text + " "
				// Tenta extrair características individuais
				lines := strings.Split(text, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					lineUpper := strings.ToUpper(line)

					// Filtra características válidas
					if len(line) > 3 && len(line) < 100 &&
						!ignoredFeatures[lineUpper] &&
						!strings.Contains(lineUpper, "CÓDIGO:") &&
						!strings.Contains(lineUpper, "R$") &&
						!strings.HasPrefix(lineUpper, "QUARTOS") &&
						!strings.HasPrefix(lineUpper, "SUÍTES") &&
						!strings.Contains(lineUpper, "HTTP") {
						caracteristicas = append(caracteristicas, line)
					}
				}
			}
		})

		// Procura por listas de características específicas do imóvel
		e.ForEach(".property-amenities li, .imovel-comodidades li, .features-list li", func(_ int, el *colly.HTMLElement) {
			feature := strings.TrimSpace(el.Text)
			featureUpper := strings.ToUpper(feature)

			if len(feature) > 3 && len(feature) < 100 &&
				!ignoredFeatures[featureUpper] &&
				!strings.Contains(featureUpper, "R$") {
				caracteristicas = append(caracteristicas, feature)
			}
		})

		// Adiciona informações extras à descrição
		if extraInfo != "" {
			descricao += " " + extraInfo
		}

		// Validações para evitar dados agregados/catálogo (mais permissivas)
		isValidProperty := func() bool {
			// Se o endereço contém MUITOS bairros/cidades, provavelmente é dados agregados
			if strings.Count(endereco, ",") > 8 {
				log.Printf("Endereço suspeito de dados agregados: %s", endereco)
				return false
			}

			// Se o valor contém MUITOS valores, é dados agregados
			if strings.Count(valor, "R$") > 3 {
				log.Printf("Valor suspeito de dados agregados: %s", valor)
				return false
			}

			// Se a descrição é muito genérica ou contém texto de catálogo
			descLower := strings.ToLower(descricao)
			catalogPhrases := []string{
				"show room de negócios imobiliários", "gedaim - gerenciamento",
			}

			for _, phrase := range catalogPhrases {
				if strings.Contains(descLower, phrase) {
					log.Printf("Descrição suspeita de ser catálogo: %s", url)
					return false
				}
			}

			return true
		}

		// Se encontrou pelo menos endereço ou valor, e passou nas validações
		if (endereco != "" || valor != "") && isValidProperty() {
			property := mapToProperty(
				endereco,
				cidade,
				descricao,
				valor,
				url,
			)

			// Adiciona bairro se encontrado
			if bairro != "" {
				property.Bairro = bairro
			}

			// Adiciona características se encontradas
			if len(caracteristicas) > 0 {
				property.Caracteristicas = caracteristicas
			}

			// Só salva se tiver informações suficientes
			if property.Endereco != "" || property.Valor > 0 {
				log.Printf("Imóvel encontrado: %s - Valor: %.2f", property.Endereco, property.Valor)

				// Processar com IA se disponível
				if aiService != nil {
					processedProperty, err := aiService.ProcessPropertyData(ctx, property)
					if err != nil {
						log.Printf("Erro ao processar dados com IA: %v", err)
						// Continua com os dados originais se houver erro
					} else {
						property = processedProperty
						log.Printf("Dados processados com IA: %s - Valor: %.2f - Tipo: %s",
							property.Endereco, property.Valor, property.TipoImovel)
					}
				}

				if err := repo.Save(ctx, property); err != nil {
					log.Printf("Error saving property: %v", err)
				}
			}
		}
	})

	c.OnRequest(func(r *colly.Request) {
		log.Printf("Visitando página de listagem: %s", r.URL)
	})

	detailCollector.OnRequest(func(r *colly.Request) {
		log.Printf("Visitando página de detalhes: %s", r.URL)
	})

	// Inicia a coleta a partir das URLs iniciais
	for _, url := range urls {
		if !isVisited(url) {
			markVisited(url)
			if err := c.Visit(url); err != nil {
				log.Printf("Error visiting URL %s: %v", url, err)
			}
		}
	}

	// Aguarda a conclusão de todas as solicitações
	c.Wait()
	detailCollector.Wait()

	// Processa qualquer propriedade restante no buffer da IA
	if aiService != nil {
		if err := aiService.FlushBatch(ctx); err != nil {
			log.Printf("Erro ao processar buffer restante da IA: %v", err)
		}

		// Log das estatísticas do cache
		total, expired := aiService.GetCacheStats()
		log.Printf("Cache IA - Total: %d, Expirados: %d", total, expired)
	}
}

// getFirstNonEmpty tenta vários seletores e retorna o primeiro que encontrar conteúdo
func getFirstNonEmpty(e *colly.HTMLElement, selectors ...string) string {
	for _, selector := range selectors {
		text := strings.TrimSpace(e.ChildText(selector))
		if text != "" {
			return text
		}
	}
	return ""
}

func LoadURLsFromFile(filePath string) ([]string, error) {
	var urls []string
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &urls); err != nil {
		return nil, err
	}

	return urls, nil
}
