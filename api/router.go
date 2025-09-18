package api

import (
	"time"

	"github.com/dujoseaugusto/go-crawler-project/api/handler"
	"github.com/dujoseaugusto/go-crawler-project/api/middleware"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/service"
	"github.com/gin-gonic/gin"
)

func SetupRouter(propertyService *service.PropertyService) *gin.Engine {
	return SetupRouterWithCitySites(propertyService, nil)
}

func SetupRouterWithCitySites(propertyService *service.PropertyService, citySitesService *service.CitySitesService) *gin.Engine {
	return SetupRouterWithPatternLearning(propertyService, citySitesService, nil)
}

func SetupRouterWithPatternLearning(propertyService *service.PropertyService, citySitesService *service.CitySitesService, patternLearner *crawler.PatternLearner) *gin.Engine {
	return SetupRouterWithContentLearning(propertyService, citySitesService, patternLearner, nil)
}

func SetupRouterWithContentLearning(propertyService *service.PropertyService, citySitesService *service.CitySitesService, patternLearner *crawler.PatternLearner, contentLearner *crawler.ContentBasedPatternLearner) *gin.Engine {
	r := gin.Default()

	// Configurar rate limiting
	// 100 requisições por hora para endpoints gerais
	generalLimiter := middleware.NewRateLimiter(100, time.Hour)

	// crawlerLimiter temporariamente removido para debug
	// crawlerLimiter := middleware.NewRateLimiter(50, time.Hour)

	// Criar handlers
	propertyHandler := handler.NewPropertyHandler(propertyService)

	var citySitesHandler *handler.CitySitesHandler
	if citySitesService != nil {
		citySitesHandler = handler.NewCitySitesHandler(citySitesService)
	}

	// Handlers de aprendizado removidos - sistema simplificado

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

	// Documentação da API (Swagger UI)
	r.GET("/docs", func(c *gin.Context) {
		c.File("./web/docs.html")
	})
	r.GET("/api-docs", func(c *gin.Context) {
		c.Redirect(301, "/docs")
	})

	// Endpoints de propriedades
	r.GET("/properties", propertyHandler.GetProperties)
	r.GET("/properties/search", propertyHandler.SearchProperties)

	// Endpoint do crawler (rate limiting removido temporariamente)
	crawlerGroup := r.Group("/crawler")
	{
		crawlerGroup.POST("/trigger", propertyHandler.TriggerCrawler)
		crawlerGroup.POST("/cleanup", propertyHandler.CleanupDatabase)
	}

	// Endpoints de cidades e sites (apenas se o serviço estiver disponível)
	if citySitesHandler != nil {
		citiesGroup := r.Group("/cities")
		{
			// Descoberta de sites
			citiesGroup.POST("/discover-sites", citySitesHandler.DiscoverSites)
			citiesGroup.GET("/discovery/jobs/:job_id", citySitesHandler.GetDiscoveryJob)
			citiesGroup.GET("/discovery/jobs", citySitesHandler.GetActiveDiscoveryJobs)

			// Gerenciamento de cidades
			citiesGroup.GET("", citySitesHandler.GetAllCities)
			citiesGroup.GET("/:city", citySitesHandler.GetCityByName)
			citiesGroup.GET("/:city/sites", citySitesHandler.GetCitySites)
			citiesGroup.POST("/:city/validate", citySitesHandler.ValidateCitySites)
			citiesGroup.DELETE("/:city", citySitesHandler.DeleteCity)

			// Gerenciamento de sites
			citiesGroup.POST("/:city/sites", citySitesHandler.AddSiteToCity)
			citiesGroup.DELETE("/:city/sites/:url", citySitesHandler.RemoveSiteFromCity)
			citiesGroup.PUT("/:city/sites/:url/stats", citySitesHandler.UpdateSiteStats)

			// Estatísticas e limpeza
			citiesGroup.GET("/statistics", citySitesHandler.GetStatistics)
			citiesGroup.POST("/cleanup", citySitesHandler.CleanupInactiveSites)

			// Busca por região
			citiesGroup.GET("/region/:region", citySitesHandler.GetCitiesByRegion)
		}
	}

	// Sistema de padrões removido - não é mais necessário

	// Sistema de aprendizado removido - não é mais necessário

	// Endpoint de health check (sem rate limiting)
	r.GET("/health", func(c *gin.Context) {
		features := []string{"web-interface", "search", "crawler", "swagger-documentation"}
		if citySitesHandler != nil {
			features = append(features, "city-sites-management", "site-discovery")
		}

		c.JSON(200, gin.H{
			"status":      "healthy",
			"service":     "go-crawler-api",
			"version":     "1.3.0",
			"features":    features,
			"docs_url":    "/docs",
			"mode":        "simplified",
			"description": "Sistema simplificado - coleta todas as páginas como propriedades",
		})
	})

	return r
}
