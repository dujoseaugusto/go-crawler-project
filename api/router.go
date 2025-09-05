package api

import (
	"github.com/dujoseaugusto/go-crawler-project/api/handler"
	"github.com/dujoseaugusto/go-crawler-project/internal/service"
	"github.com/gin-gonic/gin"
)

func SetupRouter(propertyService *service.PropertyService) *gin.Engine {
	r := gin.Default()

	propertyHandler := &handler.PropertyHandler{
		Service: propertyService,
	}

	r.GET("/properties", propertyHandler.GetProperties)
	r.POST("/crawler/trigger", propertyHandler.TriggerCrawler)

	return r
}
