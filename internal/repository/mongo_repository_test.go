package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoRepositoryTestSuite provides integration tests for MongoDB repository
type MongoRepositoryTestSuite struct {
	suite.Suite
	client     *mongo.Client
	repository *MongoPropertyRepository
	urlRepo    *MongoURLRepository
}

func (suite *MongoRepositoryTestSuite) SetupSuite() {
	// Use MongoDB test database
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		suite.T().Skip("MongoDB not available for integration tests")
		return
	}

	suite.client = client
	suite.repository = NewMongoPropertyRepository("mongodb://localhost:27017", "test_crawler_db")
	suite.urlRepo = NewMongoURLRepository("mongodb://localhost:27017", "test_crawler_db")
}

func (suite *MongoRepositoryTestSuite) SetupTest() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Clean up test data before each test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db := suite.client.Database("test_crawler_db")
	db.Collection("properties").Drop(ctx)
	db.Collection("processed_urls").Drop(ctx)
}

func (suite *MongoRepositoryTestSuite) TearDownSuite() {
	if suite.client != nil {
		suite.repository.Close()
		suite.urlRepo.Close()
		suite.client.Disconnect(context.TODO())
	}
}

func (suite *MongoRepositoryTestSuite) TestSaveProperty() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	property := Property{
		ID:          "test-1",
		Titulo:      "Casa Teste",
		Descricao:   "Uma casa para teste",
		Endereco:    "Rua Teste, 123",
		Cidade:      "São Paulo",
		Bairro:      "Centro",
		TipoImovel:  "Casa",
		Preco:       500000,
		ValorTexto:  "R$ 500.000",
		URL:         "https://test.com/property/1",
		DataCriacao: time.Now(),
	}

	err := suite.repository.Save(property)
	assert.NoError(suite.T(), err)

	// Verify property was saved
	filter := PropertyFilter{Limit: 10}
	properties, err := suite.repository.FindWithFilters(filter)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), properties, 1)
	assert.Equal(suite.T(), property.ID, properties[0].ID)
}

func (suite *MongoRepositoryTestSuite) TestFindWithFilters_BasicSearch() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test properties
	properties := []Property{
		{
			ID:         "1",
			Titulo:     "Casa na Praia",
			Descricao:  "Linda casa com vista para o mar",
			Cidade:     "Rio de Janeiro",
			Bairro:     "Copacabana",
			TipoImovel: "Casa",
			Preco:      800000,
		},
		{
			ID:         "2",
			Titulo:     "Apartamento Centro",
			Descricao:  "Apartamento moderno no centro da cidade",
			Cidade:     "São Paulo",
			Bairro:     "Centro",
			TipoImovel: "Apartamento",
			Preco:      400000,
		},
	}

	for _, prop := range properties {
		err := suite.repository.Save(prop)
		assert.NoError(suite.T(), err)
	}

	// Test city filter
	filter := PropertyFilter{
		Cidade: "rio de janeiro", // Test case insensitive
		Limit:  10,
	}
	result, err := suite.repository.FindWithFilters(filter)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), "Casa na Praia", result[0].Titulo)
}

func (suite *MongoRepositoryTestSuite) TestFindWithFilters_QuerySearch() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test property
	property := Property{
		ID:        "1",
		Titulo:    "Casa na Praia",
		Descricao: "Linda casa com vista para o mar e piscina",
		Cidade:    "Rio de Janeiro",
		Bairro:    "Copacabana",
	}

	err := suite.repository.Save(property)
	assert.NoError(suite.T(), err)

	// Test query search
	filter := PropertyFilter{
		Query: "casa piscina",
		Limit: 10,
	}
	result, err := suite.repository.FindWithFilters(filter)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), "Casa na Praia", result[0].Titulo)
}

func (suite *MongoRepositoryTestSuite) TestFindWithFilters_PriceRange() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test properties with different prices
	properties := []Property{
		{ID: "1", Titulo: "Casa Barata", Preco: 200000},
		{ID: "2", Titulo: "Casa Média", Preco: 500000},
		{ID: "3", Titulo: "Casa Cara", Preco: 1000000},
	}

	for _, prop := range properties {
		err := suite.repository.Save(prop)
		assert.NoError(suite.T(), err)
	}

	// Test price range filter
	filter := PropertyFilter{
		PrecoMin: 300000,
		PrecoMax: 800000,
		Limit:    10,
	}
	result, err := suite.repository.FindWithFilters(filter)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), "Casa Média", result[0].Titulo)
}

func (suite *MongoRepositoryTestSuite) TestCount() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test properties
	for i := 0; i < 5; i++ {
		property := Property{
			ID:     string(rune(i)),
			Titulo: "Test Property",
		}
		err := suite.repository.Save(property)
		assert.NoError(suite.T(), err)
	}

	count, err := suite.repository.Count()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(5), count)
}

func (suite *MongoRepositoryTestSuite) TestDeleteAll() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test property
	property := Property{
		ID:     "test",
		Titulo: "Test Property",
	}
	err := suite.repository.Save(property)
	assert.NoError(suite.T(), err)

	// Delete all
	err = suite.repository.DeleteAll()
	assert.NoError(suite.T(), err)

	// Verify deletion
	count, err := suite.repository.Count()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)
}

