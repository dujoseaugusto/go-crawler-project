package crawler

import (
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
		if !um.IsValidPropertyLink(absoluteLink, baseURL.Host) {
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

// IsValidPropertyLink verifica se um link é válido para uma propriedade
func (um *URLManager) IsValidPropertyLink(link, host string) bool {
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
		"javascript:", "#", "void(0)", ".pdf", ".jpg", ".png", ".gif", ".css", ".js",
	}

	linkLower := strings.ToLower(link)
	for _, pattern := range socialPatterns {
		if strings.Contains(linkLower, pattern) {
			return false
		}
	}

	// SISTEMA SIMPLIFICADO - Aceita TODOS os links internos válidos
	// Isso permite navegação profunda para encontrar anúncios específicos
	return true
}

// IsCatalogPage verifica se uma página é um catálogo/listagem
func (um *URLManager) IsCatalogPage(e *colly.HTMLElement) bool {
	// SISTEMA DE CATÁLOGO REMOVIDO COMPLETAMENTE
	// Todas as páginas são tratadas como propriedades potenciais
	return false
}

// IsPropertyPage verifica se uma página é um anúncio individual
func (um *URLManager) IsPropertyPage(e *colly.HTMLElement) bool {
	// SISTEMA SIMPLIFICADO - Todas as páginas são tratadas como propriedades potenciais
	return true
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
