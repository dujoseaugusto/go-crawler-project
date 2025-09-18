package crawler

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
)

// ContentPatternTrainer aprende padrões baseado no conteúdo das páginas de referência
type ContentPatternTrainer struct {
	referenceURLs   []string
	learnedPatterns *ContentPatterns
	logger          *logger.Logger
	httpClient      *http.Client
}

// ContentPatterns armazena os padrões aprendidos do conteúdo
type ContentPatterns struct {
	// Elementos HTML comuns em anúncios
	CommonSelectors  []string `json:"common_selectors"`
	PriceSelectors   []string `json:"price_selectors"`
	DetailsSelectors []string `json:"details_selectors"`

	// Padrões de texto
	CommonKeywords   []string         `json:"common_keywords"`
	PricePatterns    []*regexp.Regexp `json:"-"` // Não serializar regex
	PricePatternsStr []string         `json:"price_patterns"`

	// Estrutura da página
	MinTextLength    int `json:"min_text_length"`
	MaxTextLength    int `json:"max_text_length"`
	TypicalWordCount int `json:"typical_word_count"`

	// Indicadores de qualidade
	RequiredElements  []string `json:"required_elements"`
	ForbiddenKeywords []string `json:"forbidden_keywords"`

	// Estatísticas
	TotalAnalyzed      int `json:"total_analyzed"`
	SuccessfulAnalysis int `json:"successful_analysis"`
}

// NewContentPatternTrainer cria um novo treinador baseado em conteúdo
func NewContentPatternTrainer(referenceURLs []string) *ContentPatternTrainer {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		},
	}

	return &ContentPatternTrainer{
		referenceURLs: referenceURLs,
		logger:        logger.NewLogger("content_pattern_trainer"),
		httpClient:    client,
		learnedPatterns: &ContentPatterns{
			CommonSelectors:   []string{},
			PriceSelectors:    []string{},
			DetailsSelectors:  []string{},
			CommonKeywords:    []string{},
			PricePatterns:     []*regexp.Regexp{},
			PricePatternsStr:  []string{},
			RequiredElements:  []string{},
			ForbiddenKeywords: []string{},
		},
	}
}

// TrainFromReferencePages visita as páginas de referência e aprende padrões
func (cpt *ContentPatternTrainer) TrainFromReferencePages() error {
	cpt.logger.Info("Starting content pattern training from reference pages")

	var analyzedPages []PageAnalysis

	for i, url := range cpt.referenceURLs {
		if strings.TrimSpace(url) == "" {
			continue
		}

		cpt.logger.WithField("url", url).Debug("Analyzing reference page")

		analysis, err := cpt.analyzePage(url)
		if err != nil {
			cpt.logger.WithError(err).WithField("url", url).Warn("Failed to analyze reference page")
			continue
		}

		analyzedPages = append(analyzedPages, *analysis)
		cpt.learnedPatterns.TotalAnalyzed++

		// Pequeno delay para não sobrecarregar os servidores
		if i < len(cpt.referenceURLs)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	// Consolidar padrões das páginas analisadas
	err := cpt.consolidatePatterns(analyzedPages)
	if err != nil {
		return fmt.Errorf("erro ao consolidar padrões: %v", err)
	}

	cpt.learnedPatterns.SuccessfulAnalysis = len(analyzedPages)

	cpt.logger.WithFields(map[string]interface{}{
		"total_analyzed":      cpt.learnedPatterns.TotalAnalyzed,
		"successful_analysis": cpt.learnedPatterns.SuccessfulAnalysis,
		"common_selectors":    len(cpt.learnedPatterns.CommonSelectors),
		"price_selectors":     len(cpt.learnedPatterns.PriceSelectors),
		"common_keywords":     len(cpt.learnedPatterns.CommonKeywords),
	}).Info("Content pattern training completed")

	return nil
}

// PageAnalysis representa a análise de uma página
type PageAnalysis struct {
	URL            string
	Title          string
	TextContent    string
	WordCount      int
	PriceElements  []ElementInfo
	DetailElements []ElementInfo
	AllSelectors   []string
	Keywords       []string
	HasPrice       bool
	HasDetails     bool
	IsValid        bool
}

// ElementInfo informações sobre um elemento HTML
type ElementInfo struct {
	Selector string
	Text     string
	Classes  []string
	ID       string
}

// analyzePage analisa uma página específica
func (cpt *ContentPatternTrainer) analyzePage(url string) (*PageAnalysis, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PropertyCrawler/1.0)")

	resp, err := cpt.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	analysis := &PageAnalysis{
		URL:            url,
		Title:          doc.Find("title").Text(),
		TextContent:    doc.Text(),
		PriceElements:  []ElementInfo{},
		DetailElements: []ElementInfo{},
		AllSelectors:   []string{},
		Keywords:       []string{},
	}

	// Contar palavras
	words := strings.Fields(analysis.TextContent)
	analysis.WordCount = len(words)

	// Encontrar elementos de preço
	cpt.findPriceElements(doc, analysis)

	// Encontrar elementos de detalhes
	cpt.findDetailElements(doc, analysis)

	// Extrair seletores comuns
	cpt.extractCommonSelectors(doc, analysis)

	// Extrair palavras-chave
	cpt.extractKeywords(analysis)

	// Validar se é uma página válida
	analysis.IsValid = cpt.validatePage(analysis)

	return analysis, nil
}

