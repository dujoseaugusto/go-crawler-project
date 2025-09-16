package crawler

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

// GoogleSearchStrategy busca sites usando Google Search
type GoogleSearchStrategy struct {
	config SiteDiscoveryConfig
	logger *logger.Logger
}

// DirectorySearchStrategy busca em diretórios conhecidos
type DirectorySearchStrategy struct {
	config SiteDiscoveryConfig
	logger *logger.Logger
}

// DomainGuessingStrategy tenta adivinhar domínios baseado em padrões
type DomainGuessingStrategy struct {
	config SiteDiscoveryConfig
	logger *logger.Logger
}

// NewGoogleSearchStrategy cria nova estratégia de busca Google
func NewGoogleSearchStrategy(config SiteDiscoveryConfig) *GoogleSearchStrategy {
	return &GoogleSearchStrategy{
		config: config,
		logger: logger.NewLogger("google_search_strategy"),
	}
}

// NewDirectorySearchStrategy cria nova estratégia de diretório
func NewDirectorySearchStrategy(config SiteDiscoveryConfig) *DirectorySearchStrategy {
	return &DirectorySearchStrategy{
		config: config,
		logger: logger.NewLogger("directory_search_strategy"),
	}
}

// NewDomainGuessingStrategy cria nova estratégia de adivinhação de domínio
func NewDomainGuessingStrategy(config SiteDiscoveryConfig) *DomainGuessingStrategy {
	return &DomainGuessingStrategy{
		config: config,
		logger: logger.NewLogger("domain_guessing_strategy"),
	}
}

// GoogleSearchStrategy implementation

func (g *GoogleSearchStrategy) Name() string {
	return "google_search"
}

func (g *GoogleSearchStrategy) IsAvailable() bool {
	// Google Search sempre disponível (com limitações de rate)
	return true
}

func (g *GoogleSearchStrategy) GetPriority() int {
	return 1 // Alta prioridade
}

func (g *GoogleSearchStrategy) SearchRealEstateSites(ctx context.Context, city, state string) ([]repository.SiteInfo, error) {
	g.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Debug("Starting Google search")

	var sites []repository.SiteInfo

	// Queries de busca específicas para imobiliárias
	queries := []string{
		fmt.Sprintf("imobiliária %s %s site:*.com.br", city, state),
		fmt.Sprintf("corretor imóveis %s %s", city, state),
		fmt.Sprintf("casas apartamentos venda %s %s", city, state),
		fmt.Sprintf("\"%s %s\" imóveis venda", city, state),
	}

	// Configura coletor
	c := colly.NewCollector(
		colly.UserAgent(g.config.UserAgent),
	)

	extensions.RandomUserAgent(c)
	c.SetRequestTimeout(g.config.Timeout)

	// Limita requisições para evitar bloqueio
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*google.*",
		Parallelism: 1,
		Delay:       3 * time.Second,
	})

	// Extrai links dos resultados de busca
	c.OnHTML("div.g", func(e *colly.HTMLElement) {
		// Extrai URL do resultado
		link := e.ChildAttr("a[href]", "href")
		if link == "" {
			return
		}

		// Limpa URL do Google
		if strings.Contains(link, "/url?q=") {
			u, err := url.Parse(link)
			if err != nil {
				return
			}
			link = u.Query().Get("q")
		}

		// Verifica se é uma URL válida de imobiliária
		if !isValidRealEstateURL(link) {
			return
		}

		// Extrai título
		title := strings.TrimSpace(e.ChildText("h3"))

		site := repository.SiteInfo{
			URL:          normalizeURL(link),
			Name:         title,
			Domain:       extractDomainFromURL(link),
			Status:       "discovered",
			DiscoveredAt: time.Now(),
		}

		sites = append(sites, site)
	})

	// Executa buscas
	for i, query := range queries {
		if i > 0 {
			time.Sleep(g.config.DelayBetweenSearches)
		}

		searchURL := fmt.Sprintf("https://www.google.com/search?q=%s&num=10", url.QueryEscape(query))

		g.logger.WithField("query", query).Debug("Executing Google search")

		err := c.Visit(searchURL)
		if err != nil {
			g.logger.WithError(err).Warn("Google search failed")
			continue
		}

		c.Wait()

		// Limita resultados
		if len(sites) >= g.config.MaxSitesPerCity {
			break
		}
	}

	g.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"sites": len(sites),
	}).Debug("Google search completed")

	return sites, nil
}

