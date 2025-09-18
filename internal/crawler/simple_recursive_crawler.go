package crawler

import (
	"context"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
)

// SimpleRecursiveCrawler implementa crawling recursivo simples
type SimpleRecursiveCrawler struct {
	repository        repository.PropertyRepository
	urlRepo           repository.URLRepository
	preciseClassifier *PrecisePropertyClassifier
	extractor         *DataExtractor
	validator         *PropertyValidator
	logger            *logger.Logger
	visitedURLs       map[string]bool
	maxDepth          int
	currentDepth      map[string]int
}

// NewSimpleRecursiveCrawler cria um novo crawler recursivo simples
func NewSimpleRecursiveCrawler(
	propertyRepo repository.PropertyRepository,
	urlRepo repository.URLRepository,
) *SimpleRecursiveCrawler {
	return &SimpleRecursiveCrawler{
		repository:        propertyRepo,
		urlRepo:           urlRepo,
		preciseClassifier: NewPrecisePropertyClassifier(),
		extractor:         NewDataExtractor(),
		validator:         NewPropertyValidator(),
		logger:            logger.NewLogger("simple_recursive_crawler"),
		visitedURLs:       make(map[string]bool),
		maxDepth:          15, // Limite de 15 níveis para encontrar mais anúncios
		currentDepth:      make(map[string]int),
	}
}

// Start inicia o crawling recursivo simples
func (src *SimpleRecursiveCrawler) Start(ctx context.Context, urls []string) error {
	src.logger.WithFields(map[string]interface{}{
		"total_urls": len(urls),
		"max_depth":  src.maxDepth,
	}).Info("Starting simple recursive crawling")

	// Configurar collector
	collector := src.setupCollector(ctx)

	// Iniciar crawling para cada URL base
	for _, url := range urls {
		src.currentDepth[url] = 0 // Profundidade inicial = 0
		collector.Visit(url)
	}

	// Aguardar conclusão
	collector.Wait()

	src.logger.WithFields(map[string]interface{}{
		"visited_urls": len(src.visitedURLs),
	}).Info("Simple recursive crawling completed")

	return nil
}

// setupCollector configura o collector do Colly
func (src *SimpleRecursiveCrawler) setupCollector(ctx context.Context) *colly.Collector {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
	)

	// Configurações de performance
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       2 * time.Second,
	})

	// Handler principal - FLUXO RECURSIVO SIMPLES
	c.OnHTML("html", func(e *colly.HTMLElement) {
		src.handlePage(ctx, e, c)
	})

	// Handler para requisições
	c.OnRequest(func(r *colly.Request) {
		src.logger.WithField("url", r.URL.String()).Debug("Visiting URL")
	})

	// Handler para erros
	c.OnError(func(r *colly.Response, err error) {
		src.logger.WithFields(map[string]interface{}{
			"url":         r.Request.URL.String(),
			"status_code": r.StatusCode,
		}).Error("Request failed", err)
	})

	return c
}

// handlePage implementa o FLUXO RECURSIVO SIMPLES
func (src *SimpleRecursiveCrawler) handlePage(ctx context.Context, e *colly.HTMLElement, collector *colly.Collector) {
	url := e.Request.URL.String()

	// Verificar se já foi visitada
	if src.visitedURLs[url] {
		return
	}
	src.visitedURLs[url] = true

	// Verificar profundidade máxima
	depth := src.getCurrentDepth(url)
	if depth > src.maxDepth {
		src.logger.WithFields(map[string]interface{}{
			"url":   url,
			"depth": depth,
		}).Debug("Maximum depth reached, skipping")
		return
	}

	src.logger.WithFields(map[string]interface{}{
		"url":   url,
		"depth": depth,
	}).Info("Processing page")

	// PASSO 1: ANALISAR SE É PÁGINA DE ANÚNCIO
	doc := &goquery.Document{Selection: e.DOM}
	classificationResult := src.preciseClassifier.ClassifyPage(doc, url)

	src.logger.WithFields(map[string]interface{}{
		"url":                    url,
		"is_individual_property": classificationResult.IsIndividualProperty,
		"confidence":             classificationResult.Confidence,
		"score":                  classificationResult.Score,
		"reason":                 classificationResult.Reason,
	}).Info("Page classification completed")

	// PASSO 2: SE É ANÚNCIO → SALVAR NO BANCO
	if classificationResult.IsIndividualProperty {
		src.logger.WithField("url", url).Info("Property page detected - extracting data")
		src.extractAndSaveProperty(ctx, e, url)
		return // Não precisa explorar links de uma página de anúncio
	}

	// PASSO 3: SE NÃO É ANÚNCIO → EXPLORAR TODOS OS LINKS CLICÁVEIS
	src.logger.WithField("url", url).Info("Not a property page - exploring all clickable elements")
	clickableLinks := src.extractAllClickableLinks(e, url)

	src.logger.WithFields(map[string]interface{}{
		"url":             url,
		"clickable_links": len(clickableLinks),
		"depth":           depth,
	}).Info("Found clickable elements")

	// PASSO 4: PRIORIZAR LINKS RELEVANTES E EVITAR PÁGINAS INSTITUCIONAIS
	priorityLinks := []string{}
	normalLinks := []string{}

	for _, link := range clickableLinks {
		if !src.visitedURLs[link] && src.isValidLink(link, url) {
			// Evitar páginas de baixa prioridade em profundidades altas
			if depth >= 4 && src.isLowPriorityLink(link) {
				continue
			}

			if src.isPriorityLink(link) {
				priorityLinks = append(priorityLinks, link)
			} else {
				normalLinks = append(normalLinks, link)
			}
		}
	}

	// Processar primeiro os links de alta prioridade
	allLinksToProcess := append(priorityLinks, normalLinks...)

	// Limitar número de links por profundidade para evitar explosão
	maxLinksPerDepth := 50
	if depth >= 6 {
		maxLinksPerDepth = 20
	}
	if depth >= 8 {
		maxLinksPerDepth = 10
	}

	if len(allLinksToProcess) > maxLinksPerDepth {
		allLinksToProcess = allLinksToProcess[:maxLinksPerDepth]
	}

	src.logger.WithFields(map[string]interface{}{
		"url":            url,
		"priority_links": len(priorityLinks),
		"normal_links":   len(normalLinks),
		"processing":     len(allLinksToProcess),
		"depth":          depth,
	}).Info("Prioritizing links for navigation")

	// PASSO 5: PARA CADA LINK → REPETIR O FLUXO (RECURSIVO)
	for _, link := range allLinksToProcess {
		// Definir profundidade do próximo nível
		src.currentDepth[link] = depth + 1

		src.logger.WithFields(map[string]interface{}{
			"parent_url":  url,
			"child_url":   link,
			"depth":       depth + 1,
			"is_priority": src.isPriorityLink(link),
		}).Debug("Visiting child URL")

		collector.Visit(link)
	}
}

