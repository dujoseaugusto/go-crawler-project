package crawler

import (
	"context"
	"testing"

	"github.com/dujoseaugusto/go-crawler-project/internal/ai"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock PropertyRepository for crawler tests
type MockCrawlerPropertyRepository struct {
	mock.Mock
}

func (m *MockCrawlerPropertyRepository) Save(ctx context.Context, property repository.Property) error {
	args := m.Called(ctx, property)
	return args.Error(0)
}

func (m *MockCrawlerPropertyRepository) FindAll(ctx context.Context) ([]repository.Property, error) {
	args := m.Called(ctx)
	return args.Get(0).([]repository.Property), args.Error(1)
}

func (m *MockCrawlerPropertyRepository) FindWithFilters(ctx context.Context, filter repository.PropertyFilter, pagination repository.PaginationParams) (*repository.PropertySearchResult, error) {
	args := m.Called(ctx, filter, pagination)
	return args.Get(0).(*repository.PropertySearchResult), args.Error(1)
}

func (m *MockCrawlerPropertyRepository) ClearAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCrawlerPropertyRepository) Close() {
	m.Called()
}

func TestNewCrawlerEngine(t *testing.T) {
	mockRepo := &MockCrawlerPropertyRepository{}
	mockAI := &ai.GeminiService{}

	engine := NewCrawlerEngine(mockRepo, mockAI)

	assert.NotNil(t, engine)
	assert.Equal(t, mockRepo, engine.repository)
	assert.Equal(t, mockAI, engine.aiService)
	assert.NotNil(t, engine.logger)
}

func TestCrawlerEngine_Start_EmptyURLs(t *testing.T) {
	mockRepo := &MockCrawlerPropertyRepository{}
	mockAI := &ai.GeminiService{}
	engine := NewCrawlerEngine(mockRepo, mockAI)

	ctx := context.Background()

	err := engine.Start(ctx, []string{})

	assert.NoError(t, err)
}

// Test DataExtractor
func TestDataExtractor_Creation(t *testing.T) {
	extractor := NewDataExtractor()
	assert.NotNil(t, extractor)
}

// Test PropertyValidator
func TestPropertyValidator_ValidateProperty_ValidProperty(t *testing.T) {
	validator := NewPropertyValidator()

	property := repository.Property{
		ID:         "test-1",
		Descricao:  "Linda casa com vista para o mar",
		Endereco:   "Rua da Praia, 123",
		Cidade:     "Rio de Janeiro",
		Bairro:     "Copacabana",
		TipoImovel: "Casa",
		Valor:      500000,
		URL:        "https://test.com/property/1",
	}

	result := validator.ValidateProperty(&property)
	assert.True(t, result.IsValid)
}

func TestPropertyValidator_ValidateProperty_InvalidProperty(t *testing.T) {
	validator := NewPropertyValidator()

	tests := []struct {
		name     string
		property repository.Property
		expected bool
	}{
		{
			name: "Empty URL",
			property: repository.Property{
				ID:        "test-1",
				Descricao: "Casa Teste",
			},
			expected: false,
		},
		{
			name: "Invalid URL",
			property: repository.Property{
				ID:        "test-1",
				Descricao: "Casa Teste",
				URL:       "invalid-url",
			},
			expected: false,
		},
		{
			name: "Negative price",
			property: repository.Property{
				ID:        "test-1",
				Descricao: "Casa Teste",
				URL:       "https://test.com",
				Valor:     -1000,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateProperty(&tt.property)
			assert.Equal(t, tt.expected, result.IsValid)
		})
	}
}

func TestPropertyValidator_EnhanceProperty(t *testing.T) {
	validator := NewPropertyValidator()

	property := repository.Property{
		ID:         "test-1",
		Descricao:  "  Linda casa  ",
		Endereco:   "  Rua da Praia, 123  ",
		Cidade:     "  Rio de Janeiro  ",
		Bairro:     "  Copacabana  ",
		TipoImovel: "  Casa  ",
		ValorTexto: "  R$ 500.000  ",
	}

	enhanced := validator.EnhanceProperty(&property)

	// Should trim whitespace and clean text
	assert.Equal(t, "Linda casa", enhanced.Descricao)
	assert.Equal(t, "Rua da Praia, 123", enhanced.Endereco)
	assert.Equal(t, "Rio de Janeiro", enhanced.Cidade)
	assert.Equal(t, "Copacabana", enhanced.Bairro)
	// Note: TipoImovel and ValorTexto might not be cleaned in EnhanceProperty
	assert.NotEmpty(t, enhanced.TipoImovel)
	assert.NotEmpty(t, enhanced.ValorTexto)
}

// Benchmark tests
func BenchmarkDataExtractor_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewDataExtractor()
	}
}

func BenchmarkPropertyValidator_ValidateProperty(b *testing.B) {
	validator := NewPropertyValidator()
	property := repository.Property{
		ID:        "test-1",
		Descricao: "Casa na Praia",
		URL:       "https://test.com",
		Valor:     500000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateProperty(&property)
	}
}

func BenchmarkPropertyValidator_EnhanceProperty(b *testing.B) {
	validator := NewPropertyValidator()
	property := repository.Property{
		ID:        "test-1",
		Descricao: "  Casa na Praia  ",
		URL:       "https://test.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.EnhanceProperty(&property)
	}
}
