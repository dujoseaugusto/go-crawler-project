package crawler

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gocolly/colly"
)

// InteractiveDiscoveryManager gerencia descoberta através de interações (botões, formulários)
type InteractiveDiscoveryManager struct {
	logger      *logger.Logger
	visitedURLs map[string]bool
}

// InteractiveDiscoveryResult resultado da descoberta interativa
type InteractiveDiscoveryResult struct {
	SearchButtons   []SearchButton   `json:"search_buttons"`
	SearchForms     []SearchForm     `json:"search_forms"`
	NavigationLinks []NavigationLink `json:"navigation_links"`
	DiscoveredURLs  []string         `json:"discovered_urls"`
	TotalDiscovered int              `json:"total_discovered"`
}

// SearchButton representa um botão de busca encontrado
type SearchButton struct {
	Text       string  `json:"text"`
	URL        string  `json:"url"`
	Method     string  `json:"method"`
	Confidence float64 `json:"confidence"`
	ButtonType string  `json:"button_type"`
}

// SearchForm representa um formulário de busca
type SearchForm struct {
	Action     string            `json:"action"`
	Method     string            `json:"method"`
	Fields     map[string]string `json:"fields"`
	SubmitText string            `json:"submit_text"`
	Confidence float64           `json:"confidence"`
}

// NavigationLink representa um link de navegação importante
type NavigationLink struct {
	Text       string  `json:"text"`
	URL        string  `json:"url"`
	Confidence float64 `json:"confidence"`
	Category   string  `json:"category"`
}

// NewInteractiveDiscoveryManager cria um novo gerenciador de descoberta interativa
func NewInteractiveDiscoveryManager() *InteractiveDiscoveryManager {
	return &InteractiveDiscoveryManager{
		logger:      logger.NewLogger("interactive_discovery"),
		visitedURLs: make(map[string]bool),
	}
}

// DiscoverInteractiveElements descobre elementos interativos na página
func (idm *InteractiveDiscoveryManager) DiscoverInteractiveElements(doc *goquery.Document, currentURL string) InteractiveDiscoveryResult {
	result := InteractiveDiscoveryResult{
		SearchButtons:   []SearchButton{},
		SearchForms:     []SearchForm{},
		NavigationLinks: []NavigationLink{},
		DiscoveredURLs:  []string{},
	}

	idm.logger.WithField("url", currentURL).Info("Starting interactive discovery")

	// 1. DESCOBRIR BOTÕES DE BUSCA
	result.SearchButtons = idm.discoverSearchButtons(doc, currentURL)

	// 2. DESCOBRIR FORMULÁRIOS DE BUSCA
	result.SearchForms = idm.discoverSearchForms(doc, currentURL)

	// 3. DESCOBRIR LINKS DE NAVEGAÇÃO IMPORTANTES
	result.NavigationLinks = idm.discoverNavigationLinks(doc, currentURL)

	// 4. GERAR URLs DESCOBERTAS
	result.DiscoveredURLs = idm.generateDiscoveredURLs(result, currentURL)
	result.TotalDiscovered = len(result.DiscoveredURLs)

	idm.logger.WithFields(map[string]interface{}{
		"url":              currentURL,
		"search_buttons":   len(result.SearchButtons),
		"search_forms":     len(result.SearchForms),
		"navigation_links": len(result.NavigationLinks),
		"discovered_urls":  result.TotalDiscovered,
	}).Info("Interactive discovery completed")

	return result
}

// discoverSearchButtons descobre botões de busca na página
func (idm *InteractiveDiscoveryManager) discoverSearchButtons(doc *goquery.Document, baseURL string) []SearchButton {
	var buttons []SearchButton

	// SELETORES PARA BOTÕES DE BUSCA
	buttonSelectors := []string{
		"button", "input[type='submit']", "input[type='button']",
		"a[role='button']", ".btn", ".button",
		"[class*='search']", "[class*='busca']", "[class*='pesquisa']",
		"[id*='search']", "[id*='busca']", "[id*='pesquisa']",
	}

	for _, selector := range buttonSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			button := idm.analyzeButton(s, baseURL)
			if button.Confidence > 0.3 {
				buttons = append(buttons, button)
			}
		})
	}

	return buttons
}

