package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/dujoseaugusto/go-crawler-project/internal/service"
	"github.com/gin-gonic/gin"
)

type PropertyHandler struct {
	Service *service.PropertyService
	logger  *logger.Logger
}

// NewPropertyHandler cria um novo handler com logger
func NewPropertyHandler(service *service.PropertyService) *PropertyHandler {
	return &PropertyHandler{
		Service: service,
		logger:  logger.NewLogger("property_handler"),
	}
}

// SearchRequest representa os parâmetros de busca validados
type SearchRequest struct {
	Query        string  `form:"q" binding:"omitempty,max=200"` // Busca inteligente por palavras-chave
	Cidade       string  `form:"cidade" binding:"omitempty,max=50"`
	Bairro       string  `form:"bairro" binding:"omitempty,max=50"`
	TipoImovel   string  `form:"tipo_imovel" binding:"omitempty,max=30"`
	ValorMin     float64 `form:"valor_min" binding:"omitempty,min=0,max=100000000"`
	ValorMax     float64 `form:"valor_max" binding:"omitempty,min=0,max=100000000"`
	QuartosMin   int     `form:"quartos_min" binding:"omitempty,min=0,max=20"`
	QuartosMax   int     `form:"quartos_max" binding:"omitempty,min=0,max=20"`
	BanheirosMin int     `form:"banheiros_min" binding:"omitempty,min=0,max=20"`
	BanheirosMax int     `form:"banheiros_max" binding:"omitempty,min=0,max=20"`
	AreaMin      float64 `form:"area_min" binding:"omitempty,min=0,max=100000"`
	AreaMax      float64 `form:"area_max" binding:"omitempty,min=0,max=100000"`
	Page         int     `form:"page" binding:"omitempty,min=1,max=1000"`
	PageSize     int     `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// ErrorResponse representa uma resposta de erro padronizada
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// SuccessResponse representa uma resposta de sucesso padronizada
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// sanitizeString remove caracteres perigosos e limita o tamanho
func sanitizeString(input string, maxLength int) string {
	// Remove caracteres potencialmente perigosos
	input = strings.ReplaceAll(input, "<", "")
	input = strings.ReplaceAll(input, ">", "")
	input = strings.ReplaceAll(input, "\"", "")
	input = strings.ReplaceAll(input, "'", "")
	input = strings.ReplaceAll(input, "&", "")

	// Remove espaços extras
	input = strings.TrimSpace(input)

	// Limita o tamanho
	if len(input) > maxLength {
		input = input[:maxLength]
	}

	return input
}

// respondWithError envia uma resposta de erro padronizada
func (h *PropertyHandler) respondWithError(c *gin.Context, statusCode int, message string, err error) {
	h.logger.WithFields(map[string]interface{}{
		"method":      c.Request.Method,
		"path":        c.Request.URL.Path,
		"query":       c.Request.URL.RawQuery,
		"client_ip":   c.ClientIP(),
		"user_agent":  c.Request.UserAgent(),
		"status_code": statusCode,
	}).Error(message, err)

	response := ErrorResponse{
		Error:   message,
		Code:    statusCode,
		Message: "Consulte os logs para mais detalhes",
	}

	c.JSON(statusCode, response)
}

// validateSearchParams valida os parâmetros de busca
func (h *PropertyHandler) validateSearchParams(req *SearchRequest) error {
	// Valida se valor máximo é maior que mínimo
	if req.ValorMax > 0 && req.ValorMin > 0 && req.ValorMax < req.ValorMin {
		return fmt.Errorf("valor_max deve ser maior que valor_min")
	}

	// Valida se quartos máximo é maior que mínimo
	if req.QuartosMax > 0 && req.QuartosMin > 0 && req.QuartosMax < req.QuartosMin {
		return fmt.Errorf("quartos_max deve ser maior que quartos_min")
	}

	// Valida se banheiros máximo é maior que mínimo
	if req.BanheirosMax > 0 && req.BanheirosMin > 0 && req.BanheirosMax < req.BanheirosMin {
		return fmt.Errorf("banheiros_max deve ser maior que banheiros_min")
	}

	// Valida se área máxima é maior que mínima
	if req.AreaMax > 0 && req.AreaMin > 0 && req.AreaMax < req.AreaMin {
		return fmt.Errorf("area_max deve ser maior que area_min")
	}

	return nil
}

func (h *PropertyHandler) GetProperties(c *gin.Context) {
	h.logger.WithFields(map[string]interface{}{
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"client_ip": c.ClientIP(),
	}).Info("Fetching all properties")

	properties, err := h.Service.GetAllProperties(c.Request.Context())
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao buscar propriedades", err)
		return
	}

	response := SuccessResponse{
		Message: "Propriedades recuperadas com sucesso",
		Data:    properties,
	}

	h.logger.WithField("count", len(properties)).Info("Successfully fetched properties")
	c.JSON(http.StatusOK, response)
}

// SearchProperties busca propriedades com filtros e paginação validados
func (h *PropertyHandler) SearchProperties(c *gin.Context) {
	h.logger.WithFields(map[string]interface{}{
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"query":     c.Request.URL.RawQuery,
		"client_ip": c.ClientIP(),
	}).Info("Search request received")

	// Bind e valida os parâmetros usando as tags de validação
	var req SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Parâmetros de busca inválidos", err)
		return
	}

	// Validações customizadas
	if err := h.validateSearchParams(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Parâmetros de busca inválidos", err)
		return
	}

	// Sanitiza strings de entrada
	req.Query = sanitizeString(req.Query, 200)
	req.Cidade = sanitizeString(req.Cidade, 50)
	req.Bairro = sanitizeString(req.Bairro, 50)
	req.TipoImovel = sanitizeString(req.TipoImovel, 30)

	// Define valores padrão para paginação
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	// Log dos filtros aplicados
	h.logger.WithFields(map[string]interface{}{
		"filters": map[string]interface{}{
			"query":       req.Query,
			"cidade":      req.Cidade,
			"bairro":      req.Bairro,
			"tipo_imovel": req.TipoImovel,
			"valor_min":   req.ValorMin,
			"valor_max":   req.ValorMax,
		},
		"pagination": map[string]interface{}{
			"page":      req.Page,
			"page_size": req.PageSize,
		},
	}).Debug("Search filters applied")

	// Converte para estruturas internas
	filter := repository.PropertyFilter{
		Query:        req.Query,
		Cidade:       req.Cidade,
		Bairro:       req.Bairro,
		TipoImovel:   req.TipoImovel,
		ValorMin:     req.ValorMin,
		ValorMax:     req.ValorMax,
		QuartosMin:   req.QuartosMin,
		QuartosMax:   req.QuartosMax,
		BanheirosMin: req.BanheirosMin,
		BanheirosMax: req.BanheirosMax,
		AreaMin:      req.AreaMin,
		AreaMax:      req.AreaMax,
	}

	pagination := repository.PaginationParams{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	// Executar busca
	result, err := h.Service.SearchProperties(c.Request.Context(), filter, pagination)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao buscar propriedades", err)
		return
	}

	h.logger.WithFields(map[string]interface{}{
		"total_items":  result.TotalItems,
		"current_page": result.CurrentPage,
		"total_pages":  result.TotalPages,
		"results":      len(result.Properties),
	}).Info("Search completed successfully")

	c.JSON(http.StatusOK, result)
}

// TriggerCrawler força a execução do crawler para coletar dados
func (h *PropertyHandler) TriggerCrawler(c *gin.Context) {
	h.logger.WithFields(map[string]interface{}{
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"client_ip": c.ClientIP(),
	}).Info("Crawler trigger requested")

	// Verifica se é uma requisição POST válida
	if c.Request.Method != http.MethodPost {
		h.respondWithError(c, http.StatusMethodNotAllowed, "Método não permitido", fmt.Errorf("expected POST, got %s", c.Request.Method))
		return
	}

	// Inicia o crawler de forma assíncrona para não bloquear a resposta
	go func() {
		crawlerLogger := h.logger.WithField("operation", "crawler_execution")
		crawlerLogger.Info("Starting crawler execution")

		// Cria um contexto independente para o crawler que não será cancelado
		// quando a requisição HTTP terminar
		ctx := context.Background()

		if err := h.Service.ForceCrawling(ctx); err != nil {
			crawlerLogger.Error("Error during crawler execution", err)
		} else {
			crawlerLogger.Info("Crawler execution completed successfully")
		}
	}()

	response := SuccessResponse{
		Message: "Crawler iniciado com sucesso",
		Data: map[string]interface{}{
			"status": "started",
			"note":   "O processo está sendo executado em segundo plano",
		},
	}

	h.logger.Info("Crawler trigger response sent")
	c.JSON(http.StatusAccepted, response)
}
