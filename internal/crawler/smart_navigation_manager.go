package crawler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gocolly/colly"
)

// SmartNavigationManager gerencia navegação inteligente para descobrir anúncios
type SmartNavigationManager struct {
	logger             *logger.Logger
	visitedURLs        map[string]bool
	catalogURLs        map[string]bool
	maxPagesPerSite    int
	maxDepth           int
	interactiveManager *InteractiveDiscoveryManager
}

// NavigationResult resultado da análise de navegação
type NavigationResult struct {
	IsCatalogPage   bool     `json:"is_catalog_page"`
	IsPropertyPage  bool     `json:"is_property_page"`
	PropertyLinks   []string `json:"property_links"`
	PaginationLinks []string `json:"pagination_links"`
	CatalogLinks    []string `json:"catalog_links"`
	Confidence      float64  `json:"confidence"`
	Reason          string   `json:"reason"`
}

// NewSmartNavigationManager cria um novo gerenciador de navegação
func NewSmartNavigationManager() *SmartNavigationManager {
	return &SmartNavigationManager{
		logger:             logger.NewLogger("smart_navigation"),
		visitedURLs:        make(map[string]bool),
		catalogURLs:        make(map[string]bool),
		maxPagesPerSite:    50, // Máximo de páginas por site
		maxDepth:           5,  // Profundidade máxima
		interactiveManager: NewInteractiveDiscoveryManager(),
	}
}

// AnalyzePage analisa uma página para determinar tipo e extrair links
func (snm *SmartNavigationManager) AnalyzePage(doc *goquery.Document, currentURL string) NavigationResult {
	result := NavigationResult{
		PropertyLinks:   []string{},
		PaginationLinks: []string{},
		CatalogLinks:    []string{},
	}

	pageText := strings.ToLower(doc.Text())

	// 1. DETECTAR SE É PÁGINA DE CATÁLOGO
	result.IsCatalogPage = snm.isCatalogPage(currentURL, pageText, doc)

	if result.IsCatalogPage {
		snm.logger.WithField("url", currentURL).Info("Detected catalog page")

		// Extrair links de anúncios individuais do catálogo
		result.PropertyLinks = snm.extractPropertyLinksFromCatalog(doc, currentURL)

		// Extrair links de paginação
		result.PaginationLinks = snm.extractPaginationLinks(doc, currentURL)

		result.Confidence = 0.9
		result.Reason = fmt.Sprintf("Catalog page with %d properties and %d pagination links",
			len(result.PropertyLinks), len(result.PaginationLinks))

		return result
	}

	// 2. DETECTAR SE É PÁGINA DE ANÚNCIO INDIVIDUAL
	result.IsPropertyPage = snm.isIndividualPropertyPage(currentURL, pageText, doc)

	if result.IsPropertyPage {
		result.Confidence = 0.8
		result.Reason = "Individual property page detected"
		return result
	}

	// 3. PÁGINA GENÉRICA - EXTRAIR LINKS DE NAVEGAÇÃO
	result.CatalogLinks = snm.extractCatalogLinks(doc, currentURL)

	// 4. DESCOBERTA INTERATIVA - PROCURAR BOTÕES, FORMULÁRIOS E ELEMENTOS INTERATIVOS
	interactiveResult := snm.interactiveManager.DiscoverInteractiveElements(doc, currentURL)

	// Adicionar URLs descobertas aos links de catálogo
	for _, discoveredURL := range interactiveResult.DiscoveredURLs {
		if !snm.containsURL(result.CatalogLinks, discoveredURL) {
			result.CatalogLinks = append(result.CatalogLinks, discoveredURL)
		}
	}

	result.Confidence = 0.3
	if interactiveResult.TotalDiscovered > 0 {
		result.Confidence = 0.6 // Maior confiança se encontrou elementos interativos
	}

	result.Reason = fmt.Sprintf("Generic page with %d catalog links (%d from interactive discovery)",
		len(result.CatalogLinks), interactiveResult.TotalDiscovered)

	return result
}

