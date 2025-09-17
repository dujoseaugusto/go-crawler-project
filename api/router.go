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

	var patternLearningHandler *handler.PatternLearningHandler
	if patternLearner != nil {
		patternLearningHandler = handler.NewPatternLearningHandler(patternLearner)
	}

	var contentLearningHandler *handler.ContentLearningHandler
	if contentLearner != nil {
		contentLearningHandler = handler.NewContentLearningHandler(contentLearner)
	}

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

	// Endpoints de aprendizado de padrões (apenas se o serviço estiver disponível)
	if patternLearningHandler != nil {
		patternsGroup := r.Group("/patterns")
		{
			// Aprendizado baseado em URL (método antigo)
			patternsGroup.POST("/learn/catalog", patternLearningHandler.LearnCatalogURLs)
			patternsGroup.POST("/learn/property", patternLearningHandler.LearnPropertyURLs)

			// Classificação baseada em URL
			patternsGroup.POST("/classify", patternLearningHandler.ClassifyURL)

			// Gerenciamento de padrões
			patternsGroup.GET("", patternLearningHandler.GetLearnedPatterns)
			patternsGroup.GET("/export", patternLearningHandler.ExportPatterns)
			patternsGroup.POST("/import", patternLearningHandler.ImportPatterns)
		}
	}

	// Endpoints de aprendizado baseado em conteúdo (novo método inteligente)
	if contentLearningHandler != nil {
		contentGroup := r.Group("/content")
		{
			// Aprendizado baseado em conteúdo da página
			contentGroup.POST("/learn/catalog", contentLearningHandler.LearnCatalogPages)
			contentGroup.POST("/learn/property", contentLearningHandler.LearnPropertyPages)

			// Classificação baseada em conteúdo
			contentGroup.POST("/classify", contentLearningHandler.ClassifyPage)

			// Gerenciamento de padrões de conteúdo
			contentGroup.GET("/patterns", contentLearningHandler.GetLearnedPatterns)
		}
	}

	// Endpoint de health check (sem rate limiting)
	r.GET("/health", func(c *gin.Context) {
		features := []string{"web-interface", "search", "crawler"}
		if citySitesHandler != nil {
			features = append(features, "city-sites-management", "site-discovery")
		}
		if patternLearningHandler != nil {
			features = append(features, "pattern-learning", "url-classification")
		}
		if contentLearningHandler != nil {
			features = append(features, "content-learning", "intelligent-classification")
		}

		c.JSON(200, gin.H{
			"status":   "healthy",
			"service":  "go-crawler-api",
			"version":  "1.2.0",
			"features": features,
		})
	})

	return r
}
