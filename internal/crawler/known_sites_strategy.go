package crawler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
)

// KnownSitesStrategy usa uma lista de sites conhecidos de imobiliárias
type KnownSitesStrategy struct {
	config SiteDiscoveryConfig
	logger *logger.Logger
}

// NewKnownSitesStrategy cria nova estratégia de sites conhecidos
func NewKnownSitesStrategy(config SiteDiscoveryConfig) *KnownSitesStrategy {
	return &KnownSitesStrategy{
		config: config,
		logger: logger.NewLogger("known_sites_strategy"),
	}
}

func (k *KnownSitesStrategy) Name() string {
	return "known_sites"
}

func (k *KnownSitesStrategy) IsAvailable() bool {
	return true
}

func (k *KnownSitesStrategy) GetPriority() int {
	return 1 // Alta prioridade - mais confiável
}

func (k *KnownSitesStrategy) SearchRealEstateSites(ctx context.Context, city, state string) ([]repository.SiteInfo, error) {
	k.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Info("Starting known sites search")

	var sites []repository.SiteInfo

	// Sites conhecidos de imobiliárias - versão simplificada para garantir funcionamento
	knownSites := []KnownSite{
		{
			Name:        "Viva Real",
			BaseURL:     "https://www.vivareal.com.br",
			SearchPath:  "",
			Description: "Portal de imóveis nacional",
		},
		{
			Name:        "ZAP Imóveis",
			BaseURL:     "https://www.zapimoveis.com.br",
			SearchPath:  "",
			Description: "Portal de imóveis nacional",
		},
		{
			Name:        "OLX Imóveis",
			BaseURL:     "https://mg.olx.com.br",
			SearchPath:  "",
			Description: "Classificados online MG",
		},
		{
			Name:        "Imovelweb",
			BaseURL:     "https://www.imovelweb.com.br",
			SearchPath:  "",
			Description: "Portal de imóveis",
		},
	}

	for _, knownSite := range knownSites {
		// Usa URL base sem path específico para evitar problemas
		site := repository.SiteInfo{
			URL:             knownSite.BaseURL,
			Name:            fmt.Sprintf("%s - %s", knownSite.Name, city),
			Domain:          extractDomainFromURL(knownSite.BaseURL),
			Status:          "active", // Sites conhecidos são considerados ativos
			DiscoveredAt:    time.Now(),
			DiscoveryMethod: "known_sites",
		}

		sites = append(sites, site)

		k.logger.WithFields(map[string]interface{}{
			"site_name": site.Name,
			"site_url":  site.URL,
		}).Info("Added known site")

		// Limita número de sites
		if len(sites) >= k.config.MaxSitesPerCity {
			break
		}
	}

	k.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"sites": len(sites),
	}).Info("Known sites search completed")

	return sites, nil
}

// normalizeCityName normaliza o nome da cidade para URLs
func (k *KnownSitesStrategy) normalizeCityName(city string) string {
	// Remove acentos e caracteres especiais
	normalized := strings.ToLower(city)

	// Remove espaços e substitui por hífen
	normalized = strings.ReplaceAll(normalized, " ", "-")

	// Remove acentos comuns
	replacements := map[string]string{
		"ã": "a", "á": "a", "à": "a", "â": "a",
		"é": "e", "ê": "e",
		"í": "i", "î": "i",
		"ó": "o", "ô": "o", "õ": "o",
		"ú": "u", "û": "u",
		"ç": "c",
	}

	for old, new := range replacements {
		normalized = strings.ReplaceAll(normalized, old, new)
	}

	return normalized
}

// KnownSite representa um site conhecido de imobiliária
type KnownSite struct {
	Name        string
	BaseURL     string
	SearchPath  string
	Description string
}
