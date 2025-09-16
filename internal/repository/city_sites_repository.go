package repository

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CitySitesRepository define as operações para gerenciar sites por cidade
type CitySitesRepository interface {
	// CRUD básico
	SaveCitySites(ctx context.Context, citySites CitySites) error
	FindByCity(ctx context.Context, city, state string) (*CitySites, error)
	FindAllCities(ctx context.Context) ([]CitySites, error)
	FindCitiesByNames(ctx context.Context, cities []string) ([]CitySites, error)
	FindCitiesByRegion(ctx context.Context, region string) ([]CitySites, error)

	// Operações com sites
	GetAllActiveSites(ctx context.Context) ([]string, error)
	GetSitesByCity(ctx context.Context, city string) ([]string, error)
	GetSitesByCities(ctx context.Context, cities []string) ([]string, error)
	UpdateSiteStatus(ctx context.Context, city, state, url, status string) error
	UpdateSiteStats(ctx context.Context, city, state, url string, stats SiteStats) error

	// Gerenciamento
	DeleteCity(ctx context.Context, city, state string) error
	GetStatistics(ctx context.Context) (*CitySitesStatistics, error)
	CleanupInactiveSites(ctx context.Context, maxAge time.Duration) error
	Close()
}

// MongoCitySitesRepository implementa CitySitesRepository usando MongoDB
type MongoCitySitesRepository struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// NewMongoCitySitesRepository cria um novo repositório de sites por cidade
func NewMongoCitySitesRepository(uri, dbName string) (*MongoCitySitesRepository, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	if err := client.Ping(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	collection := client.Database(dbName).Collection("city_sites")

	repo := &MongoCitySitesRepository{
		client:     client,
		collection: collection,
	}

	// Criar índices necessários
	if err := repo.createIndexes(); err != nil {
		log.Printf("Warning: Failed to create city sites indexes: %v", err)
	}

	log.Printf("City sites repository initialized successfully")
	return repo, nil
}

// createIndexes cria os índices necessários para performance
func (r *MongoCitySitesRepository) createIndexes() error {
	ctx := context.Background()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "city", Value: 1}, {Key: "state", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "hash", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "region", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "last_updated", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "active_sites", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "sites.url", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "sites.status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "sites.last_success", Value: -1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %v", err)
	}

	log.Printf("City sites indexes created successfully")
	return nil
}

// SaveCitySites salva ou atualiza informações de uma cidade
func (r *MongoCitySitesRepository) SaveCitySites(ctx context.Context, citySites CitySites) error {
	// Atualiza estatísticas antes de salvar
	citySites.UpdateStats()

	// Usa upsert baseado na combinação cidade + estado
	filter := bson.M{
		"city":  citySites.City,
		"state": citySites.State,
	}

	update := bson.M{"$set": citySites}
	opts := options.Update().SetUpsert(true)

	result, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save city sites: %v", err)
	}

	if result.UpsertedCount > 0 {
		log.Printf("New city created: %s-%s with %d sites", citySites.City, citySites.State, citySites.TotalSites)
	} else if result.ModifiedCount > 0 {
		log.Printf("City updated: %s-%s with %d sites", citySites.City, citySites.State, citySites.TotalSites)
	}

	return nil
}

// FindByCity busca uma cidade específica
func (r *MongoCitySitesRepository) FindByCity(ctx context.Context, city, state string) (*CitySites, error) {
	filter := bson.M{
		"city":  city,
		"state": strings.ToUpper(state),
	}

	var citySites CitySites
	err := r.collection.FindOne(ctx, filter).Decode(&citySites)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find city: %v", err)
	}

	return &citySites, nil
}

// FindAllCities retorna todas as cidades
func (r *MongoCitySitesRepository) FindAllCities(ctx context.Context) ([]CitySites, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to find cities: %v", err)
	}
	defer cursor.Close(ctx)

	var cities []CitySites
	if err := cursor.All(ctx, &cities); err != nil {
		return nil, fmt.Errorf("failed to decode cities: %v", err)
	}

	return cities, nil
}

// FindCitiesByNames busca cidades específicas por nome
func (r *MongoCitySitesRepository) FindCitiesByNames(ctx context.Context, cities []string) ([]CitySites, error) {
	if len(cities) == 0 {
		return r.FindAllCities(ctx)
	}

	// Normaliza nomes das cidades para busca case-insensitive
	var cityFilters []bson.M
	for _, city := range cities {
		cityFilters = append(cityFilters, bson.M{
			"city": bson.M{"$regex": fmt.Sprintf("^%s$", city), "$options": "i"},
		})
	}

	filter := bson.M{"$or": cityFilters}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find cities by names: %v", err)
	}
	defer cursor.Close(ctx)

	var result []CitySites
	if err := cursor.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("failed to decode cities: %v", err)
	}

	return result, nil
}

