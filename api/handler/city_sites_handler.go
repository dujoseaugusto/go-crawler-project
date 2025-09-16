package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/dujoseaugusto/go-crawler-project/internal/service"
	"github.com/gin-gonic/gin"
)

// CitySitesHandler gerencia endpoints relacionados a sites por cidade
type CitySitesHandler struct {
	Service *service.CitySitesService
	logger  *logger.Logger
}

// NewCitySitesHandler cria um novo handler
func NewCitySitesHandler(service *service.CitySitesService) *CitySitesHandler {
	return &CitySitesHandler{
		Service: service,
		logger:  logger.NewLogger("city_sites_handler"),
	}
}

// DiscoverSitesRequest representa requisição para descoberta de sites
type DiscoverSitesRequest struct {
	Cities  []repository.CityInfo       `json:"cities" binding:"required,min=1,max=50"`
	Options repository.DiscoveryOptions `json:"options,omitempty"`
}

// AddSiteRequest representa requisição para adicionar site manualmente
type AddSiteRequest struct {
	URL    string `json:"url" binding:"required,url"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

// UpdateSiteStatsRequest representa requisição para atualizar estatísticas de site
type UpdateSiteStatsRequest struct {
	PropertiesFound int     `json:"properties_found"`
	ErrorCount      int     `json:"error_count"`
	ResponseTime    float64 `json:"response_time"`
	LastError       string  `json:"last_error,omitempty"`
}

// DiscoverSites inicia descoberta de sites para cidades
func (h *CitySitesHandler) DiscoverSites(c *gin.Context) {
	h.logger.WithFields(map[string]interface{}{
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"client_ip": c.ClientIP(),
	}).Info("Site discovery request received")

	var req DiscoverSitesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Dados de requisição inválidos", err)
		return
	}

	// Validações adicionais
	if len(req.Cities) > 50 {
		h.respondWithError(c, http.StatusBadRequest, "Máximo 50 cidades por requisição", nil)
		return
	}

	// Valida cada cidade
	for i, city := range req.Cities {
		if strings.TrimSpace(city.Name) == "" {
			h.respondWithError(c, http.StatusBadRequest, fmt.Sprintf("Nome da cidade inválido no índice %d", i), nil)
			return
		}
		if len(city.State) != 2 {
			h.respondWithError(c, http.StatusBadRequest, fmt.Sprintf("Estado deve ter 2 caracteres no índice %d", i), nil)
			return
		}
	}

	// Inicia descoberta
	job, err := h.Service.DiscoverSitesForCities(c.Request.Context(), req.Cities, req.Options)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao iniciar descoberta", err)
		return
	}

	response := SuccessResponse{
		Message: fmt.Sprintf("Descoberta iniciada para %d cidades", len(req.Cities)),
		Data: map[string]interface{}{
			"job_id":         job.ID,
			"cities_queued":  len(job.Cities),
			"status":         job.Status,
			"estimated_time": "5-15 minutos",
		},
	}

	h.logger.WithFields(map[string]interface{}{
		"job_id":       job.ID,
		"cities_count": len(req.Cities),
	}).Info("Site discovery job started")

	c.JSON(http.StatusAccepted, response)
}

// GetDiscoveryJob retorna status de um job de descoberta
func (h *CitySitesHandler) GetDiscoveryJob(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Job ID é obrigatório", nil)
		return
	}

	job, err := h.Service.GetDiscoveryJob(jobID)
	if err != nil {
		h.respondWithError(c, http.StatusNotFound, "Job não encontrado", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Job encontrado",
		Data:    job,
	})
}

// GetActiveDiscoveryJobs retorna todos os jobs ativos
func (h *CitySitesHandler) GetActiveDiscoveryJobs(c *gin.Context) {
	jobs := h.Service.GetActiveDiscoveryJobs()

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Encontrados %d jobs ativos", len(jobs)),
		Data:    jobs,
	})
}

// GetAllCities retorna todas as cidades cadastradas
func (h *CitySitesHandler) GetAllCities(c *gin.Context) {
	h.logger.WithField("client_ip", c.ClientIP()).Debug("Getting all cities")

	cities, err := h.Service.GetAllCities(c.Request.Context())
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao buscar cidades", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Encontradas %d cidades", len(cities)),
		Data:    cities,
	})
}

// GetCityByName retorna uma cidade específica
func (h *CitySitesHandler) GetCityByName(c *gin.Context) {
	city := c.Param("city")
	state := c.Param("state")

	if city == "" || state == "" {
		h.respondWithError(c, http.StatusBadRequest, "Cidade e estado são obrigatórios", nil)
		return
	}

	cityData, err := h.Service.GetCityByName(c.Request.Context(), city, state)
	if err != nil {
		h.respondWithError(c, http.StatusNotFound, "Cidade não encontrada", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Cidade encontrada",
		Data:    cityData,
	})
}

// GetCitySites retorna sites de uma cidade específica
func (h *CitySitesHandler) GetCitySites(c *gin.Context) {
	city := c.Param("city")
	if city == "" {
		h.respondWithError(c, http.StatusBadRequest, "Nome da cidade é obrigatório", nil)
		return
	}

	// Estado é opcional, pode vir como query param
	state := c.Query("state")
	if state == "" {
		state = "MG" // Padrão para Minas Gerais
	}

	cityData, err := h.Service.GetCityByName(c.Request.Context(), city, state)
	if err != nil {
		h.respondWithError(c, http.StatusNotFound, "Cidade não encontrada", err)
		return
	}

	// Filtra apenas sites ativos se solicitado
	onlyActive := c.Query("active") == "true"
	sites := cityData.Sites
	if onlyActive {
		sites = cityData.GetActiveSites()
	}

	response := map[string]interface{}{
		"city":         cityData.City,
		"state":        cityData.State,
		"total_sites":  len(cityData.Sites),
		"active_sites": cityData.ActiveSites,
		"sites":        sites,
		"last_updated": cityData.LastUpdated,
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Encontrados %d sites para %s-%s", len(sites), city, state),
		Data:    response,
	})
}

// ValidateCitySites valida sites de uma cidade
func (h *CitySitesHandler) ValidateCitySites(c *gin.Context) {
	city := c.Param("city")
	state := c.Query("state")
	if state == "" {
		state = "MG"
	}

	if city == "" {
		h.respondWithError(c, http.StatusBadRequest, "Nome da cidade é obrigatório", nil)
		return
	}

	h.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Info("Starting city sites validation")

	result, err := h.Service.ValidateCitySites(c.Request.Context(), city, state)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro na validação", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Validação concluída: %d/%d sites válidos", result.ValidSites, result.TotalSites),
		Data:    result,
	})
}

// AddSiteToCity adiciona um site manualmente a uma cidade
func (h *CitySitesHandler) AddSiteToCity(c *gin.Context) {
	city := c.Param("city")
	state := c.Query("state")
	if state == "" {
		state = "MG"
	}

	if city == "" {
		h.respondWithError(c, http.StatusBadRequest, "Nome da cidade é obrigatório", nil)
		return
	}

	var req AddSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Dados de requisição inválidos", err)
		return
	}

	// Cria SiteInfo
	site := repository.SiteInfo{
		URL:    req.URL,
		Name:   req.Name,
		Domain: extractDomainFromURL(req.URL),
		Status: "active",
	}

	if req.Status != "" {
		site.Status = req.Status
	}

	err := h.Service.AddSiteToCity(c.Request.Context(), city, state, site)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao adicionar site", err)
		return
	}

	h.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"url":   req.URL,
	}).Info("Site added to city manually")

	c.JSON(http.StatusCreated, SuccessResponse{
		Message: "Site adicionado com sucesso",
		Data: map[string]interface{}{
			"city":  city,
			"state": state,
			"url":   req.URL,
		},
	})
}

// RemoveSiteFromCity remove um site de uma cidade
func (h *CitySitesHandler) RemoveSiteFromCity(c *gin.Context) {
	city := c.Param("city")
	url := c.Param("url")
	state := c.Query("state")
	if state == "" {
		state = "MG"
	}

	if city == "" || url == "" {
		h.respondWithError(c, http.StatusBadRequest, "Cidade e URL são obrigatórios", nil)
		return
	}

	err := h.Service.RemoveSiteFromCity(c.Request.Context(), city, state, url)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao remover site", err)
		return
	}

	h.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"url":   url,
	}).Info("Site removed from city")

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Site removido com sucesso",
		Data: map[string]interface{}{
			"city":  city,
			"state": state,
			"url":   url,
		},
	})
}

// UpdateSiteStats atualiza estatísticas de um site
func (h *CitySitesHandler) UpdateSiteStats(c *gin.Context) {
	city := c.Param("city")
	url := c.Param("url")
	state := c.Query("state")
	if state == "" {
		state = "MG"
	}

	if city == "" || url == "" {
		h.respondWithError(c, http.StatusBadRequest, "Cidade e URL são obrigatórios", nil)
		return
	}

	var req UpdateSiteStatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Dados de requisição inválidos", err)
		return
	}

	stats := repository.SiteStats{
		PropertiesFound: req.PropertiesFound,
		ErrorCount:      req.ErrorCount,
		ResponseTime:    req.ResponseTime,
		LastSuccess:     time.Now(),
		LastError:       req.LastError,
	}

	err := h.Service.UpdateSiteStats(c.Request.Context(), city, state, url, stats)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao atualizar estatísticas", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Estatísticas atualizadas com sucesso",
		Data: map[string]interface{}{
			"city":             city,
			"state":            state,
			"url":              url,
			"properties_found": req.PropertiesFound,
		},
	})
}

// DeleteCity remove uma cidade e todos os seus sites
func (h *CitySitesHandler) DeleteCity(c *gin.Context) {
	city := c.Param("city")
	state := c.Query("state")
	if state == "" {
		state = "MG"
	}

	if city == "" {
		h.respondWithError(c, http.StatusBadRequest, "Nome da cidade é obrigatório", nil)
		return
	}

	// Confirmação de segurança
	confirm := c.Query("confirm")
	if confirm != "true" {
		h.respondWithError(c, http.StatusBadRequest, "Confirmação necessária: adicione ?confirm=true", nil)
		return
	}

	h.logger.WithFields(map[string]interface{}{
		"city":      city,
		"state":     state,
		"client_ip": c.ClientIP(),
	}).Warn("City deletion requested")

	err := h.Service.DeleteCity(c.Request.Context(), city, state)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao deletar cidade", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Cidade deletada com sucesso",
		Data: map[string]interface{}{
			"city":  city,
			"state": state,
		},
	})
}

// GetStatistics retorna estatísticas gerais do sistema
func (h *CitySitesHandler) GetStatistics(c *gin.Context) {
	stats, err := h.Service.GetStatistics(c.Request.Context())
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao buscar estatísticas", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Estatísticas recuperadas com sucesso",
		Data:    stats,
	})
}

// CleanupInactiveSites remove sites inativos antigos
func (h *CitySitesHandler) CleanupInactiveSites(c *gin.Context) {
	// Parâmetro opcional para idade máxima (em dias)
	maxAgeDays := 30 // Padrão: 30 dias
	if days := c.Query("max_age_days"); days != "" {
		if parsed, err := strconv.Atoi(days); err == nil && parsed > 0 {
			maxAgeDays = parsed
		}
	}

	maxAge := time.Duration(maxAgeDays) * 24 * time.Hour

	h.logger.WithFields(map[string]interface{}{
		"max_age_days": maxAgeDays,
		"client_ip":    c.ClientIP(),
	}).Info("Cleanup of inactive sites requested")

	err := h.Service.CleanupInactiveSites(c.Request.Context(), maxAge)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro na limpeza", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Limpeza concluída: sites inativos há mais de %d dias foram removidos", maxAgeDays),
		Data: map[string]interface{}{
			"max_age_days": maxAgeDays,
		},
	})
}

// GetCitiesByRegion retorna cidades de uma região específica
func (h *CitySitesHandler) GetCitiesByRegion(c *gin.Context) {
	region := c.Param("region")
	if region == "" {
		h.respondWithError(c, http.StatusBadRequest, "Região é obrigatória", nil)
		return
	}

	cities, err := h.Service.GetCitiesByRegion(c.Request.Context(), region)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Erro ao buscar cidades por região", err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Encontradas %d cidades na região %s", len(cities), region),
		Data:    cities,
	})
}

// respondWithError envia uma resposta de erro padronizada
func (h *CitySitesHandler) respondWithError(c *gin.Context, statusCode int, message string, err error) {
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

// extractDomainFromURL extrai domínio de uma URL (função auxiliar)
func extractDomainFromURL(rawURL string) string {
	if strings.HasPrefix(rawURL, "http") {
		parts := strings.Split(rawURL, "/")
		if len(parts) > 2 {
			return parts[2]
		}
	}
	return rawURL
}

