package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/crawler"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
)

// CitySitesService gerencia operações relacionadas a sites por cidade
type CitySitesService struct {
	repository      repository.CitySitesRepository
	discoveryEngine *crawler.SiteDiscoveryEngine
	logger          *logger.Logger
}

// NewCitySitesService cria um novo serviço de sites por cidade
func NewCitySitesService(repo repository.CitySitesRepository) *CitySitesService {
	// Configuração padrão para descoberta
	discoveryConfig := crawler.SiteDiscoveryConfig{
		MaxSitesPerCity:      20,
		MaxConcurrency:       3,
		DelayBetweenSearches: 2 * time.Second,
		Timeout:              30 * time.Second,
		UserAgent:            "Go-Crawler-Discovery/1.0",
		EnableValidation:     true,
		MinPropertiesFound:   3,
		ValidationTimeout:    15 * time.Second,
	}

	discoveryEngine := crawler.NewSiteDiscoveryEngine(repo, discoveryConfig)

	return &CitySitesService{
		repository:      repo,
		discoveryEngine: discoveryEngine,
		logger:          logger.NewLogger("city_sites_service"),
	}
}

// DiscoverSitesForCities inicia descoberta de sites para múltiplas cidades
func (s *CitySitesService) DiscoverSitesForCities(ctx context.Context, cities []repository.CityInfo, options repository.DiscoveryOptions) (*repository.DiscoveryJob, error) {
	s.logger.WithFields(map[string]interface{}{
		"cities_count": len(cities),
		"options":      options,
	}).Info("Starting site discovery for cities")

	// Valida entrada
	if len(cities) == 0 {
		return nil, fmt.Errorf("no cities provided for discovery")
	}

	if len(cities) > 50 {
		return nil, fmt.Errorf("too many cities provided (max 50)")
	}

	// Normaliza nomes das cidades
	for i := range cities {
		cities[i].Name = strings.TrimSpace(cities[i].Name)
		cities[i].State = strings.ToUpper(strings.TrimSpace(cities[i].State))

		if cities[i].Name == "" || cities[i].State == "" {
			return nil, fmt.Errorf("invalid city info at index %d", i)
		}
	}

	// Inicia descoberta
	job, err := s.discoveryEngine.DiscoverSitesForCities(ctx, cities, options)
	if err != nil {
		s.logger.WithError(err).Error("Failed to start discovery job", err)
		return nil, fmt.Errorf("failed to start discovery: %v", err)
	}

	s.logger.WithField("job_id", job.ID).Info("Discovery job started successfully")
	return job, nil
}

// GetDiscoveryJob retorna informações sobre um job de descoberta
func (s *CitySitesService) GetDiscoveryJob(jobID string) (*repository.DiscoveryJob, error) {
	job := s.discoveryEngine.GetJob(jobID)
	if job == nil {
		return nil, fmt.Errorf("discovery job not found: %s", jobID)
	}
	return job, nil
}

// GetActiveDiscoveryJobs retorna todos os jobs de descoberta ativos
func (s *CitySitesService) GetActiveDiscoveryJobs() []*repository.DiscoveryJob {
	return s.discoveryEngine.GetActiveJobs()
}

// GetAllCities retorna todas as cidades cadastradas
func (s *CitySitesService) GetAllCities(ctx context.Context) ([]repository.CitySites, error) {
	cities, err := s.repository.FindAllCities(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get all cities", err)
		return nil, fmt.Errorf("failed to get cities: %v", err)
	}

	s.logger.WithField("cities_count", len(cities)).Debug("Retrieved all cities")
	return cities, nil
}

// GetCityByName retorna uma cidade específica
func (s *CitySitesService) GetCityByName(ctx context.Context, city, state string) (*repository.CitySites, error) {
	cityData, err := s.repository.FindByCity(ctx, city, state)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get city", err)
		return nil, fmt.Errorf("failed to get city: %v", err)
	}

	if cityData == nil {
		return nil, fmt.Errorf("city not found: %s-%s", city, state)
	}

	return cityData, nil
}

