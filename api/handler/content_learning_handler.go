package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly"
)

// ContentLearningHandler gerencia o aprendizado baseado em conteúdo
type ContentLearningHandler struct {
	contentLearner *crawler.ContentBasedPatternLearner
	logger         *logger.Logger
}

// NewContentLearningHandler cria um novo handler para aprendizado de conteúdo
func NewContentLearningHandler(contentLearner *crawler.ContentBasedPatternLearner) *ContentLearningHandler {
	// Se não foi fornecido um learner, usa a instância compartilhada
	if contentLearner == nil {
		contentLearner = crawler.GetSharedContentLearner()
	}

	return &ContentLearningHandler{
		contentLearner: contentLearner,
		logger:         logger.NewLogger("content_learning_handler"),
	}
}

// LearnCatalogPagesRequest representa o corpo da requisição para aprender páginas de catálogo
type LearnCatalogPagesRequest struct {
	URLs        []string `json:"urls" binding:"required"`
	Description string   `json:"description"`
}

// LearnCatalogPages
// @Summary Aprende padrões de páginas de catálogo baseado no conteúdo
// @Description Visita as URLs fornecidas, analisa o conteúdo das páginas e aprende padrões que identificam páginas de catálogo/listagem
// @Tags Content Learning
// @Accept json
// @Produce json
// @Param request body LearnCatalogPagesRequest true "Lista de URLs de catálogo para análise de conteúdo"
// @Success 200 {object} map[string]interface{} "Padrões de catálogo aprendidos com sucesso"
// @Failure 400 {object} map[string]interface{} "Formato de requisição inválido"
// @Failure 500 {object} map[string]interface{} "Erro interno do servidor"
// @Router /content/learn/catalog [post]
func (clh *ContentLearningHandler) LearnCatalogPages(c *gin.Context) {
	var request LearnCatalogPagesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		clh.logger.Error("Invalid request format", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Formato de requisição inválido",
			"message": err.Error(),
		})
		return
	}

	clh.logger.WithFields(map[string]interface{}{
		"count":       len(request.URLs),
		"description": request.Description,
	}).Info("Learning catalog patterns from page content")

	// Visita as URLs e extrai conteúdo
	examples, err := clh.extractContentFromURLs(request.URLs)
	if err != nil {
		clh.logger.Error("Failed to extract content from URLs", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao extrair conteúdo das páginas",
			"message": err.Error(),
		})
		return
	}

	// Aprende padrões baseado no conteúdo
	if err := clh.contentLearner.LearnFromCatalogPages(examples); err != nil {
		clh.logger.Error("Failed to learn catalog patterns", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao aprender padrões de catálogo",
			"message": err.Error(),
		})
		return
	}

	// Salva padrões automaticamente após aprendizado
	if err := crawler.SaveSharedPatterns(); err != nil {
		clh.logger.WithFields(map[string]interface{}{"error": err}).Warn("Failed to save patterns to disk")
	} else {
		clh.logger.Info("Patterns saved to disk for persistence")
	}

	patterns := clh.contentLearner.GetLearnedPatterns()
	catalogPatterns := patterns["catalog"]

	c.JSON(http.StatusOK, gin.H{
		"message": "Padrões de catálogo aprendidos com sucesso baseado no conteúdo das páginas",
		"data": gin.H{
			"urls_processed":    len(request.URLs),
			"examples_analyzed": len(examples),
			"patterns_learned":  len(catalogPatterns),
			"learned_patterns":  catalogPatterns,
			"description":       request.Description,
			"learning_method":   "content_analysis",
		},
	})
}

