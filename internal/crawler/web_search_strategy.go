package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

// WebSearchStrategy implementa busca web real usando DuckDuckGo (mais permissivo que Google)
type WebSearchStrategy struct {
	config SiteDiscoveryConfig
	logger *logger.Logger
}

// NewWebSearchStrategy cria nova estratégia de busca web
func NewWebSearchStrategy(config SiteDiscoveryConfig) *WebSearchStrategy {
	return &WebSearchStrategy{
		config: config,
		logger: logger.NewLogger("web_search_strategy"),
	}
}

func (w *WebSearchStrategy) Name() string {
	return "web_search"
}

func (w *WebSearchStrategy) IsAvailable() bool {
	return true
}

func (w *WebSearchStrategy) GetPriority() int {
	return 1 // Alta prioridade - busca real na web
}

func (w *WebSearchStrategy) SearchRealEstateSites(ctx context.Context, city, state string) ([]repository.SiteInfo, error) {
	w.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Info("Starting web search for real estate sites")

	var allSites []repository.SiteInfo
	discoveredURLs := make(map[string]bool)

	// Queries de busca específicas para imobiliárias
	queries := []string{
		fmt.Sprintf("imobiliária %s %s", city, state),
		fmt.Sprintf("imóveis %s %s", city, state),
		fmt.Sprintf("corretor %s %s", city, state),
		fmt.Sprintf("casas apartamentos %s %s", city, state),
	}

	// Usar DuckDuckGo que é mais permissivo
	for _, query := range queries {
		sites, err := w.searchDuckDuckGo(ctx, query, city, state)
		if err != nil {
			w.logger.WithFields(map[string]interface{}{
				"query": query,
				"error": err,
			}).Warn("DuckDuckGo search failed")
			continue
		}

		// Adiciona sites únicos
		for _, site := range sites {
			if !discoveredURLs[site.URL] {
				discoveredURLs[site.URL] = true
				allSites = append(allSites, site)

				// Limita número de sites
				if len(allSites) >= w.config.MaxSitesPerCity {
					break
				}
			}
		}

		if len(allSites) >= w.config.MaxSitesPerCity {
			break
		}

		// Delay entre queries para evitar bloqueio
		time.Sleep(2 * time.Second)
	}

	w.logger.WithFields(map[string]interface{}{
		"city":        city,
		"state":       state,
		"sites_found": len(allSites),
	}).Info("Web search completed")

	return allSites, nil
}

// searchDuckDuckGo faz busca no DuckDuckGo
func (w *WebSearchStrategy) searchDuckDuckGo(ctx context.Context, query, city, state string) ([]repository.SiteInfo, error) {
	var sites []repository.SiteInfo

	// Configura coletor
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)
	extensions.RandomUserAgent(c)
	c.SetRequestTimeout(30 * time.Second)

	// Limita requisições
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*duckduckgo.com*",
		Parallelism: 1,
		Delay:       3 * time.Second,
	})

	// Extrai links dos resultados de busca do DuckDuckGo
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		title := strings.TrimSpace(e.Text)

		// Filtra links relevantes
		if w.isRelevantRealEstateLink(link, title, city, state) {
			// Limpa URL se necessário
			cleanURL := w.cleanSearchURL(link)
			if cleanURL != "" && w.isValidRealEstateURL(cleanURL) {
				site := repository.SiteInfo{
					URL:             cleanURL,
					Name:            w.extractSiteName(title, city),
					Domain:          extractDomainFromURL(cleanURL),
					Status:          "testing", // Será validado depois
					DiscoveredAt:    time.Now(),
					DiscoveryMethod: "web_search",
				}
				sites = append(sites, site)

				w.logger.WithFields(map[string]interface{}{
					"url":   cleanURL,
					"title": title,
				}).Info("Found potential real estate site")
			}
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		w.logger.WithFields(map[string]interface{}{
			"url":    r.Request.URL.String(),
			"status": r.StatusCode,
			"error":  err,
		}).Warn("Error during web search")
	})

	// Monta URL de busca do DuckDuckGo
	searchURL := fmt.Sprintf("https://duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	
	w.logger.WithFields(map[string]interface{}{
		"query":      query,
		"search_url": searchURL,
	}).Debug("Searching DuckDuckGo")

	if err := c.Visit(searchURL); err != nil {
		return nil, fmt.Errorf("failed to search DuckDuckGo: %v", err)
	}
	c.Wait()

	return sites, nil
}