// extractAllClickableLinks extrai TODOS os elementos clicáveis da página
func (src *SimpleRecursiveCrawler) extractAllClickableLinks(e *colly.HTMLElement, baseURL string) []string {
	var links []string
	linkSet := make(map[string]bool)

	// 1. TODOS OS LINKS (a[href])
	e.ForEach("a[href]", func(_ int, el *colly.HTMLElement) {
		href := el.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(href)
		if absoluteURL != "" && !linkSet[absoluteURL] {
			links = append(links, absoluteURL)
			linkSet[absoluteURL] = true
		}
	})

	// 2. TODOS OS BOTÕES COM ONCLICK OU FORMULÁRIOS
	e.ForEach("button[onclick], input[onclick], form", func(_ int, el *colly.HTMLElement) {
		// Para formulários, tentar extrair action
		if el.Name == "form" {
			action := el.Attr("action")
			if action != "" {
				absoluteURL := e.Request.AbsoluteURL(action)
				if absoluteURL != "" && !linkSet[absoluteURL] {
					links = append(links, absoluteURL)
					linkSet[absoluteURL] = true
				}
			}
		}

		// Para elementos com onclick, tentar extrair URLs (implementação básica)
		onclick := el.Attr("onclick")
		if onclick != "" {
			extractedURL := src.extractURLFromOnclick(onclick, baseURL)
			if extractedURL != "" && !linkSet[extractedURL] {
				links = append(links, extractedURL)
				linkSet[extractedURL] = true
			}
		}
	})

	// 3. ELEMENTOS COM data-href, data-url, etc.
	e.ForEach("[data-href], [data-url], [data-link]", func(_ int, el *colly.HTMLElement) {
		dataAttrs := []string{"data-href", "data-url", "data-link"}
		for _, attr := range dataAttrs {
			value := el.Attr(attr)
			if value != "" {
				absoluteURL := e.Request.AbsoluteURL(value)
				if absoluteURL != "" && !linkSet[absoluteURL] {
					links = append(links, absoluteURL)
					linkSet[absoluteURL] = true
				}
			}
		}
	})

	return links
}

// extractURLFromOnclick extrai URL de atributos onclick (implementação básica)
func (src *SimpleRecursiveCrawler) extractURLFromOnclick(onclick, baseURL string) string {
	// Procurar por padrões comuns: window.location, location.href, etc.
	patterns := []string{
		"window.location='", "window.location=\"",
		"location.href='", "location.href=\"",
		"window.open('", "window.open(\"",
	}

	for _, pattern := range patterns {
		if strings.Contains(onclick, pattern) {
			start := strings.Index(onclick, pattern) + len(pattern)
			remaining := onclick[start:]

			var end int
			if strings.Contains(pattern, "'") {
				end = strings.Index(remaining, "'")
			} else {
				end = strings.Index(remaining, "\"")
			}

			if end > 0 {
				url := remaining[:end]
				return src.resolveURL(url, baseURL)
			}
		}
	}

	return ""
}