// LearnPropertyPages
// @Summary Aprende padrões de páginas de propriedade baseado no conteúdo
// @Description Visita as URLs fornecidas, analisa o conteúdo das páginas e aprende padrões que identificam páginas de anúncios individuais
// @Tags Content Learning
// @Accept json
// @Produce json
// @Param request body LearnCatalogPagesRequest true "Lista de URLs de propriedade para análise de conteúdo"
// @Success 200 {object} map[string]interface{} "Padrões de propriedade aprendidos com sucesso"
// @Failure 400 {object} map[string]interface{} "Formato de requisição inválido"
// @Failure 500 {object} map[string]interface{} "Erro interno do servidor"
// @Router /content/learn/property [post]
func (clh *ContentLearningHandler) LearnPropertyPages(c *gin.Context) {
	var request LearnCatalogPagesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		clh.logger.Error("Invalid request format", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Formato de requisição inválido",
			"message": err.Error(),
		})
		return
	}

	clh.logger.WithFields(map[string]interface{}{
		"count":       len(request.URLs),
		"description": request.Description,
	}).Info("Learning property patterns from page content")

	// Visita as URLs e extrai conteúdo
	examples, err := clh.extractContentFromURLs(request.URLs)
	if err != nil {
		clh.logger.Error("Failed to extract content from URLs", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao extrair conteúdo das páginas",
			"message": err.Error(),
		})
		return
	}

	// Aprende padrões baseado no conteúdo
	if err := clh.contentLearner.LearnFromPropertyPages(examples); err != nil {
		clh.logger.Error("Failed to learn property patterns", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao aprender padrões de propriedade",
			"message": err.Error(),
		})
		return
	}

	// Salva padrões automaticamente após aprendizado
	if err := crawler.SaveSharedPatterns(); err != nil {
		clh.logger.WithFields(map[string]interface{}{"error": err}).Warn("Failed to save patterns to disk")
	} else {
		clh.logger.Info("Patterns saved to disk for persistence")
	}

	patterns := clh.contentLearner.GetLearnedPatterns()
	propertyPatterns := patterns["property"]

	c.JSON(http.StatusOK, gin.H{
		"message": "Padrões de propriedade aprendidos com sucesso baseado no conteúdo das páginas",
		"data": gin.H{
			"urls_processed":    len(request.URLs),
			"examples_analyzed": len(examples),
			"patterns_learned":  len(propertyPatterns),
			"learned_patterns":  propertyPatterns,
			"description":       request.Description,
			"learning_method":   "content_analysis",
		},
	})
}

// ClassifyPageRequest representa o corpo da requisição para classificar uma página
type ClassifyPageRequest struct {
	URL string `json:"url" binding:"required"`
}

// ClassifyPage
// @Summary Classifica uma página baseado no conteúdo
// @Description Visita uma URL, analisa o conteúdo da página e classifica como 'catalog', 'property' ou 'unknown'
// @Tags Content Learning
// @Accept json
// @Produce json
// @Param request body ClassifyPageRequest true "URL a ser classificada"
// @Success 200 {object} map[string]interface{} "Página classificada com sucesso"
// @Failure 400 {object} map[string]interface{} "Formato de requisição inválido"
// @Failure 500 {object} map[string]interface{} "Erro ao acessar a página"
// @Router /content/classify [post]
func (clh *ContentLearningHandler) ClassifyPage(c *gin.Context) {
	var request ClassifyPageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		clh.logger.Error("Invalid request format", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Formato de requisição inválido",
			"message": err.Error(),
		})
		return
	}

	// Cria collector para visitar a página
	collector := colly.NewCollector()

	var pageType string
	var confidence float64
	var pageFeatures map[string]interface{}
	var visitError error

	collector.OnHTML("html", func(e *colly.HTMLElement) {
		// Classifica baseado no conteúdo
		pageType, confidence = clh.contentLearner.ClassifyPageContent(e)

		// Extrai características para debug
		pageFeatures = clh.extractPageFeatures(e)
	})

	collector.OnError(func(r *colly.Response, err error) {
		visitError = err
	})

	// Visita a URL
	if err := collector.Visit(request.URL); err != nil {
		clh.logger.Error("Failed to visit URL", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao acessar a página",
			"message": err.Error(),
		})
		return
	}

	if visitError != nil {
		clh.logger.Error("Error visiting page", visitError)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao processar a página",
			"message": visitError.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":                   request.URL,
		"type":                  pageType,
		"confidence":            confidence,
		"classification_method": "content_analysis",
		"page_features":         pageFeatures,
	})
}