// DirectorySearchStrategy implementation

func (d *DirectorySearchStrategy) Name() string {
	return "directory_search"
}

func (d *DirectorySearchStrategy) IsAvailable() bool {
	return true
}

func (d *DirectorySearchStrategy) GetPriority() int {
	return 2 // Média prioridade
}

func (d *DirectorySearchStrategy) SearchRealEstateSites(ctx context.Context, city, state string) ([]repository.SiteInfo, error) {
	d.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Debug("Starting directory search")

	var sites []repository.SiteInfo

	// Diretórios conhecidos de imobiliárias
	directories := []DirectorySource{
		{
			Name:       "Viva Real",
			URL:        "https://www.vivareal.com.br",
			SearchPath: "/venda/minas-gerais/%s/",
		},
		{
			Name:       "ZAP Imóveis",
			URL:        "https://www.zapimoveis.com.br",
			SearchPath: "/venda/imoveis/mg+%s/",
		},
		{
			Name:       "OLX",
			URL:        "https://mg.olx.com.br",
			SearchPath: "/imoveis/venda/estado-mg/regiao-%s",
		},
	}

	// Configura coletor
	c := colly.NewCollector(
		colly.UserAgent(d.config.UserAgent),
	)

	extensions.RandomUserAgent(c)
	c.SetRequestTimeout(d.config.Timeout)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	// Procura por links de imobiliárias nos diretórios
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		if href == "" {
			return
		}

		// Converte para URL absoluta
		absoluteURL := e.Request.AbsoluteURL(href)

		// Verifica se é link externo (não do próprio diretório)
		if !d.isExternalRealEstateLink(absoluteURL, e.Request.URL.Host) {
			return
		}

		if isValidRealEstateURL(absoluteURL) {
			site := repository.SiteInfo{
				URL:          normalizeURL(absoluteURL),
				Name:         strings.TrimSpace(e.Text),
				Domain:       extractDomainFromURL(absoluteURL),
				Status:       "discovered",
				DiscoveredAt: time.Now(),
			}

			sites = append(sites, site)
		}
	})

	// Busca em cada diretório
	for _, dir := range directories {
		searchURL := dir.URL + fmt.Sprintf(dir.SearchPath, strings.ToLower(city))

		d.logger.WithFields(map[string]interface{}{
			"directory": dir.Name,
			"url":       searchURL,
		}).Debug("Searching directory")

		err := c.Visit(searchURL)
		if err != nil {
			d.logger.WithError(err).Warn("Directory search failed")
			continue
		}

		c.Wait()
		time.Sleep(d.config.DelayBetweenSearches)

		if len(sites) >= d.config.MaxSitesPerCity {
			break
		}
	}

	d.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"sites": len(sites),
	}).Debug("Directory search completed")

	return sites, nil
}

func (d *DirectorySearchStrategy) isExternalRealEstateLink(link, currentHost string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}

	// Deve ser domínio diferente do diretório atual
	if u.Host == currentHost || strings.Contains(u.Host, currentHost) {
		return false
	}

	// Deve conter palavras-chave de imobiliária
	return isValidRealEstateURL(link)
}

// DomainGuessingStrategy implementation

func (g *DomainGuessingStrategy) Name() string {
	return "domain_guessing"
}

func (g *DomainGuessingStrategy) IsAvailable() bool {
	return true
}