// GetCitiesByNames retorna cidades específicas por nome
func (s *CitySitesService) GetCitiesByNames(ctx context.Context, cities []string) ([]repository.CitySites, error) {
	if len(cities) == 0 {
		return s.GetAllCities(ctx)
	}

	citiesData, err := s.repository.FindCitiesByNames(ctx, cities)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get cities by names", err)
		return nil, fmt.Errorf("failed to get cities: %v", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"requested_cities": len(cities),
		"found_cities":     len(citiesData),
	}).Debug("Retrieved cities by names")

	return citiesData, nil
}

// GetSitesForCrawling retorna URLs de sites para crawling
func (s *CitySitesService) GetSitesForCrawling(ctx context.Context, cities []string) ([]string, error) {
	var urls []string
	var err error

	if len(cities) == 0 {
		// Pega todos os sites ativos
		urls, err = s.repository.GetAllActiveSites(ctx)
		s.logger.WithField("source", "all_cities").Debug("Getting sites for crawling")
	} else {
		// Pega sites das cidades especificadas
		urls, err = s.repository.GetSitesByCities(ctx, cities)
		s.logger.WithFields(map[string]interface{}{
			"cities": cities,
			"source": "specific_cities",
		}).Debug("Getting sites for crawling")
	}

	if err != nil {
		s.logger.WithError(err).Error("Failed to get sites for crawling", err)
		return nil, fmt.Errorf("failed to get sites: %v", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"cities":      cities,
		"sites_count": len(urls),
	}).Info("Sites retrieved for crawling")

	return urls, nil
}

// ValidateCitySites valida sites de uma cidade específica
func (s *CitySitesService) ValidateCitySites(ctx context.Context, city, state string) (*repository.ValidationResult, error) {
	s.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Info("Starting city sites validation")

	// Busca a cidade
	cityData, err := s.repository.FindByCity(ctx, city, state)
	if err != nil {
		return nil, fmt.Errorf("failed to find city: %v", err)
	}

	if cityData == nil {
		return nil, fmt.Errorf("city not found: %s-%s", city, state)
	}

	result := &repository.ValidationResult{
		City:        city,
		State:       state,
		TotalSites:  len(cityData.Sites),
		ValidatedAt: time.Now(),
	}

	// Valida cada site
	for _, site := range cityData.Sites {
		siteValidation := s.validateSite(ctx, site)
		result.SiteResults = append(result.SiteResults, siteValidation)

		if siteValidation.IsValid {
			result.ValidSites++
		} else {
			result.InvalidSites++
		}

		// Atualiza status do site no repositório
		newStatus := "active"
		if !siteValidation.IsValid {
			newStatus = "inactive"
		}

		err := s.repository.UpdateSiteStatus(ctx, city, state, site.URL, newStatus)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to update site status")
		}
	}

	s.logger.WithFields(map[string]interface{}{
		"city":          city,
		"state":         state,
		"total_sites":   result.TotalSites,
		"valid_sites":   result.ValidSites,
		"invalid_sites": result.InvalidSites,
	}).Info("City sites validation completed")

	return result, nil
}

// validateSite valida um site específico
func (s *CitySitesService) validateSite(ctx context.Context, site repository.SiteInfo) repository.SiteValidation {
	startTime := time.Now()

	validation := repository.SiteValidation{
		URL:         site.URL,
		ValidatedAt: time.Now(),
	}

	// Aqui você pode implementar validação mais sofisticada
	// Por enquanto, considera válido se o site tem histórico de sucesso
	if site.PropertiesFound > 0 && site.SuccessRate > 50 {
		validation.IsValid = true
		validation.PropertiesFound = site.PropertiesFound
	} else {
		validation.IsValid = false
		validation.Error = "Site has low success rate or no properties found"
	}

	validation.ResponseTime = float64(time.Since(startTime).Milliseconds())
	return validation
}

// UpdateSiteStats atualiza estatísticas de um site
func (s *CitySitesService) UpdateSiteStats(ctx context.Context, city, state, url string, stats repository.SiteStats) error {
	err := s.repository.UpdateSiteStats(ctx, city, state, url, stats)
	if err != nil {
		s.logger.WithError(err).Error("Failed to update site stats", err)
		return fmt.Errorf("failed to update site stats: %v", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"city":             city,
		"state":            state,
		"url":              url,
		"properties_found": stats.PropertiesFound,
	}).Debug("Site stats updated")

	return nil
}

