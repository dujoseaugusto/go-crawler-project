package main

import (
	"context"
	"log"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
)

// Exemplo de como usar o novo crawler refatorado
func main() {
	// Configurar logger
	logger.SetLevel(logger.INFO)

	// Carregar configuração
	cfg := config.LoadConfig()

	// Criar repositório
	repo, err := repository.NewMongoRepository(cfg.MongoURI, "crawler", "properties")
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	// Criar serviço de IA (opcional)
	var aiService *ai.GeminiService
	if aiSvc, err := ai.NewGeminiService(context.Background()); err == nil {
		aiService = aiSvc
		log.Println("AI service initialized successfully")
	} else {
		log.Printf("AI service not available: %v", err)
	}

	// Criar o novo motor de crawler
	engine := crawler.NewCrawlerEngine(repo, aiService)

	// URLs para crawling
	urls := []string{
		"https://estrelacorretora.com.br/imoveis",
		"https://www.aencasa.com.br/imoveis/?operacao=venda",
		"https://www.preguinhoimoveis.com.br/imovel/venda",
	}

	// Iniciar o crawler
	ctx := context.Background()
	if err := engine.Start(ctx, urls); err != nil {
		log.Fatalf("Crawler failed: %v", err)
	}

	// Obter estatísticas finais
	stats := engine.GetStats()
	log.Printf("Crawling completed - URLs: %d, Properties: %d, Saved: %d, Errors: %d",
		stats.URLsVisited, stats.PropertiesFound, stats.PropertiesSaved, stats.ErrorsCount)
}