// FindCitiesByRegion busca cidades por região
func (r *MongoCitySitesRepository) FindCitiesByRegion(ctx context.Context, region string) ([]CitySites, error) {
	filter := bson.M{"region": bson.M{"$regex": region, "$options": "i"}}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find cities by region: %v", err)
	}
	defer cursor.Close(ctx)

	var cities []CitySites
	if err := cursor.All(ctx, &cities); err != nil {
		return nil, fmt.Errorf("failed to decode cities: %v", err)
	}

	return cities, nil
}

// GetAllActiveSites retorna URLs de todos os sites ativos
func (r *MongoCitySitesRepository) GetAllActiveSites(ctx context.Context) ([]string, error) {
	pipeline := []bson.M{
		{"$unwind": "$sites"},
		{"$match": bson.M{"sites.status": "active"}},
		{"$project": bson.M{"url": "$sites.url"}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sites: %v", err)
	}
	defer cursor.Close(ctx)

	var urls []string
	for cursor.Next(ctx) {
		var result struct {
			URL string `bson:"url"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		urls = append(urls, result.URL)
	}

	return urls, nil
}

// GetSitesByCity retorna sites de uma cidade específica
func (r *MongoCitySitesRepository) GetSitesByCity(ctx context.Context, city string) ([]string, error) {
	filter := bson.M{
		"city": bson.M{"$regex": fmt.Sprintf("^%s$", city), "$options": "i"},
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$unwind": "$sites"},
		{"$match": bson.M{"sites.status": "active"}},
		{"$project": bson.M{"url": "$sites.url"}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get sites by city: %v", err)
	}
	defer cursor.Close(ctx)

	var urls []string
	for cursor.Next(ctx) {
		var result struct {
			URL string `bson:"url"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		urls = append(urls, result.URL)
	}

	return urls, nil
}

// GetSitesByCities retorna sites de múltiplas cidades
func (r *MongoCitySitesRepository) GetSitesByCities(ctx context.Context, cities []string) ([]string, error) {
	if len(cities) == 0 {
		return r.GetAllActiveSites(ctx)
	}

	var cityFilters []bson.M
	for _, city := range cities {
		cityFilters = append(cityFilters, bson.M{
			"city": bson.M{"$regex": fmt.Sprintf("^%s$", city), "$options": "i"},
		})
	}

	pipeline := []bson.M{
		{"$match": bson.M{"$or": cityFilters}},
		{"$unwind": "$sites"},
		{"$match": bson.M{"sites.status": "active"}},
		{"$project": bson.M{"url": "$sites.url"}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get sites by cities: %v", err)
	}
	defer cursor.Close(ctx)

	var urls []string
	for cursor.Next(ctx) {
		var result struct {
			URL string `bson:"url"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		urls = append(urls, result.URL)
	}

	return urls, nil
}

// UpdateSiteStatus atualiza o status de um site específico
func (r *MongoCitySitesRepository) UpdateSiteStatus(ctx context.Context, city, state, url, status string) error {
	filter := bson.M{
		"city":      city,
		"state":     strings.ToUpper(state),
		"sites.url": url,
	}

	update := bson.M{
		"$set": bson.M{
			"sites.$.status":       status,
			"sites.$.last_crawled": time.Now(),
			"last_updated":         time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update site status: %v", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("site not found: %s in %s-%s", url, city, state)
	}

	return nil
}

// UpdateSiteStats atualiza estatísticas de um site específico
func (r *MongoCitySitesRepository) UpdateSiteStats(ctx context.Context, city, state, url string, stats SiteStats) error {
	filter := bson.M{
		"city":      city,
		"state":     strings.ToUpper(state),
		"sites.url": url,
	}

	// Calcula success rate
	totalAttempts := stats.PropertiesFound + stats.ErrorCount
	successRate := 0.0
	if totalAttempts > 0 {
		successRate = float64(stats.PropertiesFound) / float64(totalAttempts) * 100
	}

	update := bson.M{
		"$set": bson.M{
			"sites.$.properties_found": stats.PropertiesFound,
			"sites.$.error_count":      stats.ErrorCount,
			"sites.$.response_time":    stats.ResponseTime,
			"sites.$.last_success":     stats.LastSuccess,
			"sites.$.last_error":       stats.LastError,
			"sites.$.success_rate":     successRate,
			"sites.$.last_crawled":     time.Now(),
			"last_updated":             time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update site stats: %v", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("site not found: %s in %s-%s", url, city, state)
	}

	return nil
}

// DeleteCity remove uma cidade e todos os seus sites
func (r *MongoCitySitesRepository) DeleteCity(ctx context.Context, city, state string) error {
	filter := bson.M{
		"city":  city,
		"state": strings.ToUpper(state),
	}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete city: %v", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("city not found: %s-%s", city, state)
	}

	log.Printf("City deleted: %s-%s", city, state)
	return nil
}

// GetStatistics retorna estatísticas gerais do sistema
func (r *MongoCitySitesRepository) GetStatistics(ctx context.Context) (*CitySitesStatistics, error) {
	// Pipeline para estatísticas agregadas
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":            nil,
				"total_cities":   bson.M{"$sum": 1},
				"total_sites":    bson.M{"$sum": "$total_sites"},
				"active_sites":   bson.M{"$sum": "$active_sites"},
				"last_discovery": bson.M{"$max": "$last_discovery"},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %v", err)
	}
	defer cursor.Close(ctx)

	var stats CitySitesStatistics
	if cursor.Next(ctx) {
		var result struct {
			TotalCities   int64     `bson:"total_cities"`
			TotalSites    int64     `bson:"total_sites"`
			ActiveSites   int64     `bson:"active_sites"`
			LastDiscovery time.Time `bson:"last_discovery"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode statistics: %v", err)
		}

		stats.TotalCities = result.TotalCities
		stats.TotalSites = result.TotalSites
		stats.ActiveSites = result.ActiveSites
		stats.InactiveSites = result.TotalSites - result.ActiveSites
		stats.LastDiscovery = result.LastDiscovery
	}

	// Pipeline para cidades com melhor performance
	topCitiesPipeline := []bson.M{
		{"$match": bson.M{"active_sites": bson.M{"$gt": 0}}},
		{
			"$project": bson.M{
				"city":         "$city",
				"state":        "$state",
				"active_sites": "$active_sites",
				"total_properties": bson.M{
					"$sum": "$sites.properties_found",
				},
				"success_rate": bson.M{
					"$avg": "$sites.success_rate",
				},
			},
		},
		{"$sort": bson.M{"active_sites": -1, "total_properties": -1}},
		{"$limit": 10},
	}

	topCursor, err := r.collection.Aggregate(ctx, topCitiesPipeline)
	if err == nil {
		defer topCursor.Close(ctx)
		for topCursor.Next(ctx) {
			var cityPerf CityPerformance
			if err := topCursor.Decode(&cityPerf); err == nil {
				stats.TopPerformingCities = append(stats.TopPerformingCities, cityPerf)
			}
		}
	}

	return &stats, nil
}

// CleanupInactiveSites remove sites inativos há muito tempo
func (r *MongoCitySitesRepository) CleanupInactiveSites(ctx context.Context, maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)

	// Remove sites que estão inativos há muito tempo
	filter := bson.M{
		"sites": bson.M{
			"$elemMatch": bson.M{
				"status":       bson.M{"$in": []string{"inactive", "error"}},
				"last_crawled": bson.M{"$lt": cutoff},
			},
		},
	}

	update := bson.M{
		"$pull": bson.M{
			"sites": bson.M{
				"status":       bson.M{"$in": []string{"inactive", "error"}},
				"last_crawled": bson.M{"$lt": cutoff},
			},
		},
		"$set": bson.M{
			"last_updated": time.Now(),
		},
	}

	result, err := r.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to cleanup inactive sites: %v", err)
	}

	log.Printf("Cleanup completed: updated %d cities, removed inactive sites older than %v",
		result.ModifiedCount, maxAge)

	// Remove cidades que ficaram sem sites
	emptyFilter := bson.M{
		"$or": []bson.M{
			{"sites": bson.M{"$size": 0}},
			{"sites": bson.M{"$exists": false}},
		},
	}

	deleteResult, err := r.collection.DeleteMany(ctx, emptyFilter)
	if err != nil {
		log.Printf("Warning: Failed to remove empty cities: %v", err)
	} else if deleteResult.DeletedCount > 0 {
		log.Printf("Removed %d empty cities", deleteResult.DeletedCount)
	}

	return nil
}

// Close fecha a conexão com o banco
func (r *MongoCitySitesRepository) Close() {
	if err := r.client.Disconnect(context.Background()); err != nil {
		log.Printf("Error disconnecting from MongoDB: %v", err)
	}
}

