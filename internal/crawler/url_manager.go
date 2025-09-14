package crawler

import (
	"regexp"
	"strings"
	"sync"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gocolly/colly"
)

// URLManager gerencia URLs visitadas e extrai novos links
type URLManager struct {
	visitedURLs map[string]bool
	mutex       sync.RWMutex
	logger      *logger.Logger
}

// NewURLManager cria um novo gerenciador de URLs
func NewURLManager() *URLManager {
	return &URLManager{
		visitedURLs: make(map[string]bool),
		logger:      logger.NewLogger("url_manager"),
	}
}

// IsVisited verifica se uma URL já foi visitada
func (um *URLManager) IsVisited(url string) bool {
	um.mutex.RLock()
	defer um.mutex.RUnlock()
	return um.visitedURLs[url]
}

// MarkVisited marca uma URL como visitada
func (um *URLManager) MarkVisited(url string) {
	um.mutex.Lock()
	defer um.mutex.Unlock()
	um.visitedURLs[url] = true
}

// GetVisitedCount retorna o número de URLs visitadas
func (um *URLManager) GetVisitedCount() int {
	um.mutex.RLock()
	defer um.mutex.RUnlock()
	return len(um.visitedURLs)
}

// ExtractPropertyLinks extrai links de propriedades de uma página
func (um *URLManager) ExtractPropertyLinks(e *colly.HTMLElement) []string {
	var links []string
	baseURL := e.Request.URL

	e.ForEach("a[href]", func(_ int, el *colly.HTMLElement) {
		href := el.Attr("href")
		absoluteLink := e.Request.AbsoluteURL(href)

		// Valida se é um link válido
		if !um.isValidPropertyLink(absoluteLink, baseURL.Host) {
			return
		}

		// Verifica se já foi visitado
		if !um.IsVisited(absoluteLink) {
			links = append(links, absoluteLink)
		}
	})

	um.logger.WithFields(map[string]interface{}{
		"source_url":    baseURL.String(),
		"links_found":   len(links),
		"total_visited": um.GetVisitedCount(),
	}).Debug("Property links extracted")

	return links
}

// isValidPropertyLink verifica se um link é válido para uma propriedade
func (um *URLManager) isValidPropertyLink(link, host string) bool {
	// Ignora links vazios
	if link == "" {
		return false
	}

	// Ignora links externos
	if !strings.Contains(link, host) {
		return false
	}

	// Ignora links de redes sociais e contato
	socialPatterns := []string{
		"wa.me", "whatsapp", "mailto:", "tel:", "facebook", "instagram", "twitter",
	}

	linkLower := strings.ToLower(link)
	for _, pattern := range socialPatterns {
		if strings.Contains(linkLower, pattern) {
			return false
		}
	}

	// Verifica se parece ser um link de propriedade
	propertyPatterns := []string{
		"detalhes", "imovel/", "propriedade/", "anuncio/",
		"/VENDAS/", "/ALUGUEIS/", "/casa/", "/apartamento/",
	}

	for _, pattern := range propertyPatterns {
		if strings.Contains(link, pattern) {
			return true
		}
	}

	// Verifica se termina com número (padrão comum para IDs de propriedades)
	numberEndRegex := regexp.MustCompile(`/\d+/?$`)
	if numberEndRegex.MatchString(link) {
		return true
	}

	return false
}

// IsCatalogPage verifica se uma página é um catálogo/listagem
func (um *URLManager) IsCatalogPage(e *colly.HTMLElement) bool {
	url := e.Request.URL.String()
	text := strings.ToLower(e.Text)

	// Indicadores de página de catálogo
	catalogIndicators := []string{
		"imoveis/?", "busca", "listagem", "catalogo", "pesquisa",
		"resultados", "filtros", "ordenar", "pagina=", "page=",
	}

	indicatorCount := 0
	for _, indicator := range catalogIndicators {
		if strings.Contains(url, indicator) {
			indicatorCount++
		}
	}

	// Verifica se tem múltiplos preços (indicativo de listagem)
	priceCount := strings.Count(text, "r$")
	roomCount := strings.Count(text, "quartos")

	// É catálogo se tem indicadores E múltiplos itens
	isCatalog := indicatorCount > 0 && (priceCount > 5 || roomCount > 3)

	if isCatalog {
		um.logger.WithFields(map[string]interface{}{
			"url":             url,
			"indicator_count": indicatorCount,
			"price_count":     priceCount,
			"room_count":      roomCount,
		}).Debug("Catalog page detected")
	}

	return isCatalog
}

// ShouldSkipURL verifica se uma URL deve ser ignorada
func (um *URLManager) ShouldSkipURL(url string) bool {
	// URLs que devem ser ignoradas
	skipPatterns := []string{
		"javascript:", "#", "mailto:", "tel:",
		".pdf", ".jpg", ".png", ".gif", ".css", ".js",
		"login", "admin", "wp-admin", "cadastro",
	}

	urlLower := strings.ToLower(url)
	for _, pattern := range skipPatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}

	return false
}

// CleanupOldURLs remove URLs antigas do cache (para evitar memory leak)
func (um *URLManager) CleanupOldURLs(maxURLs int) {
	um.mutex.Lock()
	defer um.mutex.Unlock()

	if len(um.visitedURLs) <= maxURLs {
		return
	}

	// Remove URLs mais antigas (implementação simples - remove metade)
	count := 0
	toRemove := len(um.visitedURLs) / 2

	for url := range um.visitedURLs {
		if count >= toRemove {
			break
		}
		delete(um.visitedURLs, url)
		count++
	}

	um.logger.WithFields(map[string]interface{}{
		"removed":   count,
		"remaining": len(um.visitedURLs),
	}).Info("Cleaned up old URLs from cache")
}
