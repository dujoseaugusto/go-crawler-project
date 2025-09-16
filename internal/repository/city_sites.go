package repository

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

// CitySites representa uma cidade e seus sites de imobiliárias
type CitySites struct {
	ID            string     `bson:"_id,omitempty" json:"id"`
	City          string     `bson:"city" json:"city" binding:"required"`
	State         string     `bson:"state" json:"state" binding:"required,len=2"`
	Region        string     `bson:"region,omitempty" json:"region,omitempty"`
	Sites         []SiteInfo `bson:"sites" json:"sites"`
	LastUpdated   time.Time  `bson:"last_updated" json:"last_updated"`
	LastDiscovery time.Time  `bson:"last_discovery" json:"last_discovery"`
	Status        string     `bson:"status" json:"status"` // active, inactive, discovering, error
	TotalSites    int        `bson:"total_sites" json:"total_sites"`
	ActiveSites   int        `bson:"active_sites" json:"active_sites"`
	Hash          string     `bson:"hash" json:"hash"`
}

// SiteInfo representa informações sobre um site de imobiliária
type SiteInfo struct {
	URL             string    `bson:"url" json:"url"`
	Name            string    `bson:"name,omitempty" json:"name,omitempty"`
	Domain          string    `bson:"domain" json:"domain"`
	Status          string    `bson:"status" json:"status"` // active, inactive, error, testing, validating
	DiscoveredAt    time.Time `bson:"discovered_at" json:"discovered_at"`
	LastCrawled     time.Time `bson:"last_crawled,omitempty" json:"last_crawled,omitempty"`
	LastSuccess     time.Time `bson:"last_success,omitempty" json:"last_success,omitempty"`
	PropertiesFound int       `bson:"properties_found" json:"properties_found"`
	ErrorCount      int       `bson:"error_count" json:"error_count"`
	SuccessRate     float64   `bson:"success_rate" json:"success_rate"`
	DiscoveryMethod string    `bson:"discovery_method,omitempty" json:"discovery_method,omitempty"` // google, bing, directory, manual
	ResponseTime    float64   `bson:"response_time,omitempty" json:"response_time,omitempty"`       // em ms
	LastError       string    `bson:"last_error,omitempty" json:"last_error,omitempty"`
}

// CityInfo representa informações básicas de uma cidade para descoberta
type CityInfo struct {
	Name   string `json:"name" binding:"required"`
	State  string `json:"state" binding:"required,len=2"`
	Region string `json:"region,omitempty"`
}

// SiteStats representa estatísticas de um site para atualização
type SiteStats struct {
	PropertiesFound int       `json:"properties_found"`
	ErrorCount      int       `json:"error_count"`
	ResponseTime    float64   `json:"response_time"`
	LastSuccess     time.Time `json:"last_success"`
	LastError       string    `json:"last_error,omitempty"`
}

// CitySitesStatistics representa estatísticas gerais do sistema
type CitySitesStatistics struct {
	TotalCities         int64             `json:"total_cities"`
	TotalSites          int64             `json:"total_sites"`
	ActiveSites         int64             `json:"active_sites"`
	InactiveSites       int64             `json:"inactive_sites"`
	ErrorSites          int64             `json:"error_sites"`
	LastDiscovery       time.Time         `json:"last_discovery"`
	AverageResponseTime float64           `json:"average_response_time"`
	TopPerformingCities []CityPerformance `json:"top_performing_cities"`
}

// CityPerformance representa performance de uma cidade
type CityPerformance struct {
	City            string  `json:"city"`
	State           string  `json:"state"`
	ActiveSites     int     `json:"active_sites"`
	TotalProperties int     `json:"total_properties"`
	SuccessRate     float64 `json:"success_rate"`
}

// DiscoveryOptions define opções para descoberta de sites
type DiscoveryOptions struct {
	MaxSitesPerCity    int      `json:"max_sites_per_city,omitempty"`
	SearchEngines      []string `json:"search_engines,omitempty"`
	OverwriteExisting  bool     `json:"overwrite_existing,omitempty"`
	EnableValidation   bool     `json:"enable_validation,omitempty"`
	MinPropertiesFound int      `json:"min_properties_found,omitempty"`
	TimeoutSeconds     int      `json:"timeout_seconds,omitempty"`
}