// GetLearnedPatterns
// @Summary Retorna todos os padrões de conteúdo aprendidos
// @Description Retorna uma lista de todos os padrões de conteúdo que o sistema aprendeu, separados por tipo (catalog/property)
// @Tags Content Learning
// @Produce json
// @Success 200 {object} map[string]interface{} "Padrões recuperados com sucesso"
// @Router /content/patterns [get]
func (clh *ContentLearningHandler) GetLearnedPatterns(c *gin.Context) {
	patterns := clh.contentLearner.GetLearnedPatterns()

	catalogCount := 0
	if _, ok := patterns["catalog"]; ok {
		catalogCount = len(patterns["catalog"])
	}

	propertyCount := 0
	if _, ok := patterns["property"]; ok {
		propertyCount = len(patterns["property"])
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Padrões de conteúdo recuperados com sucesso",
		"data": gin.H{
			"patterns":                patterns,
			"catalog_patterns_count":  catalogCount,
			"property_patterns_count": propertyCount,
			"learning_method":         "content_analysis",
		},
	})
}

// extractContentFromURLs visita URLs e extrai conteúdo para aprendizado
func (clh *ContentLearningHandler) extractContentFromURLs(urls []string) ([]crawler.ContentExample, error) {
	var examples []crawler.ContentExample

	collector := colly.NewCollector()

	collector.OnHTML("html", func(e *colly.HTMLElement) {
		features := clh.extractPageFeatures(e)

		example := crawler.ContentExample{
			URL:        e.Request.URL.String(),
			Features:   features,
			TextSample: clh.truncateText(e.Text, 200),
			AddedAt:    time.Now(),
		}

		examples = append(examples, example)
	})

	collector.OnError(func(r *colly.Response, err error) {
		clh.logger.WithFields(map[string]interface{}{
			"url":   r.Request.URL.String(),
			"error": err,
		}).Warn("Failed to extract content from URL")
	})

	// Visita todas as URLs
	for _, url := range urls {
		if err := collector.Visit(url); err != nil {
			clh.logger.WithFields(map[string]interface{}{
				"url":   url,
				"error": err,
			}).Warn("Failed to visit URL for content extraction")
		}
	}

	clh.logger.WithField("examples_extracted", len(examples)).Info("Content extraction completed")
	return examples, nil
}

// extractPageFeatures extrai características de uma página (duplicado do ContentBasedPatternLearner para uso no handler)
func (clh *ContentLearningHandler) extractPageFeatures(e *colly.HTMLElement) map[string]interface{} {
	features := make(map[string]interface{})

	text := e.Text
	textLower := strings.ToLower(text)

	// Características de preço
	priceCount := strings.Count(text, "R$")
	features["price_count"] = priceCount

	// Características de texto
	features["text_length"] = len(text)
	features["text_sample"] = clh.truncateText(text, 200)

	// Palavras-chave específicas
	features["quartos_count"] = strings.Count(textLower, "quartos")
	features["banheiros_count"] = strings.Count(textLower, "banheiros")
	features["imoveis_count"] = strings.Count(textLower, "imóveis") + strings.Count(textLower, "imoveis")

	// Características estruturais
	features["link_count"] = len(e.ChildAttrs("a", "href"))
	features["image_count"] = len(e.ChildAttrs("img", "src"))
	features["has_pagination"] = e.ChildText(".pagination") != "" || e.ChildText(".paginacao") != "" || strings.Contains(textLower, "próxima") || strings.Contains(textLower, "anterior")
	features["has_filters"] = e.ChildText(".filters") != "" || e.ChildText(".filtros") != "" || strings.Contains(textLower, "filtrar") || strings.Contains(textLower, "ordenar")

	return features
}

func (clh *ContentLearningHandler) truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