// findPriceElements encontra elementos que contêm preços
func (cpt *ContentPatternTrainer) findPriceElements(doc *goquery.Document, analysis *PageAnalysis) {
	pricePatterns := []*regexp.Regexp{
		regexp.MustCompile(`R\$\s*[\d.,]+`),
		regexp.MustCompile(`\$\s*[\d.,]+`),
		regexp.MustCompile(`[\d.,]+\s*reais?`),
		regexp.MustCompile(`Valor:?\s*R\$\s*[\d.,]+`),
		regexp.MustCompile(`Preço:?\s*R\$\s*[\d.,]+`),
	}

	// Procurar em elementos comuns de preço
	priceSelectors := []string{
		".price", ".valor", ".preco", ".value",
		"[class*='price']", "[class*='valor']", "[class*='preco']",
		"[id*='price']", "[id*='valor']", "[id*='preco']",
		".property-price", ".listing-price", ".item-price",
	}

	for _, selector := range priceSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				for _, pattern := range pricePatterns {
					if pattern.MatchString(text) {
						classes, _ := s.Attr("class")
						id, _ := s.Attr("id")

						analysis.PriceElements = append(analysis.PriceElements, ElementInfo{
							Selector: selector,
							Text:     text,
							Classes:  strings.Fields(classes),
							ID:       id,
						})
						analysis.HasPrice = true
						break
					}
				}
			}
		})
	}

	// Se não encontrou com seletores específicos, procurar no texto geral
	if !analysis.HasPrice {
		for _, pattern := range pricePatterns {
			if pattern.MatchString(analysis.TextContent) {
				analysis.HasPrice = true
				break
			}
		}
	}
}

// findDetailElements encontra elementos com detalhes do imóvel
func (cpt *ContentPatternTrainer) findDetailElements(doc *goquery.Document, analysis *PageAnalysis) {
	detailKeywords := []string{
		"quartos", "dormitórios", "suítes", "banheiros",
		"área", "m²", "m2", "metros",
		"garagem", "vagas", "estacionamento",
		"condomínio", "iptu",
	}

	detailSelectors := []string{
		".details", ".detalhes", ".caracteristicas", ".features",
		".property-details", ".listing-details", ".item-details",
		"[class*='detail']", "[class*='caracteristic']", "[class*='feature']",
		".specs", ".specifications", ".info",
	}

	for _, selector := range detailSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.ToLower(strings.TrimSpace(s.Text()))
			if text != "" {
				for _, keyword := range detailKeywords {
					if strings.Contains(text, keyword) {
						classes, _ := s.Attr("class")
						id, _ := s.Attr("id")

						analysis.DetailElements = append(analysis.DetailElements, ElementInfo{
							Selector: selector,
							Text:     text,
							Classes:  strings.Fields(classes),
							ID:       id,
						})
						analysis.HasDetails = true
						break
					}
				}
			}
		})
	}
}

