package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

// SiteDiscoveryEngine é responsável por descobrir sites de imobiliárias
type SiteDiscoveryEngine struct {
	repository repository.CitySitesRepository
	strategies []SearchStrategy
	logger     *logger.Logger
	config     SiteDiscoveryConfig
	jobManager *DiscoveryJobManager
}

// SiteDiscoveryConfig configurações para descoberta de sites
type SiteDiscoveryConfig struct {
	MaxSitesPerCity      int           `json:"max_sites_per_city"`
	MaxConcurrency       int           `json:"max_concurrency"`
	DelayBetweenSearches time.Duration `json:"delay_between_searches"`
	Timeout              time.Duration `json:"timeout"`
	UserAgent            string        `json:"user_agent"`
	EnableValidation     bool          `json:"enable_validation"`
	MinPropertiesFound   int           `json:"min_properties_found"`
	ValidationTimeout    time.Duration `json:"validation_timeout"`
}

// SearchStrategy interface para estratégias de busca
type SearchStrategy interface {
	Name() string
	SearchRealEstateSites(ctx context.Context, city, state string) ([]repository.SiteInfo, error)
	IsAvailable() bool
	GetPriority() int // 1 = alta, 2 = média, 3 = baixa
}

// DiscoveryJobManager gerencia jobs de descoberta
type DiscoveryJobManager struct {
	jobs   map[string]*repository.DiscoveryJob
	mutex  sync.RWMutex
	logger *logger.Logger
}

// NewSiteDiscoveryEngine cria um novo engine de descoberta
func NewSiteDiscoveryEngine(repo repository.CitySitesRepository, config SiteDiscoveryConfig) *SiteDiscoveryEngine {
	// Configurações padrão
	if config.MaxSitesPerCity == 0 {
		config.MaxSitesPerCity = 20
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 3
	}
	if config.DelayBetweenSearches == 0 {
		config.DelayBetweenSearches = 2 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "Go-Site-Discovery/1.0"
	}
	if config.ValidationTimeout == 0 {
		config.ValidationTimeout = 15 * time.Second
	}

	engine := &SiteDiscoveryEngine{
		repository: repo,
		logger:     logger.NewLogger("site_discovery"),
		config:     config,
		jobManager: &DiscoveryJobManager{
			jobs:   make(map[string]*repository.DiscoveryJob),
			logger: logger.NewLogger("discovery_job_manager"),
		},
	}

	// Inicializa estratégias de busca
	engine.initializeStrategies()

	return engine
}

// initializeStrategies inicializa as estratégias de busca disponíveis
func (e *SiteDiscoveryEngine) initializeStrategies() {
	e.strategies = []SearchStrategy{
		NewWebSearchStrategy(e.config),       // Prioridade 1 - busca web real
		NewKnownSitesStrategy(e.config),      // Prioridade 2 - sites conhecidos
		NewDomainGuessingStrategy(e.config),  // Prioridade 3 - backup
		// NewDirectorySearchStrategy(e.config), // Desabilitado - estava dando Forbidden
		// NewGoogleSearchStrategy(e.config), // Desabilitado - bloqueado pelo Google
	}

	// Log das estratégias disponíveis
	var availableStrategies []string
	for _, strategy := range e.strategies {
		if strategy.IsAvailable() {
			availableStrategies = append(availableStrategies, strategy.Name())
		}
	}

	e.logger.WithField("strategies", availableStrategies).Info("Discovery strategies initialized")
}

// DiscoverSitesForCities inicia descoberta para múltiplas cidades
func (e *SiteDiscoveryEngine) DiscoverSitesForCities(ctx context.Context, cities []repository.CityInfo, options repository.DiscoveryOptions) (*repository.DiscoveryJob, error) {
	// Cria job de descoberta
	job := &repository.DiscoveryJob{
		ID:        fmt.Sprintf("discovery-%d", time.Now().Unix()),
		Cities:    cities,
		Status:    "queued",
		StartedAt: time.Now(),
		Progress:  0,
		Options:   options,
	}

	// Registra o job
	e.jobManager.AddJob(job)

	// Inicia processamento assíncrono
	go e.processDiscoveryJob(ctx, job)

	return job, nil
}

