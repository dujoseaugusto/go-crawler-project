package service

import (
	"context"
	"testing"

	"github.com/dujoseaugusto/go-crawler-project/internal/config"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock PropertyRepository
type MockPropertyRepository struct {
	mock.Mock
}

func (m *MockPropertyRepository) Save(ctx context.Context, property repository.Property) error {
	args := m.Called(ctx, property)
	return args.Error(0)
}

func (m *MockPropertyRepository) FindAll(ctx context.Context) ([]repository.Property, error) {
	args := m.Called(ctx)
	return args.Get(0).([]repository.Property), args.Error(1)
}

func (m *MockPropertyRepository) FindWithFilters(ctx context.Context, filter repository.PropertyFilter, pagination repository.PaginationParams) (*repository.PropertySearchResult, error) {
	args := m.Called(ctx, filter, pagination)
	return args.Get(0).(*repository.PropertySearchResult), args.Error(1)
}

func (m *MockPropertyRepository) ClearAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockPropertyRepository) Close() {
	m.Called()
}

// Test setup
func setupTestService() (*PropertyService, *MockPropertyRepository) {
	mockRepo := &MockPropertyRepository{}
	testConfig := &config.Config{}

	service := &PropertyService{
		repo:   mockRepo,
		config: testConfig,
		logger: logger.NewLogger("test"),
	}

	return service, mockRepo
}

func TestNewPropertyService(t *testing.T) {
	mockRepo := &MockPropertyRepository{}
	testConfig := &config.Config{}

	service := NewPropertyService(mockRepo, testConfig)

	assert.NotNil(t, service)
	assert.Equal(t, mockRepo, service.repo)
	assert.Equal(t, testConfig, service.config)
	assert.NotNil(t, service.logger)
}

func TestSaveProperty_Success(t *testing.T) {
	service, mockRepo := setupTestService()
	ctx := context.Background()

	property := repository.Property{
		ID:        "1",
		Endereco:  "Rua Teste, 123",
		Cidade:    "São Paulo",
		Descricao: "Casa para teste",
	}

	mockRepo.On("Save", ctx, property).Return(nil)

	err := service.SaveProperty(ctx, property)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestSaveProperty_EmptyFields(t *testing.T) {
	service, _ := setupTestService()
	ctx := context.Background()

	tests := []struct {
		name     string
		property repository.Property
	}{
		{
			name: "Empty endereco",
			property: repository.Property{
				ID:        "1",
				Cidade:    "São Paulo",
				Descricao: "Casa para teste",
			},
		},
		{
			name: "Empty cidade",
			property: repository.Property{
				ID:        "1",
				Endereco:  "Rua Teste, 123",
				Descricao: "Casa para teste",
			},
		},
		{
			name: "Empty descricao",
			property: repository.Property{
				ID:       "1",
				Endereco: "Rua Teste, 123",
				Cidade:   "São Paulo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SaveProperty(ctx, tt.property)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "property fields cannot be empty")
		})
	}
}

func TestGetAllProperties_Success(t *testing.T) {
	service, mockRepo := setupTestService()
	ctx := context.Background()

	expectedProperties := []repository.Property{
		{
			ID:        "1",
			Endereco:  "Rua da Praia, 123",
			Cidade:    "Rio de Janeiro",
			Descricao: "Linda casa com vista para o mar",
		},
		{
			ID:        "2",
			Endereco:  "Av. Paulista, 456",
			Cidade:    "São Paulo",
			Descricao: "Apartamento moderno no centro",
		},
	}

	mockRepo.On("FindAll", ctx).Return(expectedProperties, nil)

	result, err := service.GetAllProperties(ctx)

	assert.NoError(t, err)
	assert.Equal(t, expectedProperties, result)
	mockRepo.AssertExpectations(t)
}

func TestSearchProperties_Success(t *testing.T) {
	service, mockRepo := setupTestService()
	ctx := context.Background()

	filter := repository.PropertyFilter{
		Cidade: "São Paulo",
	}

	pagination := repository.PaginationParams{
		Page:     1,
		PageSize: 10,
	}

	expectedResult := &repository.PropertySearchResult{
		Properties: []repository.Property{
			{
				ID:        "1",
				Endereco:  "Av. Paulista, 456",
				Cidade:    "São Paulo",
				Descricao: "Apartamento moderno",
			},
		},
		TotalItems:  1,
		TotalPages:  1,
		CurrentPage: 1,
		PageSize:    10,
	}

	mockRepo.On("FindWithFilters", ctx, filter, pagination).Return(expectedResult, nil)

	result, err := service.SearchProperties(ctx, filter, pagination)

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockRepo.AssertExpectations(t)
}

// Benchmark tests
func BenchmarkSaveProperty(b *testing.B) {
	service, mockRepo := setupTestService()
	ctx := context.Background()

	property := repository.Property{
		ID:        "test",
		Endereco:  "Rua Teste, 123",
		Cidade:    "São Paulo",
		Descricao: "Casa para benchmark",
	}

	mockRepo.On("Save", ctx, property).Return(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.SaveProperty(ctx, property)
	}
}

func BenchmarkGetAllProperties(b *testing.B) {
	service, mockRepo := setupTestService()
	ctx := context.Background()

	properties := make([]repository.Property, 1000)
	for i := 0; i < 1000; i++ {
		properties[i] = repository.Property{
			ID:        string(rune(i)),
			Endereco:  "Rua Teste",
			Cidade:    "São Paulo",
			Descricao: "Test Property",
		}
	}

	mockRepo.On("FindAll", ctx).Return(properties, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetAllProperties(ctx)
	}
}
