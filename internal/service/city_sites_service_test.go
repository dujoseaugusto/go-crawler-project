package service

import (
	"context"
	"testing"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCitySitesRepository é um mock do repositório para testes
type MockCitySitesRepository struct {
	mock.Mock
}

func (m *MockCitySitesRepository) SaveCitySites(ctx context.Context, citySites repository.CitySites) error {
	args := m.Called(ctx, citySites)
	return args.Error(0)
}

func (m *MockCitySitesRepository) FindByCity(ctx context.Context, city, state string) (*repository.CitySites, error) {
	args := m.Called(ctx, city, state)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.CitySites), args.Error(1)
}

func (m *MockCitySitesRepository) FindAllCities(ctx context.Context) ([]repository.CitySites, error) {
	args := m.Called(ctx)
	return args.Get(0).([]repository.CitySites), args.Error(1)
}

func (m *MockCitySitesRepository) FindCitiesByNames(ctx context.Context, cities []string) ([]repository.CitySites, error) {
	args := m.Called(ctx, cities)
	return args.Get(0).([]repository.CitySites), args.Error(1)
}

func (m *MockCitySitesRepository) FindCitiesByRegion(ctx context.Context, region string) ([]repository.CitySites, error) {
	args := m.Called(ctx, region)
	return args.Get(0).([]repository.CitySites), args.Error(1)
}

func (m *MockCitySitesRepository) GetAllActiveSites(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCitySitesRepository) GetSitesByCity(ctx context.Context, city string) ([]string, error) {
	args := m.Called(ctx, city)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCitySitesRepository) GetSitesByCities(ctx context.Context, cities []string) ([]string, error) {
	args := m.Called(ctx, cities)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCitySitesRepository) UpdateSiteStatus(ctx context.Context, city, state, url, status string) error {
	args := m.Called(ctx, city, state, url, status)
	return args.Error(0)
}

func (m *MockCitySitesRepository) UpdateSiteStats(ctx context.Context, city, state, url string, stats repository.SiteStats) error {
	args := m.Called(ctx, city, state, url, stats)
	return args.Error(0)
}

func (m *MockCitySitesRepository) DeleteCity(ctx context.Context, city, state string) error {
	args := m.Called(ctx, city, state)
	return args.Error(0)
}

func (m *MockCitySitesRepository) GetStatistics(ctx context.Context) (*repository.CitySitesStatistics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.CitySitesStatistics), args.Error(1)
}

func (m *MockCitySitesRepository) CleanupInactiveSites(ctx context.Context, maxAge time.Duration) error {
	args := m.Called(ctx, maxAge)
	return args.Error(0)
}

func (m *MockCitySitesRepository) Close() {
	m.Called()
}

func TestNewCitySitesService(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	assert.NotNil(t, service)
	assert.NotNil(t, service.repository)
	assert.NotNil(t, service.discoveryEngine)
	assert.NotNil(t, service.logger)
}