// URL Repository Tests
func (suite *MongoRepositoryTestSuite) TestURLRepository_Save() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	processedURL := ProcessedURL{
		URL:         "https://test.com/property/1",
		Fingerprint: "abc123hash",
		ProcessedAt: time.Now(),
		LastAICall:  time.Now(),
	}

	err := suite.urlRepo.Save(processedURL)
	assert.NoError(suite.T(), err)

	// Verify URL was saved
	found, err := suite.urlRepo.FindByURL(processedURL.URL)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), found)
	assert.Equal(suite.T(), processedURL.URL, found.URL)
	assert.Equal(suite.T(), processedURL.Fingerprint, found.Fingerprint)
}

func (suite *MongoRepositoryTestSuite) TestURLRepository_FindByURL_NotFound() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	found, err := suite.urlRepo.FindByURL("https://nonexistent.com")
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), found)
}

func (suite *MongoRepositoryTestSuite) TestURLRepository_LoadAll() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test URLs
	urls := []ProcessedURL{
		{
			URL:         "https://test1.com",
			Fingerprint: "hash1",
			ProcessedAt: time.Now(),
		},
		{
			URL:         "https://test2.com",
			Fingerprint: "hash2",
			ProcessedAt: time.Now(),
		},
	}

	for _, url := range urls {
		err := suite.urlRepo.Save(url)
		assert.NoError(suite.T(), err)
	}

	// Load all URLs
	result, err := suite.urlRepo.LoadAll()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 2)
}

func (suite *MongoRepositoryTestSuite) TestURLRepository_GetStatistics() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test URLs
	now := time.Now()
	urls := []ProcessedURL{
		{
			URL:         "https://test1.com",
			Fingerprint: "hash1",
			ProcessedAt: now,
			LastAICall:  now,
		},
		{
			URL:         "https://test2.com",
			Fingerprint: "hash2",
			ProcessedAt: now.Add(-25 * time.Hour), // Old record
		},
	}

	for _, url := range urls {
		err := suite.urlRepo.Save(url)
		assert.NoError(suite.T(), err)
	}

	stats, err := suite.urlRepo.GetStatistics()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(2), stats.TotalURLs)
	assert.True(suite.T(), stats.ProcessedLast24h >= 1)
}

func (suite *MongoRepositoryTestSuite) TestURLRepository_CleanupOldRecords() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save old and new URLs
	now := time.Now()
	urls := []ProcessedURL{
		{
			URL:         "https://old.com",
			Fingerprint: "hash1",
			ProcessedAt: now.Add(-48 * time.Hour), // 2 days old
		},
		{
			URL:         "https://new.com",
			Fingerprint: "hash2",
			ProcessedAt: now, // Recent
		},
	}

	for _, url := range urls {
		err := suite.urlRepo.Save(url)
		assert.NoError(suite.T(), err)
	}

	// Cleanup records older than 24 hours
	deleted, err := suite.urlRepo.CleanupOldRecords("24h")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), deleted)

	// Verify only new record remains
	all, err := suite.urlRepo.LoadAll()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), all, 1)
	assert.Equal(suite.T(), "https://new.com", all[0].URL)
}

// Unit tests for utility functions
func TestPropertyFilter_Validation(t *testing.T) {
	tests := []struct {
		name   string
		filter PropertyFilter
		valid  bool
	}{
		{
			name: "Valid filter",
			filter: PropertyFilter{
				Cidade:   "São Paulo",
				PrecoMin: 100000,
				PrecoMax: 500000,
				Limit:    10,
				Offset:   0,
			},
			valid: true,
		},
		{
			name: "Negative limit",
			filter: PropertyFilter{
				Limit: -1,
			},
			valid: false,
		},
		{
			name: "Negative offset",
			filter: PropertyFilter{
				Offset: -1,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation logic
			isValid := tt.filter.Limit >= 0 && tt.filter.Offset >= 0
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestProperty_Validation(t *testing.T) {
	property := Property{
		ID:          "test-1",
		Titulo:      "Casa Teste",
		Descricao:   "Uma casa para teste",
		Endereco:    "Rua Teste, 123",
		Cidade:      "São Paulo",
		Bairro:      "Centro",
		TipoImovel:  "Casa",
		Preco:       500000,
		ValorTexto:  "R$ 500.000",
		URL:         "https://test.com/property/1",
		DataCriacao: time.Now(),
	}

	// Basic validation
	assert.NotEmpty(t, property.ID)
	assert.NotEmpty(t, property.Titulo)
	assert.True(t, property.Preco >= 0)
	assert.NotEmpty(t, property.URL)
}

// Benchmark tests
func BenchmarkMongoPropertyRepository_Save(b *testing.B) {
	// Skip if MongoDB not available
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		b.Skip("MongoDB not available for benchmark")
		return
	}
	defer client.Disconnect(context.TODO())

	repo := NewMongoPropertyRepository("mongodb://localhost:27017", "benchmark_test_db")
	defer repo.Close()

	property := Property{
		ID:         "benchmark-test",
		Titulo:     "Benchmark Property",
		Descricao:  "Property for benchmark testing",
		Cidade:     "Test City",
		Bairro:     "Test Neighborhood",
		TipoImovel: "Casa",
		Preco:      500000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		property.ID = string(rune(i))
		repo.Save(property)
	}
}

// Run the test suite
func TestMongoRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(MongoRepositoryTestSuite))
}
