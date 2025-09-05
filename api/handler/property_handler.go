package handler

import (
	"net/http"

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

// TriggerCrawler força a execução do crawler para coletar dados
func (h *PropertyHandler) TriggerCrawler(c *gin.Context) {
	err := h.Service.ForceCrawling(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger crawler", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Crawler triggered successfully"})
}
