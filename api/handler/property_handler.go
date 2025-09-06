package handler

import (
	"net/http"
	"strconv"

	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/dujoseaugusto/go-crawler-project/internal/service"
	"github.com/gin-gonic/gin"
)

type PropertyHandler struct {
	Service *service.PropertyService
}

func (h *PropertyHandler) GetProperties(c *gin.Context) {
	properties, err := h.Service.GetAllProperties(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to fetch properties"})
		return
	}
	c.JSON(http.StatusOK, properties)
}

// SearchProperties busca propriedades com filtros e paginação
func (h *PropertyHandler) SearchProperties(c *gin.Context) {
	// Extrair filtros dos query parameters
	filter := repository.PropertyFilter{
		Cidade:     c.Query("cidade"),
		Bairro:     c.Query("bairro"),
		TipoImovel: c.Query("tipo_imovel"),
	}

	// Filtros numéricos
	if valorMin := c.Query("valor_min"); valorMin != "" {
		if val, err := strconv.ParseFloat(valorMin, 64); err == nil {
			filter.ValorMin = val
		}
	}

	if valorMax := c.Query("valor_max"); valorMax != "" {
		if val, err := strconv.ParseFloat(valorMax, 64); err == nil {
			filter.ValorMax = val
		}
	}

	if quartosMin := c.Query("quartos_min"); quartosMin != "" {
		if val, err := strconv.Atoi(quartosMin); err == nil {
			filter.QuartosMin = val
		}
	}

	if quartosMax := c.Query("quartos_max"); quartosMax != "" {
		if val, err := strconv.Atoi(quartosMax); err == nil {
			filter.QuartosMax = val
		}
	}

	if banheirosMin := c.Query("banheiros_min"); banheirosMin != "" {
		if val, err := strconv.Atoi(banheirosMin); err == nil {
			filter.BanheirosMin = val
		}
	}

	if banheirosMax := c.Query("banheiros_max"); banheirosMax != "" {
		if val, err := strconv.Atoi(banheirosMax); err == nil {
			filter.BanheirosMax = val
		}
	}

	if areaMin := c.Query("area_min"); areaMin != "" {
		if val, err := strconv.ParseFloat(areaMin, 64); err == nil {
			filter.AreaMin = val
		}
	}

	if areaMax := c.Query("area_max"); areaMax != "" {
		if val, err := strconv.ParseFloat(areaMax, 64); err == nil {
			filter.AreaMax = val
		}
	}

	// Parâmetros de paginação
	pagination := repository.PaginationParams{
		Page:     1,
		PageSize: 10,
	}

	if page := c.Query("page"); page != "" {
		if val, err := strconv.Atoi(page); err == nil && val > 0 {
			pagination.Page = val
		}
	}

	if pageSize := c.Query("page_size"); pageSize != "" {
		if val, err := strconv.Atoi(pageSize); err == nil && val > 0 && val <= 100 {
			pagination.PageSize = val
		}
	}

	// Executar busca
	result, err := h.Service.SearchProperties(c.Request.Context(), filter, pagination)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to search properties", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// TriggerCrawler força a execução do crawler para coletar dados
func (h *PropertyHandler) TriggerCrawler(c *gin.Context) {
	err := h.Service.ForceCrawling(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger crawler", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Crawler triggered successfully"})
}
