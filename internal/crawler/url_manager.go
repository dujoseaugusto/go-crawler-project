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
	visitedURLs        map[string]bool
	mutex              sync.RWMutex
	logger             *logger.Logger
	pageClassifier     *PageClassifier
	advancedClassifier *AdvancedPageClassifier
	patternLearner     *PatternLearner
	contentLearner     *ContentBasedPatternLearner
}

// NewURLManager cria um novo gerenciador de URLs
func NewURLManager() *URLManager {
	return &URLManager{
		visitedURLs:        make(map[string]bool),
		logger:             logger.NewLogger("url_manager"),
		pageClassifier:     NewPageClassifier(),
		advancedClassifier: NewAdvancedPageClassifier(),
		patternLearner:     NewPatternLearner(),
		contentLearner:     NewContentBasedPatternLearner(),
	}
}

// NewURLManagerWithPatternLearner cria um novo gerenciador de URLs com PatternLearner treinado
func NewURLManagerWithPatternLearner(trainedPatternLearner *PatternLearner) *URLManager {
	return &URLManager{
		visitedURLs:        make(map[string]bool),
		logger:             logger.NewLogger("url_manager"),
		pageClassifier:     NewPageClassifier(),
		advancedClassifier: NewAdvancedPageClassifier(),
		patternLearner:     trainedPatternLearner,
		contentLearner:     NewContentBasedPatternLearner(),
	}
}

// NewURLManagerWithContentLearner cria um novo gerenciador com ContentBasedPatternLearner treinado
func NewURLManagerWithContentLearner(trainedContentLearner *ContentBasedPatternLearner) *URLManager {
	return &URLManager{
		visitedURLs:        make(map[string]bool),
		logger:             logger.NewLogger("url_manager"),
		pageClassifier:     NewPageClassifier(),
		advancedClassifier: NewAdvancedPageClassifier(),
		patternLearner:     NewPatternLearner(),
		contentLearner:     trainedContentLearner,
	}
}

// SetPatternLearner define um PatternLearner treinado
func (um *URLManager) SetPatternLearner(trainedPatternLearner *PatternLearner) {
	um.mutex.Lock()
	defer um.mutex.Unlock()
	um.patternLearner = trainedPatternLearner
}

// SetContentLearner define um ContentBasedPatternLearner treinado
func (um *URLManager) SetContentLearner(trainedContentLearner *ContentBasedPatternLearner) {
	um.mutex.Lock()
	defer um.mutex.Unlock()
	um.contentLearner = trainedContentLearner
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

// IsCatalogPage verifica se uma página é um catálogo/listagem usando análise de conteúdo inteligente
func (um *URLManager) IsCatalogPage(e *colly.HTMLElement) bool {
	url := e.Request.URL.String()

	// PRIMEIRO: Usa o ContentBasedPatternLearner (MAIS INTELIGENTE - analisa conteúdo real)
	if um.contentLearner != nil {
		contentType, confidence := um.contentLearner.ClassifyPageContent(e)

		if contentType == "catalog" && confidence > 0.1 {
			um.logger.WithFields(map[string]interface{}{
				"url":        url,
				"confidence": confidence,
				"method":     "content_analysis",
			}).Info("CATALOG page detected by CONTENT ANALYSIS - SKIPPING data extraction")
			return true
		}

		// Se o sistema de conteúdo detectou como propriedade, confia nele
		if contentType == "property" && confidence > 0.1 {
			um.logger.WithFields(map[string]interface{}{
				"url":        url,
				"confidence": confidence,
				"method":     "content_analysis",
			}).Debug("PROPERTY page confirmed by CONTENT ANALYSIS - continuing")
			return false
		}
	}

	// SEGUNDO: Fallback para PatternLearner baseado em URL (menos confiável)
	if um.patternLearner != nil {
		urlType, confidence := um.patternLearner.ClassifyURL(url)

		if urlType == "catalog" && confidence > 0.1 {
			um.logger.WithFields(map[string]interface{}{
				"url":        url,
				"confidence": confidence,
				"method":     "url_pattern_learner",
			}).Info("CATALOG page detected by URL PATTERN - SKIPPING data extraction")
			return true
		}
	}

	// TERCEIRO: Fallback para classificador rigoroso (último recurso)
	isCatalog := um.advancedClassifier.IsCatalogPageStrict(e)
	if isCatalog {
		um.logger.WithFields(map[string]interface{}{
			"url":    url,
			"method": "advanced_classifier",
		}).Info("CATALOG page detected by ADVANCED classifier - SKIPPING data extraction")
		return true
	}

	return false
}

// IsPropertyPage verifica se uma página é um anúncio individual usando análise de conteúdo inteligente
func (um *URLManager) IsPropertyPage(e *colly.HTMLElement) bool {
	url := e.Request.URL.String()

	// PRIMEIRO: Usa o ContentBasedPatternLearner (MAIS INTELIGENTE - analisa conteúdo real)
	if um.contentLearner != nil {
		contentType, confidence := um.contentLearner.ClassifyPageContent(e)

		if contentType == "property" && confidence > 0.1 {
			um.logger.WithFields(map[string]interface{}{
				"url":        url,
				"confidence": confidence,
				"method":     "content_analysis",
			}).Info("PROPERTY page detected by CONTENT ANALYSIS - PROCESSING data")
			return true
		}

		// Se o sistema de conteúdo detectou como catálogo, definitivamente NÃO é property
		if contentType == "catalog" && confidence > 0.1 {
			um.logger.WithFields(map[string]interface{}{
				"url":        url,
				"confidence": confidence,
				"method":     "content_analysis",
			}).Debug("Page is CATALOG by CONTENT ANALYSIS - NOT a property page")
			return false
		}
	}

	// SEGUNDO: Fallback para PatternLearner baseado em URL
	if um.patternLearner != nil {
		urlType, confidence := um.patternLearner.ClassifyURL(url)

		if urlType == "property" && confidence > 0.1 {
			um.logger.WithFields(map[string]interface{}{
				"url":        url,
				"confidence": confidence,
				"method":     "url_pattern_learner",
			}).Info("PROPERTY page detected by URL PATTERN - PROCESSING data")
			return true
		}

		// Se URL pattern detectou como catálogo, definitivamente NÃO é property
		if urlType == "catalog" && confidence > 0.1 {
			um.logger.WithFields(map[string]interface{}{
				"url":        url,
				"confidence": confidence,
				"method":     "url_pattern_learner",
			}).Debug("Page is CATALOG by URL PATTERN - NOT a property page")
			return false
		}
	}

	// TERCEIRO: Fallback para classificador rigoroso
	isProperty := um.advancedClassifier.IsPropertyPageStrict(e)
	if isProperty {
		um.logger.WithFields(map[string]interface{}{
			"url":    url,
			"method": "advanced_classifier",
		}).Info("PROPERTY page detected by ADVANCED classifier - PROCESSING data")
		return true
	}

	// QUARTO: Se não tem certeza, assume que NÃO é property (mais conservador para evitar catálogos)
	um.logger.WithFields(map[string]interface{}{
		"url": url,
	}).Debug("Page classification UNCERTAIN - SKIPPING to avoid catalog pages")

	return false
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