// isCatalogPage detecta se é uma página de catálogo/listagem
func (snm *SmartNavigationManager) isCatalogPage(currentURL, pageText string, doc *goquery.Document) bool {
	urlLower := strings.ToLower(currentURL)

	// 1. PADRÕES DE URL DE CATÁLOGO
	catalogURLPatterns := []string{
		"/venda/", "/aluguel/", "/comprar/", "/alugar/",
		"/imoveis/", "/properties/", "/listings/",
		"/busca", "/search", "/pesquisa",
		"/catalogo", "/listagem", "/resultados",
		"todos-os-bairros", "todas-as-regioes",
	}

	urlMatches := 0
	for _, pattern := range catalogURLPatterns {
		if strings.Contains(urlLower, pattern) {
			urlMatches++
		}
	}

	// 2. INDICADORES DE CONTEÚDO DE CATÁLOGO
	catalogContentIndicators := []string{
		"ordenar por", "filtros", "resultados encontrados",
		"próxima página", "página anterior", "página 1",
		"nenhum imóvel foi encontrado", "realize uma nova busca",
		"mais filtros", "ver mapa", "carregando...",
	}

	contentMatches := 0
	for _, indicator := range catalogContentIndicators {
		if strings.Contains(pageText, indicator) {
			contentMatches++
		}
	}

	// 3. ESTRUTURA HTML DE CATÁLOGO
	structureScore := 0

	// Procurar por múltiplos elementos de preço (indica listagem)
	priceElements := doc.Find(".price, .valor, .preco, [class*='price'], [class*='valor']").Length()
	if priceElements > 3 {
		structureScore += 2
	}

	// Procurar por elementos de paginação
	paginationElements := doc.Find(".pagination, .paginacao, [class*='page'], [class*='pagina']").Length()
	if paginationElements > 0 {
		structureScore += 2
	}

	// Procurar por filtros
	filterElements := doc.Find(".filter, .filtro, [class*='filter'], select, input[type='range']").Length()
	if filterElements > 2 {
		structureScore += 1
	}

	// DECISÃO FINAL
	isCatalog := (urlMatches >= 1 && contentMatches >= 2) ||
		(urlMatches >= 2) ||
		(contentMatches >= 3 && structureScore >= 2)

	snm.logger.WithFields(map[string]interface{}{
		"url":             currentURL,
		"url_matches":     urlMatches,
		"content_matches": contentMatches,
		"structure_score": structureScore,
		"is_catalog":      isCatalog,
	}).Debug("Catalog page analysis")

	return isCatalog
}

// isIndividualPropertyPage detecta se é uma página de anúncio individual
func (snm *SmartNavigationManager) isIndividualPropertyPage(currentURL, pageText string, doc *goquery.Document) bool {
	// Usar o classificador rigoroso existente seria ideal aqui
	// Por enquanto, uma detecção básica

	urlLower := strings.ToLower(currentURL)

	// Padrões de URL de anúncio individual
	individualPatterns := []string{
		"/imovel/", "/propriedade/", "/detalhes/",
		"/property/", "/listing/", "/anuncio/",
	}

	hasIndividualURL := false
	for _, pattern := range individualPatterns {
		if strings.Contains(urlLower, pattern) {
			hasIndividualURL = true
			break
		}
	}

	// Padrões de ID numérico no final da URL
	hasNumericID := regexp.MustCompile(`/\d+/?$`).MatchString(currentURL)

	// Indicadores de conteúdo individual
	individualIndicators := []string{
		"código do imóvel", "ref:", "código:",
		"agendar visita", "entre em contato",
		"características do imóvel", "detalhes do imóvel",
	}

	contentMatches := 0
	for _, indicator := range individualIndicators {
		if strings.Contains(pageText, indicator) {
			contentMatches++
		}
	}

	return (hasIndividualURL || hasNumericID) && contentMatches >= 1
}