// DeleteCity remove uma cidade e todos os seus sites
func (s *CitySitesService) DeleteCity(ctx context.Context, city, state string) error {
	s.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Warn("Deleting city and all its sites")

	err := s.repository.DeleteCity(ctx, city, state)
	if err != nil {
		s.logger.WithError(err).Error("Failed to delete city", err)
		return fmt.Errorf("failed to delete city: %v", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Info("City deleted successfully")

	return nil
}

// GetStatistics retorna estatísticas gerais do sistema
func (s *CitySitesService) GetStatistics(ctx context.Context) (*repository.CitySitesStatistics, error) {
	stats, err := s.repository.GetStatistics(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get statistics", err)
		return nil, fmt.Errorf("failed to get statistics: %v", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"total_cities": stats.TotalCities,
		"total_sites":  stats.TotalSites,
		"active_sites": stats.ActiveSites,
	}).Debug("Statistics retrieved")

	return stats, nil
}

// CleanupInactiveSites remove sites inativos antigos
func (s *CitySitesService) CleanupInactiveSites(ctx context.Context, maxAge time.Duration) error {
	s.logger.WithField("max_age", maxAge).Info("Starting cleanup of inactive sites")

	err := s.repository.CleanupInactiveSites(ctx, maxAge)
	if err != nil {
		s.logger.WithError(err).Error("Failed to cleanup inactive sites", err)
		return fmt.Errorf("failed to cleanup sites: %v", err)
	}

	s.logger.Info("Inactive sites cleanup completed")
	return nil
}

// AddSiteToCity adiciona um site manualmente a uma cidade
func (s *CitySitesService) AddSiteToCity(ctx context.Context, city, state string, site repository.SiteInfo) error {
	s.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"url":   site.URL,
	}).Info("Adding site to city manually")

	// Busca a cidade
	cityData, err := s.repository.FindByCity(ctx, city, state)
	if err != nil {
		return fmt.Errorf("failed to find city: %v", err)
	}

	// Se a cidade não existe, cria uma nova
	if cityData == nil {
		cityData = &repository.CitySites{
			City:          city,
			State:         strings.ToUpper(state),
			Status:        "active",
			LastDiscovery: time.Now(),
		}
	}

	// Adiciona o site
	site.DiscoveredAt = time.Now()
	site.DiscoveryMethod = "manual"
	site.Status = "active"

	cityData.AddSite(site)

	// Salva no repositório
	err = s.repository.SaveCitySites(ctx, *cityData)
	if err != nil {
		s.logger.WithError(err).Error("Failed to save city with new site", err)
		return fmt.Errorf("failed to save city: %v", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"url":   site.URL,
	}).Info("Site added to city successfully")

	return nil
}

// RemoveSiteFromCity remove um site de uma cidade
func (s *CitySitesService) RemoveSiteFromCity(ctx context.Context, city, state, url string) error {
	s.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"url":   url,
	}).Info("Removing site from city")

	// Busca a cidade
	cityData, err := s.repository.FindByCity(ctx, city, state)
	if err != nil {
		return fmt.Errorf("failed to find city: %v", err)
	}

	if cityData == nil {
		return fmt.Errorf("city not found: %s-%s", city, state)
	}

	// Remove o site
	if !cityData.RemoveSite(url) {
		return fmt.Errorf("site not found in city: %s", url)
	}

	// Salva no repositório
	err = s.repository.SaveCitySites(ctx, *cityData)
	if err != nil {
		s.logger.WithError(err).Error("Failed to save city after removing site", err)
		return fmt.Errorf("failed to save city: %v", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"url":   url,
	}).Info("Site removed from city successfully")

	return nil
}

// GetCitiesByRegion retorna cidades de uma região específica
func (s *CitySitesService) GetCitiesByRegion(ctx context.Context, region string) ([]repository.CitySites, error) {
	cities, err := s.repository.FindCitiesByRegion(ctx, region)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get cities by region", err)
		return nil, fmt.Errorf("failed to get cities by region: %v", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"region":       region,
		"cities_count": len(cities),
	}).Debug("Retrieved cities by region")

	return cities, nil
}