// analyzeButton analisa um botão para determinar se é relevante para busca
func (idm *InteractiveDiscoveryManager) analyzeButton(s *goquery.Selection, baseURL string) SearchButton {
	button := SearchButton{
		Method:     "GET",
		Confidence: 0.0,
	}

	// Extrair texto do botão
	text := strings.TrimSpace(s.Text())
	if text == "" {
		if value, exists := s.Attr("value"); exists {
			text = value
		}
		if title, exists := s.Attr("title"); exists && text == "" {
			text = title
		}
	}
	button.Text = text

	// Analisar relevância do texto
	searchTexts := []string{
		"buscar", "pesquisar", "search", "procurar", "encontrar",
		"ver todos", "todos os imóveis", "buscar imóveis",
		"comprar", "alugar", "venda", "aluguel",
		"apartamentos", "casas", "terrenos", "imóveis",
		"filtrar", "aplicar filtros", "pesquisa avançada",
	}

	textLower := strings.ToLower(text)
	for _, searchText := range searchTexts {
		if strings.Contains(textLower, searchText) {
			button.Confidence += 0.3
			button.ButtonType = "search"
			break
		}
	}

	// Analisar classes e IDs
	if class, exists := s.Attr("class"); exists {
		classLower := strings.ToLower(class)
		if strings.Contains(classLower, "search") ||
			strings.Contains(classLower, "busca") ||
			strings.Contains(classLower, "pesquisa") {
			button.Confidence += 0.2
		}
	}

	if id, exists := s.Attr("id"); exists {
		idLower := strings.ToLower(id)
		if strings.Contains(idLower, "search") ||
			strings.Contains(idLower, "busca") ||
			strings.Contains(idLower, "pesquisa") {
			button.Confidence += 0.2
		}
	}

	// Determinar URL do botão
	if href, exists := s.Attr("href"); exists {
		button.URL = idm.resolveURL(href, baseURL)
	} else {
		// Para botões de formulário, tentar encontrar o formulário pai
		form := s.Closest("form")
		if form.Length() > 0 {
			if action, exists := form.Attr("action"); exists {
				button.URL = idm.resolveURL(action, baseURL)
			} else {
				// Se não tem action, usar a URL base
				button.URL = baseURL
			}
			if method, exists := form.Attr("method"); exists {
				button.Method = strings.ToUpper(method)
			}
		} else {
			// Se não é link nem formulário, gerar URLs baseadas no texto do botão
			button.URL = idm.generateURLFromButtonText(textLower, baseURL)
		}
	}

	return button
}

// discoverSearchForms descobre formulários de busca
func (idm *InteractiveDiscoveryManager) discoverSearchForms(doc *goquery.Document, baseURL string) []SearchForm {
	var forms []SearchForm

	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		form := idm.analyzeForm(s, baseURL)
		if form.Confidence > 0.3 {
			forms = append(forms, form)
		}
	})

	return forms
}

// analyzeForm analisa um formulário para determinar se é relevante para busca
func (idm *InteractiveDiscoveryManager) analyzeForm(s *goquery.Selection, baseURL string) SearchForm {
	form := SearchForm{
		Method:     "GET",
		Fields:     make(map[string]string),
		Confidence: 0.0,
	}

	// Extrair action e method
	if action, exists := s.Attr("action"); exists {
		form.Action = idm.resolveURL(action, baseURL)
	} else {
		form.Action = baseURL
	}

	if method, exists := s.Attr("method"); exists {
		form.Method = strings.ToUpper(method)
	}

	// Analisar campos do formulário
	s.Find("input, select, textarea").Each(func(i int, field *goquery.Selection) {
		if name, exists := field.Attr("name"); exists {
			fieldType, _ := field.Attr("type")
			placeholder, _ := field.Attr("placeholder")

			form.Fields[name] = fieldType

			// Analisar relevância dos campos
			nameLower := strings.ToLower(name)
			placeholderLower := strings.ToLower(placeholder)

			searchFields := []string{
				"search", "busca", "pesquisa", "query", "q",
				"cidade", "city", "bairro", "neighborhood",
				"tipo", "type", "categoria", "category",
				"preco", "price", "valor", "value",
				"quartos", "rooms", "bedrooms",
				"area", "tamanho", "size",
			}

			for _, searchField := range searchFields {
				if strings.Contains(nameLower, searchField) ||
					strings.Contains(placeholderLower, searchField) {
					form.Confidence += 0.2
					break
				}
			}
		}
	})

	// Analisar botão de submit
	submitButton := s.Find("input[type='submit'], button[type='submit'], button:not([type])")
	if submitButton.Length() > 0 {
		form.SubmitText = strings.TrimSpace(submitButton.First().Text())
		if form.SubmitText == "" {
			if value, exists := submitButton.First().Attr("value"); exists {
				form.SubmitText = value
			}
		}

		submitTextLower := strings.ToLower(form.SubmitText)
		if strings.Contains(submitTextLower, "buscar") ||
			strings.Contains(submitTextLower, "pesquisar") ||
			strings.Contains(submitTextLower, "search") {
			form.Confidence += 0.3
		}
	}

	// Bonus para formulários com múltiplos campos relevantes
	if len(form.Fields) >= 2 {
		form.Confidence += 0.2
	}

	return form
}

