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

	// crawlerLimiter temporariamente removido para debug
	// crawlerLimiter := middleware.NewRateLimiter(50, time.Hour)

	propertyHandler := handler.NewPropertyHandler(propertyService)

	// Aplicar middlewares
	r.Use(middleware.CORSMiddleware())
	r.Use(generalLimiter.Middleware())

	// Servir arquivos estáticos da interface web
	r.Static("/static", "./web/static")

	// Servir página principal
	r.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})
	r.GET("/index.html", func(c *gin.Context) {
		c.File("./web/index.html")
	})

	// Rota adicional para servir a interface web
	r.GET("/web", func(c *gin.Context) {
		c.Redirect(301, "/")
	})

	// Endpoints de propriedades
	r.GET("/properties", propertyHandler.GetProperties)
	r.GET("/properties/search", propertyHandler.SearchProperties)

	// Endpoint do crawler (rate limiting removido temporariamente)
	crawlerGroup := r.Group("/crawler")
	{
		crawlerGroup.POST("/trigger", propertyHandler.TriggerCrawler)
	}

	// Endpoint de health check (sem rate limiting)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "go-crawler-api",
			"version": "1.0.0",
		})
	})

	return r
}