func (g *DomainGuessingStrategy) GetPriority() int {
	return 3 // Baixa prioridade
}

func (g *DomainGuessingStrategy) SearchRealEstateSites(ctx context.Context, city, state string) ([]repository.SiteInfo, error) {
	g.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
	}).Debug("Starting domain guessing")

	var sites []repository.SiteInfo

	// Padrões comuns de domínios de imobiliárias (expandidos)
	patterns := []string{
		"imobiliaria%s.com.br",
		"%simobiliaria.com.br",
		"imoveis%s.com.br",
		"%simoveis.com.br",
		"corretor%s.com.br",
		"%scorretor.com.br",
		"casas%s.com.br",
		"%scasas.com.br",
		"imob%s.com.br",
		"%simob.com.br",
		"creci%s.com.br",
		"%screci.com.br",
		"propriedades%s.com.br",
		"%spropriedades.com.br",
		"realestate%s.com.br",
		"%srealestate.com.br",
		"vendas%s.com.br",
		"%svendas.com.br",
		"loteamentos%s.com.br",
		"%sloteamentos.com.br",
	}

	cityNormalized := g.normalizeCityName(city)

	// Configura coletor para validação
	c := colly.NewCollector(
		colly.UserAgent(g.config.UserAgent),
	)

	extensions.RandomUserAgent(c)
	c.SetRequestTimeout(g.config.Timeout)

	// Testa cada padrão
	for _, pattern := range patterns {
		domain := fmt.Sprintf(pattern, cityNormalized)
		testURL := "https://" + domain

		g.logger.WithField("domain", domain).Debug("Testing domain")

		// Testa se o domínio responde
		isValid := false
		c.OnHTML("html", func(e *colly.HTMLElement) {
			// Verifica se tem conteúdo relacionado a imóveis
			text := strings.ToLower(e.Text)
			title := strings.ToLower(e.ChildText("title"))

			keywords := []string{
				"imovel", "imóvel", "casa", "apartamento", "venda", "aluguel",
				"corretor", "imobiliaria", "imobiliária", "creci", "propriedade",
				"terreno", "lote", "comercial", "residencial", "comprar", "alugar",
			}

			keywordCount := 0
			allText := text + " " + title

			for _, keyword := range keywords {
				if strings.Contains(allText, keyword) {
					keywordCount++
				}
			}

			// Reduz critério para 1 keyword para ser mais permissivo
			if keywordCount >= 1 {
				isValid = true
			}
		})

		c.OnError(func(r *colly.Response, err error) {
			// Domínio não existe ou não responde
			isValid = false
		})

		err := c.Visit(testURL)
		if err == nil {
			c.Wait()

			if isValid {
				site := repository.SiteInfo{
					URL:          testURL,
					Name:         fmt.Sprintf("Imobiliária %s", city),
					Domain:       domain,
					Status:       "discovered",
					DiscoveredAt: time.Now(),
				}

				sites = append(sites, site)
			}
		}

		time.Sleep(g.config.DelayBetweenSearches)

		if len(sites) >= g.config.MaxSitesPerCity/2 { // Limita mais para domain guessing
			break
		}
	}

	g.logger.WithFields(map[string]interface{}{
		"city":  city,
		"state": state,
		"sites": len(sites),
	}).Debug("Domain guessing completed")

	return sites, nil
}

func (g *DomainGuessingStrategy) normalizeCityName(city string) string {
	// Remove acentos e caracteres especiais
	normalized := strings.ToLower(city)

	// Remove espaços e substitui por hífen ou remove
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "-", "")

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

	// Remove caracteres não alfanuméricos
	reg := regexp.MustCompile(`[^a-z0-9]`)
	normalized = reg.ReplaceAllString(normalized, "")

	return normalized
}

// DirectorySource representa uma fonte de diretório
type DirectorySource struct {
	Name       string
	URL        string
	SearchPath string
}