// discoverNavigationLinks descobre links de navegação importantes
func (idm *InteractiveDiscoveryManager) discoverNavigationLinks(doc *goquery.Document, baseURL string) []NavigationLink {
	var links []NavigationLink

	// SELETORES PARA NAVEGAÇÃO
	navSelectors := []string{
		"nav a", ".navigation a", ".menu a", ".navbar a",
		".header a", ".footer a", ".sidebar a",
		"[class*='nav'] a", "[class*='menu'] a",
	}

	for _, selector := range navSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			link := idm.analyzeNavigationLink(s, baseURL)
			if link.Confidence > 0.4 {
				links = append(links, link)
			}
		})
	}

	return links
}

// analyzeNavigationLink analisa um link de navegação
func (idm *InteractiveDiscoveryManager) analyzeNavigationLink(s *goquery.Selection, baseURL string) NavigationLink {
	link := NavigationLink{
		Confidence: 0.0,
	}

	// Extrair texto e URL
	link.Text = strings.TrimSpace(s.Text())
	if href, exists := s.Attr("href"); exists {
		link.URL = idm.resolveURL(href, baseURL)
	}

	if link.URL == "" || link.Text == "" {
		return link
	}

	textLower := strings.ToLower(link.Text)
	urlLower := strings.ToLower(link.URL)

	// CATEGORIAS DE NAVEGAÇÃO IMPORTANTES
	categories := map[string][]string{
		"search":    {"buscar", "pesquisar", "search", "procurar", "encontrar"},
		"catalog":   {"comprar", "alugar", "venda", "aluguel", "imóveis", "propriedades"},
		"types":     {"apartamentos", "casas", "terrenos", "comercial", "rural"},
		"locations": {"cidades", "bairros", "regiões", "localização"},
	}

	for category, keywords := range categories {
		for _, keyword := range keywords {
			if strings.Contains(textLower, keyword) || strings.Contains(urlLower, keyword) {
				link.Category = category
				link.Confidence += 0.3
				break
			}
		}
		if link.Confidence > 0 {
			break
		}
	}

	return link
}

// generateDiscoveredURLs gera URLs descobertas a partir dos elementos interativos
func (idm *InteractiveDiscoveryManager) generateDiscoveredURLs(result InteractiveDiscoveryResult, baseURL string) []string {
	var urls []string
	urlSet := make(map[string]bool)

	// URLs de botões de busca
	for _, button := range result.SearchButtons {
		if button.URL != "" && !urlSet[button.URL] {
			urls = append(urls, button.URL)
			urlSet[button.URL] = true
		}
	}

	// URLs de formulários (com parâmetros padrão)
	for _, form := range result.SearchForms {
		if form.Action != "" {
			// Gerar URLs com parâmetros comuns
			generatedURLs := idm.generateFormURLs(form, baseURL)
			for _, url := range generatedURLs {
				if !urlSet[url] {
					urls = append(urls, url)
					urlSet[url] = true
				}
			}
		}
	}

	// URLs de links de navegação
	for _, navLink := range result.NavigationLinks {
		if navLink.URL != "" && !urlSet[navLink.URL] {
			urls = append(urls, navLink.URL)
			urlSet[navLink.URL] = true
		}
	}

	return urls
}

// generateFormURLs gera URLs a partir de formulários com parâmetros padrão
func (idm *InteractiveDiscoveryManager) generateFormURLs(form SearchForm, baseURL string) []string {
	var urls []string

	if form.Method != "GET" {
		return urls
	}

	// Parâmetros padrão para tentar (comentado por enquanto)
	// defaultParams := map[string][]string{
	//	"tipo":      {"todos", "apartamento", "casa", "terreno"},
	//	"operacao":  {"venda", "aluguel", "compra"},
	//	"cidade":    {"todos", ""},
	//	"bairro":    {"todos", ""},
	//	"quartos":   {"", "1", "2", "3"},
	//	"preco_min": {""},
	//	"preco_max": {""},
	// }

	// Gerar algumas combinações básicas
	baseParams := []map[string]string{
		{}, // Sem parâmetros
		{"operacao": "venda"},
		{"operacao": "aluguel"},
		{"tipo": "apartamento", "operacao": "venda"},
		{"tipo": "casa", "operacao": "venda"},
		{"tipo": "terreno", "operacao": "venda"},
		{"tipo": "todos", "operacao": "venda"},
		{"cidade": "todos", "operacao": "venda"},
		{"bairro": "todos-os-bairros", "operacao": "venda"},
	}
	
	// ADICIONAR URLs ESPECÍFICAS CONHECIDAS para sites brasileiros
	knownCatalogURLs := idm.generateKnownCatalogURLs(baseURL)
	for _, knownURL := range knownCatalogURLs {
		urls = append(urls, knownURL)
	}

	for _, params := range baseParams {
		url := idm.buildURLWithParams(form.Action, params)
		if url != "" {
			urls = append(urls, url)
		}
	}

	return urls
}

