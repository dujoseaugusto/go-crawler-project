package handler

import (
	"encoding/json"
	"net/http"

	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/gin-gonic/gin"
)

// PatternLearningHandler gerencia o aprendizado de padrões
type PatternLearningHandler struct {
	patternLearner *crawler.PatternLearner
	logger         *logger.Logger
}

// NewPatternLearningHandler cria um novo handler de aprendizado de padrões
func NewPatternLearningHandler(patternLearner *crawler.PatternLearner) *PatternLearningHandler {
	return &PatternLearningHandler{
		patternLearner: patternLearner,
		logger:         logger.NewLogger("pattern_learning_handler"),
	}
}

// LearnCatalogURLsRequest representa a requisição para aprender URLs de catálogo
type LearnCatalogURLsRequest struct {
	URLs        []string `json:"urls" binding:"required"`
	Description string   `json:"description,omitempty"`
}

// LearnPropertyURLsRequest representa a requisição para aprender URLs de propriedade
type LearnPropertyURLsRequest struct {
	URLs        []string `json:"urls" binding:"required"`
	Description string   `json:"description,omitempty"`
}

// ClassifyURLRequest representa a requisição para classificar uma URL
type ClassifyURLRequest struct {
	URL string `json:"url" binding:"required"`
}

// ClassifyURLResponse representa a resposta da classificação
type ClassifyURLResponse struct {
	URL        string  `json:"url"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
}

// LearnCatalogURLs ensina o sistema sobre URLs de catálogo
// @Summary Ensinar URLs de catálogo
// @Description Fornece exemplos de URLs de catálogo para o sistema aprender padrões
// @Tags Pattern Learning
// @Accept json
// @Produce json
// @Param request body LearnCatalogURLsRequest true "URLs de catálogo para aprender"
// @Success 200 {object} map[string]interface{} "Padrões aprendidos com sucesso"
// @Failure 400 {object} map[string]interface{} "Erro na requisição"
// @Failure 500 {object} map[string]interface{} "Erro interno do servidor"
// @Router /patterns/learn/catalog [post]
func (plh *PatternLearningHandler) LearnCatalogURLs(c *gin.Context) {
	var request LearnCatalogURLsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		plh.logger.Error("Invalid request format", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Formato de requisição inválido",
			"message": err.Error(),
		})
		return
	}

	// Valida se há URLs
	if len(request.URLs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Nenhuma URL fornecida",
			"message": "É necessário fornecer pelo menos uma URL de exemplo",
		})
		return
	}

	plh.logger.WithFields(map[string]interface{}{
		"url_count":   len(request.URLs),
		"description": request.Description,
	}).Info("Learning catalog URL patterns")

	// Aprende os padrões
	if err := plh.patternLearner.LearnCatalogURLs(request.URLs); err != nil {
		plh.logger.Error("Failed to learn catalog patterns", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao aprender padrões",
			"message": err.Error(),
		})
		return
	}

	// Retorna os padrões aprendidos
	patterns := plh.patternLearner.GetLearnedPatterns()
	catalogPatterns := patterns["catalog"]

	c.JSON(http.StatusOK, gin.H{
		"message": "Padrões de catálogo aprendidos com sucesso",
		"data": gin.H{
			"urls_processed":   len(request.URLs),
			"patterns_learned": len(catalogPatterns),
			"description":      request.Description,
			"learned_patterns": catalogPatterns,
		},
	})
}

// LearnPropertyURLs ensina o sistema sobre URLs de propriedade
// @Summary Ensinar URLs de propriedade
// @Description Fornece exemplos de URLs de propriedade individual para o sistema aprender padrões
// @Tags Pattern Learning
// @Accept json
// @Produce json
// @Param request body LearnPropertyURLsRequest true "URLs de propriedade para aprender"
// @Success 200 {object} map[string]interface{} "Padrões aprendidos com sucesso"
// @Failure 400 {object} map[string]interface{} "Erro na requisição"
// @Failure 500 {object} map[string]interface{} "Erro interno do servidor"
// @Router /patterns/learn/property [post]
func (plh *PatternLearningHandler) LearnPropertyURLs(c *gin.Context) {
	var request LearnPropertyURLsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		plh.logger.Error("Invalid request format", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Formato de requisição inválido",
			"message": err.Error(),
		})
		return
	}

	// Valida se há URLs
	if len(request.URLs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Nenhuma URL fornecida",
			"message": "É necessário fornecer pelo menos uma URL de exemplo",
		})
		return
	}

	plh.logger.WithFields(map[string]interface{}{
		"url_count":   len(request.URLs),
		"description": request.Description,
	}).Info("Learning property URL patterns")

	// Aprende os padrões
	if err := plh.patternLearner.LearnPropertyURLs(request.URLs); err != nil {
		plh.logger.Error("Failed to learn property patterns", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao aprender padrões",
			"message": err.Error(),
		})
		return
	}

	// Retorna os padrões aprendidos
	patterns := plh.patternLearner.GetLearnedPatterns()
	propertyPatterns := patterns["property"]

	c.JSON(http.StatusOK, gin.H{
		"message": "Padrões de propriedade aprendidos com sucesso",
		"data": gin.H{
			"urls_processed":   len(request.URLs),
			"patterns_learned": len(propertyPatterns),
			"description":      request.Description,
			"learned_patterns": propertyPatterns,
		},
	})
}

// ClassifyURL classifica uma URL baseada nos padrões aprendidos
// @Summary Classificar URL
// @Description Classifica uma URL como catálogo, propriedade ou desconhecido baseado nos padrões aprendidos
// @Tags Pattern Learning
// @Accept json
// @Produce json
// @Param request body ClassifyURLRequest true "URL para classificar"
// @Success 200 {object} ClassifyURLResponse "Classificação da URL"
// @Failure 400 {object} map[string]interface{} "Erro na requisição"
// @Router /patterns/classify [post]
func (plh *PatternLearningHandler) ClassifyURL(c *gin.Context) {
	var request ClassifyURLRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		plh.logger.Error("Invalid request format", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Formato de requisição inválido",
			"message": err.Error(),
		})
		return
	}

	// Classifica a URL
	urlType, confidence := plh.patternLearner.ClassifyURL(request.URL)

	plh.logger.WithFields(map[string]interface{}{
		"url":        request.URL,
		"type":       urlType,
		"confidence": confidence,
	}).Debug("URL classified")

	response := ClassifyURLResponse{
		URL:        request.URL,
		Type:       urlType,
		Confidence: confidence,
	}

	c.JSON(http.StatusOK, response)
}

// GetLearnedPatterns retorna todos os padrões aprendidos
// @Summary Obter padrões aprendidos
// @Description Retorna todos os padrões que o sistema aprendeu até agora
// @Tags Pattern Learning
// @Produce json
// @Success 200 {object} map[string]interface{} "Padrões aprendidos"
// @Router /patterns [get]
func (plh *PatternLearningHandler) GetLearnedPatterns(c *gin.Context) {
	patterns := plh.patternLearner.GetLearnedPatterns()

	catalogCount := len(patterns["catalog"])
	propertyCount := len(patterns["property"])

	c.JSON(http.StatusOK, gin.H{
		"message": "Padrões recuperados com sucesso",
		"data": gin.H{
			"catalog_patterns_count":  catalogCount,
			"property_patterns_count": propertyCount,
			"patterns":                patterns,
		},
	})
}

// ExportPatterns exporta os padrões aprendidos
// @Summary Exportar padrões
// @Description Exporta todos os padrões aprendidos em formato JSON
// @Tags Pattern Learning
// @Produce json
// @Success 200 {object} map[string]interface{} "Padrões exportados"
// @Failure 500 {object} map[string]interface{} "Erro interno do servidor"
// @Router /patterns/export [get]
func (plh *PatternLearningHandler) ExportPatterns(c *gin.Context) {
	data, err := plh.patternLearner.ExportPatterns()
	if err != nil {
		plh.logger.Error("Failed to export patterns", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao exportar padrões",
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=learned_patterns.json")
	c.Data(http.StatusOK, "application/json", data)
}

// ImportPatterns importa padrões de um arquivo JSON
// @Summary Importar padrões
// @Description Importa padrões de um arquivo JSON
// @Tags Pattern Learning
// @Accept json
// @Produce json
// @Param patterns body map[string]interface{} true "Padrões para importar"
// @Success 200 {object} map[string]interface{} "Padrões importados com sucesso"
// @Failure 400 {object} map[string]interface{} "Erro na requisição"
// @Failure 500 {object} map[string]interface{} "Erro interno do servidor"
// @Router /patterns/import [post]
func (plh *PatternLearningHandler) ImportPatterns(c *gin.Context) {
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		plh.logger.Error("Invalid JSON format", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Formato JSON inválido",
			"message": err.Error(),
		})
		return
	}

	// Converte de volta para JSON para importar
	jsonData, err := json.Marshal(data)
	if err != nil {
		plh.logger.Error("Failed to marshal data", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao processar dados",
			"message": err.Error(),
		})
		return
	}

	if err := plh.patternLearner.ImportPatterns(jsonData); err != nil {
		plh.logger.Error("Failed to import patterns", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao importar padrões",
			"message": err.Error(),
		})
		return
	}

	patterns := plh.patternLearner.GetLearnedPatterns()
	catalogCount := len(patterns["catalog"])
	propertyCount := len(patterns["property"])

	c.JSON(http.StatusOK, gin.H{
		"message": "Padrões importados com sucesso",
		"data": gin.H{
			"catalog_patterns_imported":  catalogCount,
			"property_patterns_imported": propertyCount,
		},
	})
}