// processDiscoveryJob processa um job de descoberta
func (e *SiteDiscoveryEngine) processDiscoveryJob(parentCtx context.Context, job *repository.DiscoveryJob) {
	// Cria contexto independente para evitar cancelamento prematuro
	ctx := context.Background()
	e.jobManager.UpdateJobStatus(job.ID, "running")
	e.logger.WithField("job_id", job.ID).Info("Starting discovery job")

	totalCities := len(job.Cities)
	processedCities := 0

	// Canal para controlar concorrência
	semaphore := make(chan struct{}, e.config.MaxConcurrency)

	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, city := range job.Cities {
		wg.Add(1)
		go func(cityInfo repository.CityInfo) {
			defer wg.Done()

			// Controla concorrência
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Processa cidade
			sites, err := e.DiscoverSitesForCity(ctx, cityInfo.Name, cityInfo.State)
			if err != nil {
				mutex.Lock()
				job.Errors = append(job.Errors, fmt.Sprintf("%s-%s: %v", cityInfo.Name, cityInfo.State, err))
				mutex.Unlock()
				e.logger.WithFields(map[string]interface{}{
					"city":  cityInfo.Name,
					"state": cityInfo.State,
					"error": err,
				}).Error("Failed to discover sites for city", err)
			} else {
				mutex.Lock()
				job.SitesDiscovered += len(sites)
				mutex.Unlock()
				e.logger.WithFields(map[string]interface{}{
					"city":  cityInfo.Name,
					"state": cityInfo.State,
					"sites": len(sites),
				}).Info("Sites discovered for city")
			}

			// Atualiza progresso
			mutex.Lock()
			processedCities++
			job.Progress = int(float64(processedCities) / float64(totalCities) * 100)
			mutex.Unlock()

			// Delay entre processamentos
			time.Sleep(e.config.DelayBetweenSearches)
		}(city)
	}

	// Aguarda conclusão
	wg.Wait()

	// Finaliza job
	now := time.Now()
	job.CompletedAt = &now
	job.Progress = 100

	if len(job.Errors) > 0 {
		e.jobManager.UpdateJobStatus(job.ID, "completed_with_errors")
	} else {
		e.jobManager.UpdateJobStatus(job.ID, "completed")
	}

	e.logger.WithFields(map[string]interface{}{
		"job_id":           job.ID,
		"sites_discovered": job.SitesDiscovered,
		"errors":           len(job.Errors),
		"duration":         time.Since(job.StartedAt),
	}).Info("Discovery job completed")
}

// DiscoverSitesForCity descobre sites para uma cidade específica
func (e *SiteDiscoveryEngine) DiscoverSitesForCity(ctx context.Context, city, state string) ([]repository.SiteInfo, error) {
	e.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Info("Starting site discovery for city")

	var allSites []repository.SiteInfo
	var discoveredURLs = make(map[string]bool) // Para evitar duplicatas

	// Executa cada estratégia de busca
	for _, strategy := range e.strategies {
		if !strategy.IsAvailable() {
			continue
		}

		e.logger.WithFields(map[string]interface{}{
			"city":     city,
			"state":    state,
			"strategy": strategy.Name(),
		}).Debug("Executing search strategy")

		sites, err := strategy.SearchRealEstateSites(ctx, city, state)
		if err != nil {
			e.logger.WithFields(map[string]interface{}{
				"city":     city,
				"state":    state,
				"strategy": strategy.Name(),
				"error":    err,
			}).Warn("Search strategy failed")
			continue
		}

		// Adiciona sites únicos
		for _, site := range sites {
			if !discoveredURLs[site.URL] {
				discoveredURLs[site.URL] = true
				site.DiscoveredAt = time.Now()
				site.DiscoveryMethod = strategy.Name()
				allSites = append(allSites, site)

				// Limita número de sites por cidade
				if len(allSites) >= e.config.MaxSitesPerCity {
					break
				}
			}
		}

		if len(allSites) >= e.config.MaxSitesPerCity {
			break
		}

		// Delay entre estratégias
		time.Sleep(e.config.DelayBetweenSearches)
	}

	// Valida sites se habilitado (sempre validar para garantir qualidade)
	if len(allSites) > 0 {
		e.logger.WithFields(map[string]interface{}{
			"city":        city,
			"state":       state,
			"sites_found": len(allSites),
		}).Info("Starting site validation")
		
		allSites = e.validateSites(ctx, allSites)
		
		e.logger.WithFields(map[string]interface{}{
			"city":            city,
			"state":           state,
			"validated_sites": len(allSites),
		}).Info("Site validation completed")
	}

	// Salva resultados no repositório
	if len(allSites) > 0 {
		citySites := repository.CitySites{
			City:          city,
			State:         strings.ToUpper(state),
			Sites:         allSites,
			LastDiscovery: time.Now(),
			Status:        "active",
		}

		if err := e.repository.SaveCitySites(ctx, citySites); err != nil {
			e.logger.WithError(err).Error("Failed to save discovered sites", err)
			return nil, fmt.Errorf("failed to save sites: %v", err)
		}
	}

	e.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"sites": len(allSites),
	}).Info("Site discovery completed")

	return allSites, nil
}

