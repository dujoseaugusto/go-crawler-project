package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-crawler-project/internal/service"
)

type PropertyHandler struct {
	Service *service.PropertyService
}

func (h *PropertyHandler) GetProperties(c *gin.Context) {
	properties, err := h.Service.GetAllProperties()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to fetch properties"})
		return
	}
	c.JSON(http.StatusOK, properties)
}