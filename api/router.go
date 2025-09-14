package api

import (
	"time"

	"github.com/dujoseaugusto/go-crawler-project/api/handler"
	"github.com/dujoseaugusto/go-crawler-project/api/middleware"
	"github.com/dujoseaugusto/go-crawler-project/internal/service"
	"github.com/gin-gonic/gin"
)

func SetupRouter(propertyService *service.PropertyService) *gin.Engine {
	r := gin.Default()

	// Configurar rate limiting
	// 100 requisições por hora para endpoints gerais
	generalLimiter := middleware.NewRateLimiter(100, time.Hour)

	// 10 requisições por hora para o crawler (mais restritivo)
	crawlerLimiter := middleware.NewRateLimiter(10, time.Hour)

	propertyHandler := handler.NewPropertyHandler(propertyService)

	// Aplicar rate limiting geral
	r.Use(generalLimiter.Middleware())

	// Endpoints de propriedades
	r.GET("/properties", propertyHandler.GetProperties)
	r.GET("/properties/search", propertyHandler.SearchProperties)

	// Endpoint do crawler com rate limiting mais restritivo
	crawlerGroup := r.Group("/crawler")
	crawlerGroup.Use(crawlerLimiter.Middleware())
	{
		crawlerGroup.POST("/trigger", propertyHandler.TriggerCrawler)
		// Debug endpoint para testar se o grupo funciona
		crawlerGroup.GET("/debug", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "Crawler group is working",
				"path":    c.Request.URL.Path,
			})
		})
	}

	// Endpoint de health check (sem rate limiting)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "go-crawler-api",
		})
	})

	// Endpoint de trigger sem rate limiting (para debug)
	r.POST("/trigger-debug", propertyHandler.TriggerCrawler)

	return r
}
