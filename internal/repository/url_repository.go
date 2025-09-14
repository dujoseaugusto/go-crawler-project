package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ProcessedURL representa uma URL que já foi processada
type ProcessedURL struct {
	URL         string    `bson:"_id" json:"url"`
	ProcessedAt time.Time `bson:"processed_at" json:"processed_at"`
	Status      string    `bson:"status" json:"status"` // "success", "failed", "skipped"
	Hash        string    `bson:"hash,omitempty" json:"hash,omitempty"`
	ErrorMsg    string    `bson:"error_msg,omitempty" json:"error_msg,omitempty"`
}

// PageFingerprint representa a "impressão digital" de uma página
type PageFingerprint struct {
	URL            string    `bson:"_id" json:"url"`
	ContentHash    string    `bson:"content_hash" json:"content_hash"`
	LastModified   time.Time `bson:"last_modified" json:"last_modified"`
	PropertyCount  int       `bson:"property_count" json:"property_count"`
	LastCrawled    time.Time `bson:"last_crawled" json:"last_crawled"`
	ChangeDetected bool      `bson:"change_detected" json:"change_detected"`
	AIProcessed    bool      `bson:"ai_processed" json:"ai_processed"`
	ProcessingTime float64   `bson:"processing_time" json:"processing_time"` // em segundos
}

// URLRepository define as operações para gerenciar URLs processadas
type URLRepository interface {
	SaveProcessedURL(ctx context.Context, url ProcessedURL) error
	GetProcessedURL(ctx context.Context, url string) (*ProcessedURL, error)
	GetProcessedURLsSince(ctx context.Context, since time.Time) ([]ProcessedURL, error)
	IsURLProcessedRecently(ctx context.Context, url string, maxAge time.Duration) (bool, error)

	SaveFingerprint(ctx context.Context, fingerprint PageFingerprint) error
	GetFingerprint(ctx context.Context, url string) (*PageFingerprint, error)
	UpdateFingerprint(ctx context.Context, fingerprint PageFingerprint) error
	GetFingerprintsRequiringUpdate(ctx context.Context, maxAge time.Duration) ([]PageFingerprint, error)

	CleanupOldRecords(ctx context.Context, maxAge time.Duration) error
	GetStatistics(ctx context.Context) (*URLStatistics, error)
	Close()
}

// URLStatistics fornece estatísticas sobre URLs processadas
type URLStatistics struct {
	TotalURLs          int64     `json:"total_urls"`
	ProcessedToday     int64     `json:"processed_today"`
	SuccessfulToday    int64     `json:"successful_today"`
	FailedToday        int64     `json:"failed_today"`
	SkippedToday       int64     `json:"skipped_today"`
	LastCleanup        time.Time `json:"last_cleanup"`
	AverageProcessTime float64   `json:"average_process_time"`
	TotalAISavings     int64     `json:"total_ai_savings"` // URLs que não precisaram de IA
}

// MongoURLRepository implementa URLRepository usando MongoDB
type MongoURLRepository struct {
	client                *mongo.Client
	urlCollection         *mongo.Collection
	fingerprintCollection *mongo.Collection
}

// NewMongoURLRepository cria um novo repositório de URLs
func NewMongoURLRepository(uri, dbName string) (*MongoURLRepository, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(context.Background(), nil); err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	urlCollection := db.Collection("processed_urls")
	fingerprintCollection := db.Collection("page_fingerprints")

	repo := &MongoURLRepository{
		client:                client,
		urlCollection:         urlCollection,
		fingerprintCollection: fingerprintCollection,
	}

	// Criar índices necessários
	if err := repo.createIndexes(); err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	}

	return repo, nil
}