// buildURLWithParams constrói URL com parâmetros
func (idm *InteractiveDiscoveryManager) buildURLWithParams(baseURL string, params map[string]string) string {
	if len(params) == 0 {
		return baseURL
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	q := u.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	return u.String()
}

// resolveURL resolve URL relativa para absoluta
func (idm *InteractiveDiscoveryManager) resolveURL(href, baseURL string) string {
	if href == "" {
		return ""
	}

	// Se já é absoluta
	if strings.HasPrefix(href, "http") {
		return href
	}

	// Resolver URL relativa
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	resolved, err := base.Parse(href)
	if err != nil {
		return ""
	}

	return resolved.String()
}

// generateURLFromButtonText gera URLs baseadas no texto do botão
func (idm *InteractiveDiscoveryManager) generateURLFromButtonText(buttonText, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	
	// Mapear textos de botões para URLs comuns
	buttonMappings := map[string]string{
		"buscar":           "/busca",
		"pesquisar":        "/pesquisa", 
		"search":           "/search",
		"ver todos":        "/imoveis",
		"todos os imóveis": "/imoveis",
		"buscar imóveis":   "/busca-imoveis",
		"comprar":          "/venda",
		"alugar":           "/aluguel",
		"venda":            "/venda",
		"aluguel":          "/aluguel",
		"apartamentos":     "/venda/apartamento",
		"casas":            "/venda/casa",
		"terrenos":         "/venda/terreno",
		"imóveis":          "/imoveis",
	}
	
	// Procurar por correspondências
	for keyword, path := range buttonMappings {
		if strings.Contains(buttonText, keyword) {
			resolved, err := base.Parse(path)
			if err == nil {
				return resolved.String()
			}
		}
	}
	
	// Se não encontrou correspondência, tentar URLs genéricas
	genericPaths := []string{
		"/busca",
		"/pesquisa", 
		"/imoveis",
		"/venda",
		"/aluguel",
	}
	
	for _, path := range genericPaths {
		resolved, err := base.Parse(path)
		if err == nil {
			return resolved.String()
		}
	}
	
	return ""
}

// generateKnownCatalogURLs gera URLs de catálogos conhecidos para sites brasileiros
func (idm *InteractiveDiscoveryManager) generateKnownCatalogURLs(baseURL string) []string {
	var urls []string
	
	base, err := url.Parse(baseURL)
	if err != nil {
		return urls
	}
	
	// URLs de catálogos comuns em sites brasileiros de imóveis
	catalogPaths := []string{
		// Padrões gerais
		"/venda/imovel/brasil/todos-os-bairros",
		"/aluguel/imovel/brasil/todos-os-bairros", 
		"/busca/imoveis",
		"/pesquisa/imoveis",
		"/imoveis/venda",
		"/imoveis/aluguel",
		"/venda/todos",
		"/aluguel/todos",
		
		// Padrões por tipo
		"/venda/apartamento",
		"/venda/casa", 
		"/venda/terreno",
		"/venda/comercial",
		"/aluguel/apartamento",
		"/aluguel/casa",
		"/aluguel/comercial",
		
		// Padrões com filtros
		"/busca?operacao=venda",
		"/busca?operacao=aluguel",
		"/imoveis?tipo=todos&operacao=venda",
		"/pesquisa?cidade=todos&operacao=venda",
		
		// Padrões específicos observados
		"/comprar/imoveis",
		"/alugar/imoveis",
		"/resultados/venda",
		"/resultados/aluguel",
		"/listagem/imoveis",
		"/catalogo/imoveis",
	}
	
	// Gerar URLs absolutas
	for _, path := range catalogPaths {
		resolved, err := base.Parse(path)
		if err == nil {
			urls = append(urls, resolved.String())
		}
	}
	
	return urls
}

// ExecuteInteractiveDiscovery executa descoberta interativa e visita URLs descobertas
func (idm *InteractiveDiscoveryManager) ExecuteInteractiveDiscovery(collector *colly.Collector, doc *goquery.Document, currentURL string) {
	result := idm.DiscoverInteractiveElements(doc, currentURL)
	
	// Visitar URLs descobertas
	for _, discoveredURL := range result.DiscoveredURLs {
		if !idm.visitedURLs[discoveredURL] {
			idm.visitedURLs[discoveredURL] = true
			idm.logger.WithFields(map[string]interface{}{
				"discovered_url": discoveredURL,
				"source_url":     currentURL,
			}).Info("Visiting discovered URL from interactive elements")
			collector.Visit(discoveredURL)
		}
	}
}