// isRelevantRealEstateLink verifica se um link é relevante para imóveis
func (w *WebSearchStrategy) isRelevantRealEstateLink(link, title, city, state string) bool {
	if link == "" || title == "" {
		return false
	}

	// Ignora links internos do DuckDuckGo
	if strings.Contains(link, "duckduckgo.com") {
		return false
	}

	// Ignora links de redes sociais e outros irrelevantes
	irrelevantDomains := []string{
		"facebook.com", "instagram.com", "twitter.com", "youtube.com",
		"google.com", "maps.google.com", "wikipedia.org",
	}
	for _, domain := range irrelevantDomains {
		if strings.Contains(link, domain) {
			return false
		}
	}

	// Verifica se o título contém palavras-chave relevantes
	titleLower := strings.ToLower(title)
	keywords := []string{
		"imobiliária", "imobiliaria", "imoveis", "imóveis",
		"corretor", "casas", "apartamentos", "venda", "aluguel",
		"propriedades", "terrenos", "lotes",
	}

	for _, keyword := range keywords {
		if strings.Contains(titleLower, keyword) {
			return true
		}
	}

	return false
}

// cleanSearchURL limpa URLs de redirecionamento do DuckDuckGo
func (w *WebSearchStrategy) cleanSearchURL(rawURL string) string {
	// DuckDuckGo usa URLs diretas, mas pode ter parâmetros
	if strings.HasPrefix(rawURL, "http") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return ""
		}
		// Remove parâmetros de tracking
		u.RawQuery = ""
		u.Fragment = ""
		return u.String()
	}
	return ""
}

// isValidRealEstateURL verifica se uma URL é válida para imóveis
func (w *WebSearchStrategy) isValidRealEstateURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Deve ser HTTP/HTTPS
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	// Deve ter domínio válido
	if u.Host == "" {
		return false
	}

	// Preferência por domínios .com.br
	if strings.HasSuffix(u.Host, ".com.br") {
		return true
	}

	// Aceita outros domínios se contém palavras-chave
	hostLower := strings.ToLower(u.Host)
	keywords := []string{
		"imoveis", "imobiliaria", "corretor", "casas", "propriedades",
	}
	for _, keyword := range keywords {
		if strings.Contains(hostLower, keyword) {
			return true
		}
	}

	return false
}

// extractSiteName extrai nome do site a partir do título
func (w *WebSearchStrategy) extractSiteName(title, city string) string {
	if title == "" {
		return fmt.Sprintf("Site de Imóveis - %s", city)
	}

	// Limita tamanho do nome
	if len(title) > 100 {
		title = title[:100] + "..."
	}

	return title
}

// validateRealEstateSite valida se um site realmente tem conteúdo de imóveis
func (w *WebSearchStrategy) validateRealEstateSite(ctx context.Context, siteURL string) (bool, int) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	var propertiesFound int
	var hasRealEstateContent bool

	c.OnHTML("body", func(e *colly.HTMLElement) {
		text := strings.ToLower(e.Text)
		
		// Verifica indicadores de conteúdo imobiliário
		indicators := []string{
			"imóveis", "imoveis", "casas", "apartamentos", "venda", "aluguel",
			"quartos", "banheiros", "m²", "r$", "reais", "propriedades",
		}

		indicatorCount := 0
		for _, indicator := range indicators {
			if strings.Contains(text, indicator) {
				indicatorCount++
			}
		}

		if indicatorCount >= 3 {
			hasRealEstateContent = true
		}

		// Conta elementos que parecem listagens de propriedades
		e.ForEach(".property, .imovel, .listing, .card, .item", func(_ int, el *colly.HTMLElement) {
			elText := strings.ToLower(el.Text)
			if strings.Contains(elText, "r$") || strings.Contains(elText, "quartos") {
				propertiesFound++
			}
		})
	})

	c.OnError(func(r *colly.Response, err error) {
		w.logger.WithFields(map[string]interface{}{
			"url":   siteURL,
			"error": err,
		}).Debug("Error validating site")
	})

	if err := c.Visit(siteURL); err != nil {
		return false, 0
	}
	c.Wait()

	return hasRealEstateContent && propertiesFound > 0, propertiesFound
}
