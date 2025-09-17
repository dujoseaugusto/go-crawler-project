package mocks

import (
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockPropertyService is a mock implementation of PropertyServiceInterface
type MockPropertyService struct {
	mock.Mock
}

func (m *MockPropertyService) GetAllProperties(filter repository.PropertyFilter) ([]repository.Property, error) {
	args := m.Called(filter)
	return args.Get(0).([]repository.Property), args.Error(1)
}

func (m *MockPropertyService) ForceCrawling() error {
	args := m.Called()
	return args.Error(0)
}

// MockPropertyRepository is a mock implementation of PropertyRepositoryInterface
type MockPropertyRepository struct {
	mock.Mock
}

func (m *MockPropertyRepository) Save(property repository.Property) error {
	args := m.Called(property)
	return args.Error(0)
}

func (m *MockPropertyRepository) FindWithFilters(filter repository.PropertyFilter) ([]repository.Property, error) {
	args := m.Called(filter)
	return args.Get(0).([]repository.Property), args.Error(1)
}

func (m *MockPropertyRepository) DeleteAll() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockPropertyRepository) Count() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPropertyRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockURLRepository is a mock implementation of URLRepositoryInterface
type MockURLRepository struct {
	mock.Mock
}

func (m *MockURLRepository) Save(url repository.ProcessedURL) error {
	args := m.Called(url)
	return args.Error(0)
}

func (m *MockURLRepository) FindByURL(url string) (*repository.ProcessedURL, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ProcessedURL), args.Error(1)
}

func (m *MockURLRepository) LoadAll() ([]repository.ProcessedURL, error) {
	args := m.Called()
	return args.Get(0).([]repository.ProcessedURL), args.Error(1)
}

func (m *MockURLRepository) CleanupOldRecords(maxAge string) (int64, error) {
	args := m.Called(maxAge)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockURLRepository) GetStatistics() (*repository.URLStatistics, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.URLStatistics), args.Error(1)
}

func (m *MockURLRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockAIService is a mock implementation of AIServiceInterface
type MockAIService struct {
	mock.Mock
}

func (m *MockAIService) ProcessProperty(property repository.Property) (repository.Property, error) {
	args := m.Called(property)
	return args.Get(0).(repository.Property), args.Error(1)
}

func (m *MockAIService) ProcessBatch(properties []repository.Property) ([]repository.Property, error) {
	args := m.Called(properties)
	return args.Get(0).([]repository.Property), args.Error(1)
}