// resolveURL resolve URL relativa para absoluta
func (src *SimpleRecursiveCrawler) resolveURL(href, baseURL string) string {
	if href == "" {
		return ""
	}

	// Limpar href de espaços e caracteres inválidos
	href = strings.TrimSpace(href)
	if href == "" || href == "#" || strings.HasPrefix(href, "javascript:") {
		return ""
	}

	// Se já é absoluta
	if strings.HasPrefix(href, "http") {
		return href
	}

	// Extrair domínio base
	domain := src.extractDomain(baseURL)
	if domain == "" {
		return ""
	}

	// Construir URL absoluta corretamente
	if strings.HasPrefix(href, "/") {
		// URL relativa à raiz
		return "https://" + domain + href
	}

	// URL relativa ao diretório atual (evitar duplicação de barras)
	if strings.HasSuffix(baseURL, "/") {
		return baseURL + href
	}
	return baseURL + "/" + href
}

// isValidLink verifica se um link é válido para crawling
func (src *SimpleRecursiveCrawler) isValidLink(link, parentURL string) bool {
	// Ignorar links vazios
	if link == "" {
		return false
	}

	// Ignorar links externos (manter apenas mesmo domínio)
	if !strings.Contains(link, src.extractDomain(parentURL)) {
		return false
	}

	// Ignorar tipos de arquivo não relevantes
	skipExtensions := []string{
		".pdf", ".jpg", ".jpeg", ".png", ".gif", ".css", ".js",
		".zip", ".rar", ".doc", ".docx", ".xls", ".xlsx",
	}

	linkLower := strings.ToLower(link)
	for _, ext := range skipExtensions {
		if strings.HasSuffix(linkLower, ext) {
			return false
		}
	}

	// Ignorar links de redes sociais e contato
	skipPatterns := []string{
		"javascript:", "mailto:", "tel:", "#",
		"facebook.com", "instagram.com", "twitter.com", "whatsapp",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(linkLower, pattern) {
			return false
		}
	}

	return true
}

// isPriorityLink verifica se um link tem prioridade para navegação (pode levar a anúncios)
func (src *SimpleRecursiveCrawler) isPriorityLink(href string) bool {
	lowerHref := strings.ToLower(href)

	// Links de alta prioridade (provavelmente levam a anúncios)
	priorityPatterns := []string{
		"/venda", "/aluguel", "/comprar", "/alugar",
		"/imovel", "/imoveis", "/propriedade", "/propriedades",
		"/apartamento", "/casa", "/terreno", "/sitio", "/chacara",
		"/buscar", "/pesquisar", "/catalogo", "/listagem",
		"/resultado", "/filtro", "id=", "/detalhes",
	}

	for _, pattern := range priorityPatterns {
		if strings.Contains(lowerHref, pattern) {
			return true
		}
	}

	return false
}

// isLowPriorityLink verifica se um link tem baixa prioridade (páginas institucionais)
func (src *SimpleRecursiveCrawler) isLowPriorityLink(href string) bool {
	lowerHref := strings.ToLower(href)

	// Links de baixa prioridade (páginas institucionais)
	lowPriorityPatterns := []string{
		"/contato", "/sobre", "/empresa", "/equipe", "/historia",
		"/politica", "/privacidade", "/termos", "/condicoes",
		"/integre", "/anuncie", "/cadastr", "/login", "/admin",
		"/ajuda", "/faq", "/suporte", "/blog", "/noticias",
	}

	for _, pattern := range lowPriorityPatterns {
		if strings.Contains(lowerHref, pattern) {
			return true
		}
	}

	return false
}

// extractDomain extrai o domínio de uma URL
func (src *SimpleRecursiveCrawler) extractDomain(url string) string {
	if strings.HasPrefix(url, "http://") {
		url = url[7:]
	} else if strings.HasPrefix(url, "https://") {
		url = url[8:]
	}

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[0]
	}

	return url
}

// getCurrentDepth obtém a profundidade atual de uma URL
func (src *SimpleRecursiveCrawler) getCurrentDepth(url string) int {
	if depth, exists := src.currentDepth[url]; exists {
		return depth
	}
	return 0
}

// extractAndSaveProperty extrai dados da propriedade e salva no banco
func (src *SimpleRecursiveCrawler) extractAndSaveProperty(ctx context.Context, e *colly.HTMLElement, url string) {
	// Usar o extrator existente
	property := src.extractor.ExtractProperty(e, url)
	if property == nil {
		src.logger.WithField("url", url).Warn("Failed to extract property data")
		return
	}

	// Validar propriedade
	if !src.validator.IsValidForSaving(property) {
		src.logger.WithField("url", url).Warn("Invalid property data extracted")
		return
	}

	// Salvar no banco
	err := src.repository.Save(ctx, *property)
	if err != nil {
		src.logger.WithFields(map[string]interface{}{
			"url": url,
		}).Error("Failed to save property", err)
		return
	}

	src.logger.WithFields(map[string]interface{}{
		"url":         url,
		"description": property.Descricao,
		"value":       property.Valor,
		"address":     property.Endereco,
	}).Info("Property saved successfully")
}
