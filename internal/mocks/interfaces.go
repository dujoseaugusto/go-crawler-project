package mocks

import (
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
)

// PropertyServiceInterface defines the interface for property service
type PropertyServiceInterface interface {
	GetAllProperties(filter repository.PropertyFilter) ([]repository.Property, error)
	ForceCrawling() error
}

// PropertyRepositoryInterface defines the interface for property repository
type PropertyRepositoryInterface interface {
	Save(property repository.Property) error
	FindWithFilters(filter repository.PropertyFilter) ([]repository.Property, error)
	DeleteAll() error
	Count() (int64, error)
	Close() error
}

// URLRepositoryInterface defines the interface for URL repository
type URLRepositoryInterface interface {
	Save(url repository.ProcessedURL) error
	FindByURL(url string) (*repository.ProcessedURL, error)
	LoadAll() ([]repository.ProcessedURL, error)
	CleanupOldRecords(maxAge string) (int64, error)
	GetStatistics() (repository.URLStats, error)
	Close() error
}

// AIServiceInterface defines the interface for AI service
type AIServiceInterface interface {
	ProcessProperty(property repository.Property) (repository.Property, error)
	ProcessBatch(properties []repository.Property) ([]repository.Property, error)
}