// extractPropertyLinksFromCatalog extrai links de anúncios individuais de uma página de catálogo
func (snm *SmartNavigationManager) extractPropertyLinksFromCatalog(doc *goquery.Document, baseURL string) []string {
	var links []string

	// SELETORES ESPECÍFICOS para sites brasileiros de imóveis
	propertySelectors := []string{
		// Links diretos para anúncios
		"a[href*='/imovel/']",
		"a[href*='/propriedade/']",
		"a[href*='/detalhes/']",
		"a[href*='/property/']",
		"a[href*='/listing/']",
		"a[href*='/anuncio/']",

		// Cards e containers de propriedades
		".property-card a", ".imovel-card a", ".listing-item a",
		".property-link", ".imovel-link", ".card a",
		".item a", ".resultado a", ".anuncio a",

		// Seletores genéricos para imóveis
		"[class*='property'] a", "[class*='imovel'] a",
		"[class*='listing'] a", "[class*='card'] a",
		"[class*='item'] a", "[class*='resultado'] a",

		// Containers de resultados
		".results a", ".resultados a", ".listings a",
		".properties a", ".imoveis a",
	}

	// Tentar seletores específicos primeiro
	for _, selector := range propertySelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists {
				return
			}

			absoluteURL := snm.resolveURL(href, baseURL)
			if absoluteURL != "" && !snm.visitedURLs[absoluteURL] && snm.looksLikePropertyURL(absoluteURL) {
				links = append(links, absoluteURL)
			}
		})
	}

	// BUSCA MAIS AMPLA: Se não encontrou com seletores específicos
	if len(links) == 0 {
		snm.logger.WithField("base_url", baseURL).Debug("No links found with specific selectors, trying broader search")

		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			absoluteURL := snm.resolveURL(href, baseURL)

			if snm.looksLikePropertyURL(absoluteURL) && !snm.visitedURLs[absoluteURL] {
				links = append(links, absoluteURL)
			}
		})
	}

	// BUSCA AINDA MAIS AMPLA: Procurar por qualquer link interno com ID numérico
	if len(links) == 0 {
		snm.logger.WithField("base_url", baseURL).Debug("Still no links found, trying numeric ID pattern search")

		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			absoluteURL := snm.resolveURL(href, baseURL)

			// Aceitar qualquer link interno com padrão numérico ou palavras-chave
			if absoluteURL != "" &&
				strings.Contains(absoluteURL, baseURL) &&
				(regexp.MustCompile(`/\d+/?$`).MatchString(absoluteURL) ||
					strings.Contains(absoluteURL, "/imovel") ||
					strings.Contains(absoluteURL, "/propriedade") ||
					strings.Contains(absoluteURL, "/detalhes")) &&
				!snm.visitedURLs[absoluteURL] {
				links = append(links, absoluteURL)
			}
		})
	}

	// Remover duplicatas
	uniqueLinks := make(map[string]bool)
	var finalLinks []string
	for _, link := range links {
		if !uniqueLinks[link] {
			uniqueLinks[link] = true
			finalLinks = append(finalLinks, link)
		}
	}

	// Limitar número de links para evitar sobrecarga
	if len(finalLinks) > 30 {
		finalLinks = finalLinks[:30]
	}

	snm.logger.WithFields(map[string]interface{}{
		"base_url":       baseURL,
		"property_links": len(finalLinks),
		"total_found":    len(links),
		"unique_links":   len(uniqueLinks),
	}).Info("Extracted property links from catalog")

	return finalLinks
}

// extractPaginationLinks extrai links de paginação
func (snm *SmartNavigationManager) extractPaginationLinks(doc *goquery.Document, baseURL string) []string {
	var links []string

	// Seletores para paginação
	paginationSelectors := []string{
		".pagination a", ".paginacao a", ".page-numbers a",
		"a[href*='page=']", "a[href*='pagina=']", "a[href*='p=']",
		".next", ".proximo", ".anterior", ".previous",
	}

	for _, selector := range paginationSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists {
				return
			}

			text := strings.ToLower(strings.TrimSpace(s.Text()))

			// Filtrar apenas links relevantes de paginação
			if strings.Contains(text, "próxima") ||
				strings.Contains(text, "next") ||
				strings.Contains(text, "página") ||
				regexp.MustCompile(`^\d+$`).MatchString(text) {

				absoluteURL := snm.resolveURL(href, baseURL)
				if absoluteURL != "" && !snm.visitedURLs[absoluteURL] {
					links = append(links, absoluteURL)
				}
			}
		})
	}

	// Limitar paginação para evitar loops infinitos
	if len(links) > 5 {
		links = links[:5]
	}

	return links
}