// extractCommonSelectors extrai seletores comuns da página
func (cpt *ContentPatternTrainer) extractCommonSelectors(doc *goquery.Document, analysis *PageAnalysis) {
	// Elementos que geralmente contêm informações importantes
	importantSelectors := []string{
		"h1", "h2", "h3", ".title", ".titulo",
		".description", ".descricao", ".content", ".conteudo",
		".property", ".imovel", ".listing", ".anuncio",
		".contact", ".contato", ".phone", ".telefone",
	}

	for _, selector := range importantSelectors {
		if doc.Find(selector).Length() > 0 {
			analysis.AllSelectors = append(analysis.AllSelectors, selector)
		}
	}
}

// extractKeywords extrai palavras-chave relevantes
func (cpt *ContentPatternTrainer) extractKeywords(analysis *PageAnalysis) {
	text := strings.ToLower(analysis.TextContent)

	// EXPANDIDO: Mais palavras-chave para melhor detecção
	propertyKeywords := []string{
		// Tipos de imóveis
		"casa", "apartamento", "terreno", "sitio", "chacara", "lote",
		"sobrado", "cobertura", "loft", "studio", "comercial", "kitnet",
		"loja", "sala", "galpao", "barracao", "fazenda", "rural",
		"residencial", "imovel", "propriedade", "edificio", "predio",

		// Transações
		"venda", "aluguel", "locacao", "comprar", "alugar", "vender",
		"negocio", "oportunidade", "investimento", "financiamento",

		// Características
		"quartos", "dormitorios", "banheiros", "suites", "garagem", "vagas",
		"area", "metros", "m²", "m2", "condominio", "iptu", "escritura",
		"piscina", "jardim", "churrasqueira", "sacada", "varanda",

		// Valores e medidas
		"preco", "valor", "reais", "mil", "milhao", "parcelas",
		"entrada", "financiado", "vista", "negociavel",
	}

	for _, keyword := range propertyKeywords {
		if strings.Contains(text, keyword) {
			analysis.Keywords = append(analysis.Keywords, keyword)
		}
	}
}

// validatePage valida se a página é um anúncio válido
func (cpt *ContentPatternTrainer) validatePage(analysis *PageAnalysis) bool {
	// Critérios para ser considerado um anúncio válido
	hasMinimumContent := analysis.WordCount >= 50
	hasPropertyKeywords := len(analysis.Keywords) >= 3
	hasStructure := len(analysis.AllSelectors) >= 3

	return hasMinimumContent && hasPropertyKeywords && hasStructure && (analysis.HasPrice || analysis.HasDetails)
}