// createIndexes cria os índices necessários para performance
func (r *MongoURLRepository) createIndexes() error {
	ctx := context.Background()

	// Índice para processed_urls
	urlIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "processed_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "processed_at", Value: -1}, {Key: "status", Value: 1}},
		},
	}

	_, err := r.urlCollection.Indexes().CreateMany(ctx, urlIndexes)
	if err != nil {
		return fmt.Errorf("failed to create URL indexes: %v", err)
	}

	// Índice para page_fingerprints
	fingerprintIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "last_crawled", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "content_hash", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "change_detected", Value: 1}, {Key: "last_crawled", Value: -1}},
		},
	}

	_, err = r.fingerprintCollection.Indexes().CreateMany(ctx, fingerprintIndexes)
	if err != nil {
		return fmt.Errorf("failed to create fingerprint indexes: %v", err)
	}

	log.Printf("URL repository indexes created successfully")
	return nil
}

// SaveProcessedURL salva uma URL processada
func (r *MongoURLRepository) SaveProcessedURL(ctx context.Context, url ProcessedURL) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": url.URL}
	update := bson.M{"$set": url}

	_, err := r.urlCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save processed URL: %v", err)
	}

	return nil
}

// GetProcessedURL recupera uma URL processada
func (r *MongoURLRepository) GetProcessedURL(ctx context.Context, url string) (*ProcessedURL, error) {
	var result ProcessedURL
	err := r.urlCollection.FindOne(ctx, bson.M{"_id": url}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get processed URL: %v", err)
	}

	return &result, nil
}

// GetProcessedURLsSince recupera URLs processadas desde uma data
func (r *MongoURLRepository) GetProcessedURLsSince(ctx context.Context, since time.Time) ([]ProcessedURL, error) {
	filter := bson.M{"processed_at": bson.M{"$gte": since}}
	cursor, err := r.urlCollection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find processed URLs: %v", err)
	}
	defer cursor.Close(ctx)

	var urls []ProcessedURL
	if err := cursor.All(ctx, &urls); err != nil {
		return nil, fmt.Errorf("failed to decode processed URLs: %v", err)
	}

	return urls, nil
}

// IsURLProcessedRecently verifica se uma URL foi processada recentemente
func (r *MongoURLRepository) IsURLProcessedRecently(ctx context.Context, url string, maxAge time.Duration) (bool, error) {
	since := time.Now().Add(-maxAge)
	filter := bson.M{
		"_id":          url,
		"processed_at": bson.M{"$gte": since},
		"status":       "success",
	}

	count, err := r.urlCollection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("failed to check if URL processed recently: %v", err)
	}

	return count > 0, nil
}

// SaveFingerprint salva um fingerprint de página
func (r *MongoURLRepository) SaveFingerprint(ctx context.Context, fingerprint PageFingerprint) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": fingerprint.URL}
	update := bson.M{"$set": fingerprint}

	_, err := r.fingerprintCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save fingerprint: %v", err)
	}

	return nil
}

// GetFingerprint recupera um fingerprint de página
func (r *MongoURLRepository) GetFingerprint(ctx context.Context, url string) (*PageFingerprint, error) {
	var result PageFingerprint
	err := r.fingerprintCollection.FindOne(ctx, bson.M{"_id": url}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get fingerprint: %v", err)
	}

	return &result, nil
}

// UpdateFingerprint atualiza um fingerprint existente
func (r *MongoURLRepository) UpdateFingerprint(ctx context.Context, fingerprint PageFingerprint) error {
	filter := bson.M{"_id": fingerprint.URL}
	update := bson.M{"$set": fingerprint}

	result, err := r.fingerprintCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update fingerprint: %v", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("fingerprint not found for URL: %s", fingerprint.URL)
	}

	return nil
}

// GetFingerprintsRequiringUpdate recupera fingerprints que precisam ser atualizados
func (r *MongoURLRepository) GetFingerprintsRequiringUpdate(ctx context.Context, maxAge time.Duration) ([]PageFingerprint, error) {
	since := time.Now().Add(-maxAge)
	filter := bson.M{
		"last_crawled": bson.M{"$lt": since},
	}

	cursor, err := r.fingerprintCollection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find fingerprints requiring update: %v", err)
	}
	defer cursor.Close(ctx)

	var fingerprints []PageFingerprint
	if err := cursor.All(ctx, &fingerprints); err != nil {
		return nil, fmt.Errorf("failed to decode fingerprints: %v", err)
	}

	return fingerprints, nil
}