// validateSites valida se os sites encontrados realmente têm imóveis
func (e *SiteDiscoveryEngine) validateSites(ctx context.Context, sites []repository.SiteInfo) []repository.SiteInfo {
	e.logger.WithField("sites_count", len(sites)).Info("Starting site validation")

	var validSites []repository.SiteInfo
	semaphore := make(chan struct{}, e.config.MaxConcurrency)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, site := range sites {
		wg.Add(1)
		go func(s repository.SiteInfo) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if e.validateSite(ctx, s.URL) {
				s.Status = "active"
				mutex.Lock()
				validSites = append(validSites, s)
				mutex.Unlock()
			} else {
				s.Status = "inactive"
			}
		}(site)
	}

	wg.Wait()

	e.logger.WithFields(map[string]interface{}{
		"total_sites": len(sites),
		"valid_sites": len(validSites),
	}).Info("Site validation completed")

	return validSites
}

// validateSite valida um site específico
func (e *SiteDiscoveryEngine) validateSite(ctx context.Context, siteURL string) bool {
	// Cria contexto com timeout
	ctx, cancel := context.WithTimeout(ctx, e.config.ValidationTimeout)
	defer cancel()

	// Configura coletor para validação
	c := colly.NewCollector(
		colly.UserAgent(e.config.UserAgent),
	)

	extensions.RandomUserAgent(c)
	c.SetRequestTimeout(e.config.ValidationTimeout)

	// Procura por indicadores de imóveis
	foundProperties := false
	propertyIndicators := []string{
		"imovel", "imóvel", "propriedade", "casa", "apartamento",
		"venda", "aluguel", "real estate", "property", "house",
		"r$", "reais", "preço", "valor",
	}

	c.OnHTML("body", func(e *colly.HTMLElement) {
		text := strings.ToLower(e.Text)

		indicatorCount := 0
		for _, indicator := range propertyIndicators {
			if strings.Contains(text, indicator) {
				indicatorCount++
			}
		}

		// Considera válido se encontrar pelo menos 3 indicadores
		if indicatorCount >= 3 {
			foundProperties = true
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		// Site com erro não é considerado válido
		foundProperties = false
	})

	// Visita o site
	err := c.Visit(siteURL)
	if err != nil {
		return false
	}

	c.Wait()
	return foundProperties
}

// GetJob retorna informações sobre um job de descoberta
func (e *SiteDiscoveryEngine) GetJob(jobID string) *repository.DiscoveryJob {
	return e.jobManager.GetJob(jobID)
}

// GetActiveJobs retorna todos os jobs ativos
func (e *SiteDiscoveryEngine) GetActiveJobs() []*repository.DiscoveryJob {
	return e.jobManager.GetActiveJobs()
}

// DiscoveryJobManager methods

// AddJob adiciona um novo job
func (m *DiscoveryJobManager) AddJob(job *repository.DiscoveryJob) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.jobs[job.ID] = job
}

// GetJob retorna um job específico
func (m *DiscoveryJobManager) GetJob(jobID string) *repository.DiscoveryJob {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.jobs[jobID]
}

// UpdateJobStatus atualiza o status de um job
func (m *DiscoveryJobManager) UpdateJobStatus(jobID, status string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if job, exists := m.jobs[jobID]; exists {
		job.Status = status
	}
}

// GetActiveJobs retorna jobs ativos
func (m *DiscoveryJobManager) GetActiveJobs() []*repository.DiscoveryJob {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var activeJobs []*repository.DiscoveryJob
	for _, job := range m.jobs {
		if job.Status == "queued" || job.Status == "running" {
			activeJobs = append(activeJobs, job)
		}
	}
	return activeJobs
}

// CleanupOldJobs remove jobs antigos
func (m *DiscoveryJobManager) CleanupOldJobs(maxAge time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for jobID, job := range m.jobs {
		if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
			delete(m.jobs, jobID)
		}
	}
}

// Utility functions

// extractDomainFromURL extrai domínio de uma URL
func extractDomainFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

// isValidRealEstateURL verifica se uma URL parece ser de imobiliária
func isValidRealEstateURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}

	// Verifica se é uma URL válida
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}

	// Palavras-chave que indicam sites de imobiliária
	keywords := []string{
		"imovel", "imobiliaria", "corretor", "real", "estate",
		"propriedade", "casa", "apartamento", "venda", "aluguel",
	}

	urlLower := strings.ToLower(rawURL)
	for _, keyword := range keywords {
		if strings.Contains(urlLower, keyword) {
			return true
		}
	}

	return false
}

// normalizeURL normaliza uma URL para comparação
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Remove www
	host := strings.TrimPrefix(u.Host, "www.")

	// Reconstrói URL normalizada
	u.Host = host
	u.Fragment = ""

	// Remove parâmetros de query desnecessários
	query := u.Query()
	for key := range query {
		if strings.Contains(strings.ToLower(key), "utm") ||
			strings.Contains(strings.ToLower(key), "ref") ||
			strings.Contains(strings.ToLower(key), "source") {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()

	return u.String()
}