// consolidatePatterns consolida os padrões de todas as páginas analisadas
func (cpt *ContentPatternTrainer) consolidatePatterns(analyses []PageAnalysis) error {
	if len(analyses) == 0 {
		return fmt.Errorf("nenhuma página foi analisada com sucesso")
	}

	// Contar frequência de seletores
	selectorCount := make(map[string]int)
	keywordCount := make(map[string]int)

	totalWordCount := 0
	validAnalyses := 0

	for _, analysis := range analyses {
		if !analysis.IsValid {
			continue
		}

		validAnalyses++
		totalWordCount += analysis.WordCount

		// Contar seletores
		for _, selector := range analysis.AllSelectors {
			selectorCount[selector]++
		}

		// Contar palavras-chave
		for _, keyword := range analysis.Keywords {
			keywordCount[keyword]++
		}

		// Adicionar seletores de preço
		for _, priceEl := range analysis.PriceElements {
			cpt.learnedPatterns.PriceSelectors = append(cpt.learnedPatterns.PriceSelectors, priceEl.Selector)
		}

		// Adicionar seletores de detalhes
		for _, detailEl := range analysis.DetailElements {
			cpt.learnedPatterns.DetailsSelectors = append(cpt.learnedPatterns.DetailsSelectors, detailEl.Selector)
		}
	}

	if validAnalyses == 0 {
		return fmt.Errorf("nenhuma página válida foi encontrada")
	}

	// Selecionar seletores mais comuns (aparecem em pelo menos 30% das páginas)
	minFrequency := validAnalyses * 30 / 100
	if minFrequency < 1 {
		minFrequency = 1
	}

	for selector, count := range selectorCount {
		if count >= minFrequency {
			cpt.learnedPatterns.CommonSelectors = append(cpt.learnedPatterns.CommonSelectors, selector)
		}
	}

	// Selecionar palavras-chave mais comuns
	for keyword, count := range keywordCount {
		if count >= minFrequency {
			cpt.learnedPatterns.CommonKeywords = append(cpt.learnedPatterns.CommonKeywords, keyword)
		}
	}

	// Calcular estatísticas
	cpt.learnedPatterns.TypicalWordCount = totalWordCount / validAnalyses
	cpt.learnedPatterns.MinTextLength = cpt.learnedPatterns.TypicalWordCount / 2
	cpt.learnedPatterns.MaxTextLength = cpt.learnedPatterns.TypicalWordCount * 3

	// Definir elementos obrigatórios
	cpt.learnedPatterns.RequiredElements = []string{"h1", "h2", "h3"} // Pelo menos um título

	// REMOVIDO: Palavras proibidas - foco apenas em características POSITIVAS
	// Não usar palavras proibidas, apenas detectar anúncios por características positivas
	cpt.learnedPatterns.ForbiddenKeywords = []string{}

	// Compilar padrões de preço
	pricePatternStrs := []string{
		`R\$\s*[\d.,]+`,
		`\$\s*[\d.,]+`,
		`[\d.,]+\s*reais?`,
		`Valor:?\s*R\$\s*[\d.,]+`,
		`Preço:?\s*R\$\s*[\d.,]+`,
	}

	for _, patternStr := range pricePatternStrs {
		if pattern, err := regexp.Compile(patternStr); err == nil {
			cpt.learnedPatterns.PricePatterns = append(cpt.learnedPatterns.PricePatterns, pattern)
			cpt.learnedPatterns.PricePatternsStr = append(cpt.learnedPatterns.PricePatternsStr, patternStr)
		}
	}

	// Remover duplicatas
	cpt.learnedPatterns.CommonSelectors = cpt.removeDuplicates(cpt.learnedPatterns.CommonSelectors)
	cpt.learnedPatterns.PriceSelectors = cpt.removeDuplicates(cpt.learnedPatterns.PriceSelectors)
	cpt.learnedPatterns.DetailsSelectors = cpt.removeDuplicates(cpt.learnedPatterns.DetailsSelectors)
	cpt.learnedPatterns.CommonKeywords = cpt.removeDuplicates(cpt.learnedPatterns.CommonKeywords)

	return nil
}

// removeDuplicates remove elementos duplicados de uma slice
func (cpt *ContentPatternTrainer) removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// GetLearnedPatterns retorna os padrões aprendidos
func (cpt *ContentPatternTrainer) GetLearnedPatterns() *ContentPatterns {
	return cpt.learnedPatterns
}

// PrintAnalysis imprime análise dos padrões aprendidos
func (cpt *ContentPatternTrainer) PrintAnalysis() {
	fmt.Printf("=== Análise dos Padrões de Conteúdo ===\n")
	fmt.Printf("Páginas analisadas: %d\n", cpt.learnedPatterns.TotalAnalyzed)
	fmt.Printf("Análises bem-sucedidas: %d\n", cpt.learnedPatterns.SuccessfulAnalysis)
	fmt.Printf("Contagem típica de palavras: %d\n", cpt.learnedPatterns.TypicalWordCount)
	fmt.Printf("\nSeletores comuns (%d):\n", len(cpt.learnedPatterns.CommonSelectors))
	for _, selector := range cpt.learnedPatterns.CommonSelectors {
		fmt.Printf("  - %s\n", selector)
	}
	fmt.Printf("\nSeletores de preço (%d):\n", len(cpt.learnedPatterns.PriceSelectors))
	for _, selector := range cpt.learnedPatterns.PriceSelectors {
		fmt.Printf("  - %s\n", selector)
	}
	fmt.Printf("\nPalavras-chave comuns (%d):\n", len(cpt.learnedPatterns.CommonKeywords))
	for _, keyword := range cpt.learnedPatterns.CommonKeywords {
		fmt.Printf("  - %s\n", keyword)
	}
}