// extractCatalogLinks extrai links para páginas de catálogo
func (snm *SmartNavigationManager) extractCatalogLinks(doc *goquery.Document, baseURL string) []string {
	var links []string

	// Procurar por links que levam a catálogos
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := strings.ToLower(strings.TrimSpace(s.Text()))

		// Textos que indicam links para catálogos
		catalogTexts := []string{
			"ver todos", "todos os imóveis", "buscar imóveis",
			"comprar", "alugar", "venda", "aluguel",
			"apartamentos", "casas", "terrenos",
		}

		isCatalogText := false
		for _, catalogText := range catalogTexts {
			if strings.Contains(text, catalogText) {
				isCatalogText = true
				break
			}
		}

		if isCatalogText {
			absoluteURL := snm.resolveURL(href, baseURL)
			if absoluteURL != "" && !snm.visitedURLs[absoluteURL] {
				links = append(links, absoluteURL)
			}
		}
	})

	return links
}

// looksLikePropertyURL verifica se uma URL parece ser de um anúncio individual
func (snm *SmartNavigationManager) looksLikePropertyURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	urlLower := strings.ToLower(urlStr)

	// Padrões positivos EXPANDIDOS
	positivePatterns := []string{
		"/imovel/", "/propriedade/", "/detalhes/",
		"/property/", "/listing/", "/anuncio/",
		"/casa/", "/apartamento/", "/terreno/",
		"/comercial/", "/rural/", "/loja/",
		"/publicacao.aspx", "/comprar/", "/alugar/",
	}

	for _, pattern := range positivePatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}

	// Padrão de ID numérico no final
	if regexp.MustCompile(`/\d+/?$`).MatchString(urlStr) {
		return true
	}

	// Padrões com IDs específicos
	if regexp.MustCompile(`[?&]id=\d+`).MatchString(urlLower) ||
		regexp.MustCompile(`[?&]imovel=\d+`).MatchString(urlLower) ||
		regexp.MustCompile(`[?&]codigo=\w+`).MatchString(urlLower) {
		return true
	}

	return false
}

// resolveURL resolve URL relativa para absoluta
func (snm *SmartNavigationManager) resolveURL(href, baseURL string) string {
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

// NavigateFromCatalog navega a partir de uma página de catálogo
func (snm *SmartNavigationManager) NavigateFromCatalog(collector *colly.Collector, catalogURL string, result NavigationResult) {
	snm.catalogURLs[catalogURL] = true

	// Visitar links de anúncios individuais
	for _, propertyURL := range result.PropertyLinks {
		if !snm.visitedURLs[propertyURL] {
			snm.visitedURLs[propertyURL] = true
			snm.logger.WithField("property_url", propertyURL).Debug("Visiting property from catalog")
			collector.Visit(propertyURL)
		}
	}

	// Visitar páginas de paginação (limitado)
	visitedPages := 0
	for _, paginationURL := range result.PaginationLinks {
		if !snm.visitedURLs[paginationURL] && visitedPages < 3 {
			snm.visitedURLs[paginationURL] = true
			visitedPages++
			snm.logger.WithField("pagination_url", paginationURL).Debug("Visiting pagination page")
			collector.Visit(paginationURL)
		}
	}
}

// NavigateFromGeneric navega a partir de uma página genérica
func (snm *SmartNavigationManager) NavigateFromGeneric(collector *colly.Collector, genericURL string, result NavigationResult) {
	// Visitar links de catálogo encontrados
	for _, catalogURL := range result.CatalogLinks {
		if !snm.catalogURLs[catalogURL] && !snm.visitedURLs[catalogURL] {
			snm.visitedURLs[catalogURL] = true
			snm.logger.WithField("catalog_url", catalogURL).Debug("Visiting catalog from generic page")
			collector.Visit(catalogURL)
		}
	}
}

// IsVisited verifica se uma URL já foi visitada
func (snm *SmartNavigationManager) IsVisited(url string) bool {
	return snm.visitedURLs[url]
}

// MarkVisited marca uma URL como visitada
func (snm *SmartNavigationManager) MarkVisited(url string) {
	snm.visitedURLs[url] = true
}

// GetStats retorna estatísticas de navegação
func (snm *SmartNavigationManager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"visited_urls": len(snm.visitedURLs),
		"catalog_urls": len(snm.catalogURLs),
		"max_pages":    snm.maxPagesPerSite,
	}
}

// containsURL verifica se uma URL já está na lista
func (snm *SmartNavigationManager) containsURL(urls []string, targetURL string) bool {
	for _, url := range urls {
		if url == targetURL {
			return true
		}
	}
	return false
}
