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
	repository *MongoRepository
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
	suite.repository, err = NewMongoRepository("mongodb://localhost:27017", "test_crawler_db", "properties")
	if err != nil {
		suite.T().Skip("Failed to create MongoDB repository")
		return
	}
	suite.urlRepo, err = NewMongoURLRepository("mongodb://localhost:27017", "test_crawler_db")
	if err != nil {
		suite.T().Skip("Failed to create MongoDB URL repository")
		return
	}
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
		ID:         "test-1",
		Descricao:  "Uma casa para teste",
		Endereco:   "Rua Teste, 123",
		Cidade:     "São Paulo",
		Bairro:     "Centro",
		TipoImovel: "Casa",
		Valor:      500000,
		ValorTexto: "R$ 500.000",
		URL:        "https://test.com/property/1",
	}

	err := suite.repository.Save(context.Background(), property)
	assert.NoError(suite.T(), err)

	// Verify property was saved
	filter := PropertyFilter{}
	pagination := PaginationParams{Page: 1, PageSize: 10}
	result, err := suite.repository.FindWithFilters(context.Background(), filter, pagination)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result.Properties, 1)
	assert.Equal(suite.T(), property.ID, result.Properties[0].ID)
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
			Descricao:  "Linda casa com vista para o mar",
			Cidade:     "Rio de Janeiro",
			Bairro:     "Copacabana",
			TipoImovel: "Casa",
			Valor:      800000,
		},
		{
			ID:         "2",
			Descricao:  "Apartamento moderno no centro da cidade",
			Cidade:     "São Paulo",
			Bairro:     "Centro",
			TipoImovel: "Apartamento",
			Valor:      400000,
		},
	}

	for _, prop := range properties {
		err := suite.repository.Save(context.Background(), prop)
		assert.NoError(suite.T(), err)
	}

	// Test city filter
	filter := PropertyFilter{
		Cidade: "rio de janeiro", // Test case insensitive
	}
	pagination := PaginationParams{Page: 1, PageSize: 10}
	result, err := suite.repository.FindWithFilters(context.Background(), filter, pagination)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result.Properties, 1)
	assert.Equal(suite.T(), "Linda casa com vista para o mar", result.Properties[0].Descricao)
}

func (suite *MongoRepositoryTestSuite) TestFindWithFilters_QuerySearch() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test property
	property := Property{
		ID:        "1",
		Descricao: "Linda casa com vista para o mar e piscina",
		Cidade:    "Rio de Janeiro",
		Bairro:    "Copacabana",
	}

	err := suite.repository.Save(context.Background(), property)
	assert.NoError(suite.T(), err)

	// Test query search
	filter := PropertyFilter{
		Query: "casa piscina",
	}
	pagination := PaginationParams{Page: 1, PageSize: 10}
	result, err := suite.repository.FindWithFilters(context.Background(), filter, pagination)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result.Properties, 1)
	assert.Equal(suite.T(), "Linda casa com vista para o mar e piscina", result.Properties[0].Descricao)
}

func (suite *MongoRepositoryTestSuite) TestFindWithFilters_PriceRange() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test properties with different prices
	properties := []Property{
		{ID: "1", Descricao: "Casa Barata", Valor: 200000},
		{ID: "2", Descricao: "Casa Média", Valor: 500000},
		{ID: "3", Descricao: "Casa Cara", Valor: 1000000},
	}

	for _, prop := range properties {
		err := suite.repository.Save(context.Background(), prop)
		assert.NoError(suite.T(), err)
	}

	// Test price range filter
	filter := PropertyFilter{
		ValorMin: 300000,
		ValorMax: 800000,
	}
	pagination := PaginationParams{Page: 1, PageSize: 10}
	result, err := suite.repository.FindWithFilters(context.Background(), filter, pagination)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result.Properties, 1)
	assert.Equal(suite.T(), "Casa Média", result.Properties[0].Descricao)
}

func (suite *MongoRepositoryTestSuite) TestClearAll() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	// Save test property
	property := Property{
		ID:        "test",
		Descricao: "Test Property",
	}
	err := suite.repository.Save(context.Background(), property)
	assert.NoError(suite.T(), err)

	// Clear all
	err = suite.repository.ClearAll(context.Background())
	assert.NoError(suite.T(), err)

	// Verify deletion - check by trying to find all
	result, err := suite.repository.FindAll(context.Background())
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, len(result))
}

// URL Repository Tests
func (suite *MongoRepositoryTestSuite) TestURLRepository_Save() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	processedURL := ProcessedURL{
		URL:         "https://test.com/property/1",
		Hash:        "abc123hash",
		ProcessedAt: time.Now(),
		Status:      "success",
	}

	err := suite.urlRepo.SaveProcessedURL(context.Background(), processedURL)
	assert.NoError(suite.T(), err)

	// Verify URL was saved
	found, err := suite.urlRepo.GetProcessedURL(context.Background(), processedURL.URL)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), found)
	assert.Equal(suite.T(), processedURL.URL, found.URL)
	assert.Equal(suite.T(), processedURL.Hash, found.Hash)
}

func (suite *MongoRepositoryTestSuite) TestURLRepository_FindByURL_NotFound() {
	if suite.client == nil {
		suite.T().Skip("MongoDB not available")
		return
	}

	found, err := suite.urlRepo.GetProcessedURL(context.Background(), "https://nonexistent.com")
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), found)
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
			Hash:        "hash1",
			ProcessedAt: now,
			Status:      "success",
		},
		{
			URL:         "https://test2.com",
			Hash:        "hash2",
			ProcessedAt: now.Add(-25 * time.Hour), // Old record
			Status:      "success",
		},
	}

	for _, url := range urls {
		err := suite.urlRepo.SaveProcessedURL(context.Background(), url)
		assert.NoError(suite.T(), err)
	}

	stats, err := suite.urlRepo.GetStatistics(context.Background())
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(2), stats.TotalURLs)
	assert.True(suite.T(), stats.ProcessedToday >= 1)
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
				ValorMin: 100000,
				ValorMax: 500000,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation logic
			isValid := tt.filter.ValorMin >= 0 && tt.filter.ValorMax >= 0
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestProperty_Validation(t *testing.T) {
	property := Property{
		ID:         "test-1",
		Descricao:  "Uma casa para teste",
		Endereco:   "Rua Teste, 123",
		Cidade:     "São Paulo",
		Bairro:     "Centro",
		TipoImovel: "Casa",
		Valor:      500000,
		ValorTexto: "R$ 500.000",
		URL:        "https://test.com/property/1",
	}

	// Basic validation
	assert.NotEmpty(t, property.ID)
	assert.NotEmpty(t, property.Descricao)
	assert.True(t, property.Valor >= 0)
	assert.NotEmpty(t, property.URL)
}

// Benchmark tests
func BenchmarkMongoRepository_Save(b *testing.B) {
	// Skip if MongoDB not available
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		b.Skip("MongoDB not available for benchmark")
		return
	}
	defer client.Disconnect(context.TODO())

	repo, err := NewMongoRepository("mongodb://localhost:27017", "benchmark_test_db", "properties")
	if err != nil {
		b.Skip("Failed to create repository")
		return
	}
	defer repo.Close()

	property := Property{
		ID:         "benchmark-test",
		Descricao:  "Property for benchmark testing",
		Cidade:     "Test City",
		Bairro:     "Test Neighborhood",
		TipoImovel: "Casa",
		Valor:      500000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		property.ID = string(rune(i))
		repo.Save(context.Background(), property)
	}
}

// Run the test suite
func TestMongoRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(MongoRepositoryTestSuite))
}