// CleanupOldRecords remove registros antigos
func (r *MongoURLRepository) CleanupOldRecords(ctx context.Context, maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)

	// Limpar URLs processadas antigas
	urlFilter := bson.M{"processed_at": bson.M{"$lt": cutoff}}
	urlResult, err := r.urlCollection.DeleteMany(ctx, urlFilter)
	if err != nil {
		return fmt.Errorf("failed to cleanup old processed URLs: %v", err)
	}

	// Limpar fingerprints antigos
	fingerprintFilter := bson.M{"last_crawled": bson.M{"$lt": cutoff}}
	fingerprintResult, err := r.fingerprintCollection.DeleteMany(ctx, fingerprintFilter)
	if err != nil {
		return fmt.Errorf("failed to cleanup old fingerprints: %v", err)
	}

	log.Printf("Cleanup completed: removed %d URLs and %d fingerprints older than %v",
		urlResult.DeletedCount, fingerprintResult.DeletedCount, maxAge)

	return nil
}

// GetStatistics retorna estatísticas sobre URLs processadas
func (r *MongoURLRepository) GetStatistics(ctx context.Context) (*URLStatistics, error) {
	today := time.Now().Truncate(24 * time.Hour)

	// Total de URLs
	totalURLs, err := r.urlCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to count total URLs: %v", err)
	}

	// URLs processadas hoje
	todayFilter := bson.M{"processed_at": bson.M{"$gte": today}}
	processedToday, err := r.urlCollection.CountDocuments(ctx, todayFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to count processed URLs today: %v", err)
	}

	// URLs bem-sucedidas hoje
	successFilter := bson.M{
		"processed_at": bson.M{"$gte": today},
		"status":       "success",
	}
	successfulToday, err := r.urlCollection.CountDocuments(ctx, successFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to count successful URLs today: %v", err)
	}

	// URLs com falha hoje
	failedFilter := bson.M{
		"processed_at": bson.M{"$gte": today},
		"status":       "failed",
	}
	failedToday, err := r.urlCollection.CountDocuments(ctx, failedFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to count failed URLs today: %v", err)
	}

	// URLs puladas hoje
	skippedFilter := bson.M{
		"processed_at": bson.M{"$gte": today},
		"status":       "skipped",
	}
	skippedToday, err := r.urlCollection.CountDocuments(ctx, skippedFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to count skipped URLs today: %v", err)
	}

	// Total de economia de IA (fingerprints que não mudaram)
	aiSavingsFilter := bson.M{
		"change_detected": false,
		"ai_processed":    false,
	}
	aiSavings, err := r.fingerprintCollection.CountDocuments(ctx, aiSavingsFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to count AI savings: %v", err)
	}

	return &URLStatistics{
		TotalURLs:       totalURLs,
		ProcessedToday:  processedToday,
		SuccessfulToday: successfulToday,
		FailedToday:     failedToday,
		SkippedToday:    skippedToday,
		TotalAISavings:  aiSavings,
		LastCleanup:     time.Now(), // Seria melhor armazenar isso separadamente
	}, nil
}

// Close fecha a conexão com o banco
func (r *MongoURLRepository) Close() {
	if err := r.client.Disconnect(context.Background()); err != nil {
		log.Printf("Error disconnecting from MongoDB: %v", err)
	}
}

// GenerateContentHash gera um hash do conteúdo relevante da página
func GenerateContentHash(prices []string, addresses []string, descriptions []string) string {
	// Normaliza e combina o conteúdo relevante
	content := fmt.Sprintf("prices:%v|addresses:%v|descriptions:%v",
		normalizeStringSlice(prices),
		normalizeStringSlice(addresses),
		normalizeStringSlice(descriptions))

	// Gera hash SHA-256
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// normalizeStringSlice normaliza uma slice de strings para comparação
func normalizeStringSlice(slice []string) []string {
	var normalized []string
	for _, s := range slice {
		clean := strings.TrimSpace(strings.ToLower(s))
		if clean != "" {
			normalized = append(normalized, clean)
		}
	}
	return normalized
}