// DiscoveryJob representa um job de descoberta em andamento
type DiscoveryJob struct {
	ID              string           `json:"id"`
	Cities          []CityInfo       `json:"cities"`
	Status          string           `json:"status"` // queued, running, completed, failed
	StartedAt       time.Time        `json:"started_at"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	Progress        int              `json:"progress"` // 0-100
	SitesDiscovered int              `json:"sites_discovered"`
	Errors          []string         `json:"errors,omitempty"`
	Options         DiscoveryOptions `json:"options"`
}

// ValidationResult representa resultado da validação de sites
type ValidationResult struct {
	City         string           `json:"city"`
	State        string           `json:"state"`
	TotalSites   int              `json:"total_sites"`
	ValidSites   int              `json:"valid_sites"`
	InvalidSites int              `json:"invalid_sites"`
	SiteResults  []SiteValidation `json:"site_results"`
	ValidatedAt  time.Time        `json:"validated_at"`
}

// SiteValidation representa resultado da validação de um site específico
type SiteValidation struct {
	URL             string    `json:"url"`
	IsValid         bool      `json:"is_valid"`
	PropertiesFound int       `json:"properties_found"`
	ResponseTime    float64   `json:"response_time"`
	Error           string    `json:"error,omitempty"`
	ValidatedAt     time.Time `json:"validated_at"`
}

// GenerateCitySitesHash gera um hash único para uma cidade
func GenerateCitySitesHash(city, state string) string {
	// Normaliza os dados para gerar hash consistente
	cityNorm := strings.TrimSpace(strings.ToLower(city))
	stateNorm := strings.TrimSpace(strings.ToUpper(state))

	data := fmt.Sprintf("city:%s|state:%s", cityNorm, stateNorm)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// GetCityKey retorna uma chave única para a cidade
func (cs *CitySites) GetCityKey() string {
	return fmt.Sprintf("%s-%s", strings.ToLower(cs.City), strings.ToUpper(cs.State))
}

// UpdateStats atualiza as estatísticas da cidade
func (cs *CitySites) UpdateStats() {
	cs.TotalSites = len(cs.Sites)
	cs.ActiveSites = 0

	for _, site := range cs.Sites {
		if site.Status == "active" {
			cs.ActiveSites++
		}
	}

	cs.LastUpdated = time.Now()
	cs.Hash = GenerateCitySitesHash(cs.City, cs.State)
}

// AddSite adiciona um site à cidade
func (cs *CitySites) AddSite(site SiteInfo) {
	// Verifica se o site já existe
	for i, existingSite := range cs.Sites {
		if existingSite.URL == site.URL {
			// Atualiza site existente
			cs.Sites[i] = site
			cs.UpdateStats()
			return
		}
	}

	// Adiciona novo site
	cs.Sites = append(cs.Sites, site)
	cs.UpdateStats()
}

// RemoveSite remove um site da cidade
func (cs *CitySites) RemoveSite(url string) bool {
	for i, site := range cs.Sites {
		if site.URL == url {
			cs.Sites = append(cs.Sites[:i], cs.Sites[i+1:]...)
			cs.UpdateStats()
			return true
		}
	}
	return false
}

// GetActiveSites retorna apenas os sites ativos
func (cs *CitySites) GetActiveSites() []SiteInfo {
	var activeSites []SiteInfo
	for _, site := range cs.Sites {
		if site.Status == "active" {
			activeSites = append(activeSites, site)
		}
	}
	return activeSites
}

// GetActiveSiteURLs retorna URLs dos sites ativos
func (cs *CitySites) GetActiveSiteURLs() []string {
	var urls []string
	for _, site := range cs.Sites {
		if site.Status == "active" {
			urls = append(urls, site.URL)
		}
	}
	return urls
}

// UpdateSiteStatus atualiza o status de um site específico
func (cs *CitySites) UpdateSiteStatus(url, status string) bool {
	for i, site := range cs.Sites {
		if site.URL == url {
			cs.Sites[i].Status = status
			cs.UpdateStats()
			return true
		}
	}
	return false
}

// UpdateSiteStats atualiza estatísticas de um site específico
func (cs *CitySites) UpdateSiteStats(url string, stats SiteStats) bool {
	for i, site := range cs.Sites {
		if site.URL == url {
			cs.Sites[i].PropertiesFound = stats.PropertiesFound
			cs.Sites[i].ErrorCount = stats.ErrorCount
			cs.Sites[i].ResponseTime = stats.ResponseTime
			cs.Sites[i].LastSuccess = stats.LastSuccess
			cs.Sites[i].LastError = stats.LastError
			cs.Sites[i].LastCrawled = time.Now()

			// Calcula success rate
			totalAttempts := cs.Sites[i].PropertiesFound + cs.Sites[i].ErrorCount
			if totalAttempts > 0 {
				cs.Sites[i].SuccessRate = float64(cs.Sites[i].PropertiesFound) / float64(totalAttempts) * 100
			}

			cs.UpdateStats()
			return true
		}
	}
	return false
}

// IsEmpty verifica se a cidade não tem sites
func (cs *CitySites) IsEmpty() bool {
	return len(cs.Sites) == 0
}

// HasActiveSites verifica se a cidade tem sites ativos
func (cs *CitySites) HasActiveSites() bool {
	return cs.ActiveSites > 0
}

// GetSiteByURL retorna um site específico por URL
func (cs *CitySites) GetSiteByURL(url string) *SiteInfo {
	for _, site := range cs.Sites {
		if site.URL == url {
			return &site
		}
	}
	return nil
}