func TestCitySitesService_GetAllCities(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	expectedCities := []repository.CitySites{
		{
			City:        "Muzambinho",
			State:       "MG",
			TotalSites:  5,
			ActiveSites: 3,
		},
		{
			City:        "Alfenas",
			State:       "MG",
			TotalSites:  8,
			ActiveSites: 6,
		},
	}

	mockRepo.On("FindAllCities", mock.Anything).Return(expectedCities, nil)

	ctx := context.Background()
	cities, err := service.GetAllCities(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(cities))
	assert.Equal(t, "Muzambinho", cities[0].City)
	assert.Equal(t, "Alfenas", cities[1].City)

	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_GetCityByName(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	expectedCity := &repository.CitySites{
		City:        "Muzambinho",
		State:       "MG",
		TotalSites:  5,
		ActiveSites: 3,
		Sites: []repository.SiteInfo{
			{URL: "http://example1.com", Status: "active"},
			{URL: "http://example2.com", Status: "active"},
			{URL: "http://example3.com", Status: "inactive"},
		},
	}

	mockRepo.On("FindByCity", mock.Anything, "Muzambinho", "MG").Return(expectedCity, nil)

	ctx := context.Background()
	city, err := service.GetCityByName(ctx, "Muzambinho", "MG")

	assert.NoError(t, err)
	assert.NotNil(t, city)
	assert.Equal(t, "Muzambinho", city.City)
	assert.Equal(t, "MG", city.State)
	assert.Equal(t, 5, city.TotalSites)
	assert.Equal(t, 3, city.ActiveSites)

	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_GetCityByName_NotFound(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	mockRepo.On("FindByCity", mock.Anything, "NonExistent", "MG").Return(nil, nil)

	ctx := context.Background()
	city, err := service.GetCityByName(ctx, "NonExistent", "MG")

	assert.Error(t, err)
	assert.Nil(t, city)
	assert.Contains(t, err.Error(), "city not found")

	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_GetSitesForCrawling_AllCities(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	expectedURLs := []string{
		"http://example1.com",
		"http://example2.com",
		"http://example3.com",
	}

	mockRepo.On("GetAllActiveSites", mock.Anything).Return(expectedURLs, nil)

	ctx := context.Background()
	urls, err := service.GetSitesForCrawling(ctx, []string{})

	assert.NoError(t, err)
	assert.Equal(t, 3, len(urls))
	assert.Contains(t, urls, "http://example1.com")
	assert.Contains(t, urls, "http://example2.com")
	assert.Contains(t, urls, "http://example3.com")

	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_GetSitesForCrawling_SpecificCities(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	cities := []string{"Muzambinho", "Alfenas"}
	expectedURLs := []string{
		"http://muzambinho1.com",
		"http://alfenas1.com",
	}

	mockRepo.On("GetSitesByCities", mock.Anything, cities).Return(expectedURLs, nil)

	ctx := context.Background()
	urls, err := service.GetSitesForCrawling(ctx, cities)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(urls))
	assert.Contains(t, urls, "http://muzambinho1.com")
	assert.Contains(t, urls, "http://alfenas1.com")

	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_AddSiteToCity(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	existingCity := &repository.CitySites{
		City:  "Muzambinho",
		State: "MG",
		Sites: []repository.SiteInfo{
			{URL: "http://existing.com", Status: "active"},
		},
	}

	newSite := repository.SiteInfo{
		URL:    "http://newsite.com",
		Name:   "New Site",
		Status: "active",
	}

	mockRepo.On("FindByCity", mock.Anything, "Muzambinho", "MG").Return(existingCity, nil)
	mockRepo.On("SaveCitySites", mock.Anything, mock.MatchedBy(func(cs repository.CitySites) bool {
		return cs.City == "Muzambinho" && cs.State == "MG" && len(cs.Sites) == 2
	})).Return(nil)

	ctx := context.Background()
	err := service.AddSiteToCity(ctx, "Muzambinho", "MG", newSite)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_AddSiteToCity_NewCity(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	newSite := repository.SiteInfo{
		URL:    "http://newsite.com",
		Name:   "New Site",
		Status: "active",
	}

	// City doesn't exist
	mockRepo.On("FindByCity", mock.Anything, "NewCity", "MG").Return(nil, nil)
	mockRepo.On("SaveCitySites", mock.Anything, mock.MatchedBy(func(cs repository.CitySites) bool {
		return cs.City == "NewCity" && cs.State == "MG" && len(cs.Sites) == 1
	})).Return(nil)

	ctx := context.Background()
	err := service.AddSiteToCity(ctx, "NewCity", "MG", newSite)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_DeleteCity(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	mockRepo.On("DeleteCity", mock.Anything, "Muzambinho", "MG").Return(nil)

	ctx := context.Background()
	err := service.DeleteCity(ctx, "Muzambinho", "MG")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_GetStatistics(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	expectedStats := &repository.CitySitesStatistics{
		TotalCities:   10,
		TotalSites:    50,
		ActiveSites:   35,
		InactiveSites: 15,
		LastDiscovery: time.Now(),
	}

	mockRepo.On("GetStatistics", mock.Anything).Return(expectedStats, nil)

	ctx := context.Background()
	stats, err := service.GetStatistics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(10), stats.TotalCities)
	assert.Equal(t, int64(50), stats.TotalSites)
	assert.Equal(t, int64(35), stats.ActiveSites)
	assert.Equal(t, int64(15), stats.InactiveSites)

	mockRepo.AssertExpectations(t)
}

func TestCitySitesService_CleanupInactiveSites(t *testing.T) {
	mockRepo := &MockCitySitesRepository{}
	service := NewCitySitesService(mockRepo)

	maxAge := 30 * 24 * time.Hour
	mockRepo.On("CleanupInactiveSites", mock.Anything, maxAge).Return(nil)

	ctx := context.Background()
	err := service.CleanupInactiveSites(ctx, maxAge)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

