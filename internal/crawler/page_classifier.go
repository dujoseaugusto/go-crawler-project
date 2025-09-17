package crawler

import (
	"regexp"
	"strings"

	"github.com/gocolly/colly"
)

// PageType representa o tipo de página
type PageType int

const (
	PageTypeUnknown PageType = iota
	PageTypeProperty          // Página de anúncio individual
	PageTypeCatalog           // Página de catálogo/listagem
	PageTypeHome              // Página inicial
	PageTypeContact           // Página de contato
)

// PageClassifier classifica tipos de página
type PageClassifier struct {
	// Padrões para identificar páginas de anúncio individual
	propertyIndicators []string
	// Padrões para identificar páginas de catálogo
	catalogIndicators []string
	// Padrões de URL que indicam anúncio individual
	propertyURLPatterns []*regexp.Regexp
	// Padrões de URL que indicam catálogo
	catalogURLPatterns []*regexp.Regexp
}

// NewPageClassifier cria um novo classificador de páginas
func NewPageClassifier() *PageClassifier {
	pc := &PageClassifier{
		// Indicadores de conteúdo para anúncios individuais
		propertyIndicators: []string{
			// Elementos únicos de anúncio
			"área útil", "área total", "iptu", "condomínio",
			"código do imóvel", "ref:", "código:",
			"agendar visita", "entre em contato", "solicitar informações",
			"características do imóvel", "detalhes do imóvel",
			"localização", "mapa", "street view",
			"fotos do imóvel", "galeria de fotos",
			"financiamento", "documentação",
			// Campos específicos
			"dormitórios:", "suítes:", "vagas:",
			"ano de construção", "estado de conservação",
		},

		// Indicadores de conteúdo para catálogos
		catalogIndicators: []string{
			// Elementos de listagem
			"resultados encontrados", "imóveis encontrados",
			"ordenar por", "filtrar por", "buscar por",
			"página anterior", "próxima página", "página 1",
			"ver mais imóveis", "mais resultados",
			"refinar busca", "nova busca",
			"imóveis similares", "outras opções",
			// Múltiplos itens
			"a partir de r$", "valores a partir",
			"diversos imóveis", "várias opções",
		},
	}

	// Compila padrões de URL para anúncios individuais
	propertyPatterns := []string{
		`/imovel/\d+`,           // /imovel/123
		`/propriedade/\d+`,      // /propriedade/456
		`/anuncio/\d+`,          // /anuncio/789
		`/detalhes/\d+`,         // /detalhes/123
		`/casa/\d+`,             // /casa/456
		`/apartamento/\d+`,      // /apartamento/789
		`/ref-\d+`,              // /ref-123
		`/codigo-\d+`,           // /codigo-456
		`/id-\d+`,               // /id-789
		`-\d{4,}/?$`,            // termina com 4+ dígitos
		`/[a-z]+-\d+-[a-z]+`,    // padrão tipo casa-123-venda
	}

	// Compila padrões de URL para catálogos
	catalogPatterns := []string{
		`/imoveis/?$`,                    // /imoveis ou /imoveis/
		`/busca`,                         // qualquer busca
		`/pesquisa`,                      // qualquer pesquisa
		`/listagem`,                      // qualquer listagem
		`/catalogo`,                      // qualquer catálogo
		`/resultados`,                    // qualquer resultado
		`[?&]page=`,                      // parâmetro de página
		`[?&]pagina=`,                    // parâmetro de página em português
		`[?&]filtro`,                     // parâmetros de filtro
		`[?&]ordenar`,                    // parâmetros de ordenação
		`/venda/?$`,                      // apenas /venda
		`/aluguel/?$`,                    // apenas /aluguel
		`/casa/?$`,                       // apenas /casa
		`/apartamento/?$`,                // apenas /apartamento
		`/comercial/?$`,                  // apenas /comercial
		`/categoria/`,                    // páginas de categoria
	}

	// Compila regex patterns
	for _, pattern := range propertyPatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			pc.propertyURLPatterns = append(pc.propertyURLPatterns, compiled)
		}
	}

	for _, pattern := range catalogPatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			pc.catalogURLPatterns = append(pc.catalogURLPatterns, compiled)
		}
	}

	return pc
}

// ClassifyPage classifica uma página baseada na URL e conteúdo
func (pc *PageClassifier) ClassifyPage(e *colly.HTMLElement) PageType {
	url := e.Request.URL.String()
	text := strings.ToLower(e.Text)
	title := strings.ToLower(e.ChildText("title"))

	// 1. Classificação por URL (mais confiável)
	urlType := pc.classifyByURL(url)
	if urlType != PageTypeUnknown {
		return urlType
	}

	// 2. Classificação por conteúdo
	contentType := pc.classifyByContent(text, title)
	if contentType != PageTypeUnknown {
		return contentType
	}

	// 3. Classificação por estrutura HTML
	structureType := pc.classifyByStructure(e)
	return structureType
}

// classifyByURL classifica baseado apenas na URL
func (pc *PageClassifier) classifyByURL(url string) PageType {
	urlLower := strings.ToLower(url)

	// Verifica padrões de anúncio individual
	for _, pattern := range pc.propertyURLPatterns {
		if pattern.MatchString(urlLower) {
			return PageTypeProperty
		}
	}

	// Verifica padrões de catálogo
	for _, pattern := range pc.catalogURLPatterns {
		if pattern.MatchString(urlLower) {
			return PageTypeCatalog
		}
	}

	return PageTypeUnknown
}

// classifyByContent classifica baseado no conteúdo da página
func (pc *PageClassifier) classifyByContent(text, title string) PageType {
	propertyScore := 0
	catalogScore := 0

	// Conta indicadores de anúncio individual
	for _, indicator := range pc.propertyIndicators {
		if strings.Contains(text, indicator) || strings.Contains(title, indicator) {
			propertyScore++
		}
	}

	// Conta indicadores de catálogo
	for _, indicator := range pc.catalogIndicators {
		if strings.Contains(text, indicator) || strings.Contains(title, indicator) {
			catalogScore++
		}
	}

	// Verifica múltiplos preços (forte indicador de catálogo)
	priceCount := strings.Count(text, "r$")
	if priceCount > 5 {
		catalogScore += 3
	}

	// Verifica múltiplas referências a quartos (indicador de catálogo)
	roomCount := strings.Count(text, "quartos")
	if roomCount > 3 {
		catalogScore += 2
	}

	// Decide baseado nos scores
	if propertyScore > catalogScore && propertyScore >= 2 {
		return PageTypeProperty
	}
	if catalogScore > propertyScore && catalogScore >= 2 {
		return PageTypeCatalog
	}

	return PageTypeUnknown
}

// classifyByStructure classifica baseado na estrutura HTML
func (pc *PageClassifier) classifyByStructure(e *colly.HTMLElement) PageType {
	// Conta elementos que indicam anúncio individual
	propertyElements := 0
	
	// Elementos típicos de anúncio individual
	if e.ChildText(".property-details") != "" || 
	   e.ChildText(".imovel-detalhes") != "" ||
	   e.ChildText("#property-info") != "" {
		propertyElements++
	}

	// Galeria de fotos (comum em anúncios individuais)
	photoGallery := len(e.ChildAttrs(".gallery img", "src")) +
	               len(e.ChildAttrs(".fotos img", "src")) +
	               len(e.ChildAttrs(".property-photos img", "src"))
	
	if photoGallery > 3 {
		propertyElements += 2
	}

	// Conta elementos que indicam catálogo
	catalogElements := 0

	// Múltiplos cards/itens de propriedade
	propertyCards := 0
	e.ForEach(".property-card", func(_ int, el *colly.HTMLElement) { propertyCards++ })
	e.ForEach(".imovel-item", func(_ int, el *colly.HTMLElement) { propertyCards++ })
	e.ForEach(".listing-item", func(_ int, el *colly.HTMLElement) { propertyCards++ })
	e.ForEach(".property-listing", func(_ int, el *colly.HTMLElement) { propertyCards++ })

	if propertyCards > 2 {
		catalogElements += 3
	}

	// Elementos de paginação
	if e.ChildText(".pagination") != "" || 
	   e.ChildText(".paginacao") != "" ||
	   e.ChildText(".page-numbers") != "" {
		catalogElements += 2
	}

	// Elementos de filtro
	if e.ChildText(".filters") != "" || 
	   e.ChildText(".filtros") != "" ||
	   e.ChildText(".search-filters") != "" {
		catalogElements += 2
	}

	// Decide baseado na estrutura
	if propertyElements > catalogElements && propertyElements >= 2 {
		return PageTypeProperty
	}
	if catalogElements > propertyElements && catalogElements >= 3 {
		return PageTypeCatalog
	}

	return PageTypeUnknown
}

// IsPropertyPage verifica se é uma página de anúncio individual
func (pc *PageClassifier) IsPropertyPage(e *colly.HTMLElement) bool {
	return pc.ClassifyPage(e) == PageTypeProperty
}

// IsCatalogPage verifica se é uma página de catálogo
func (pc *PageClassifier) IsCatalogPage(e *colly.HTMLElement) bool {
	return pc.ClassifyPage(e) == PageTypeCatalog
}

// GetPageTypeString retorna string representando o tipo da página
func (pc *PageClassifier) GetPageTypeString(pageType PageType) string {
	switch pageType {
	case PageTypeProperty:
		return "property"
	case PageTypeCatalog:
		return "catalog"
	case PageTypeHome:
		return "home"
	case PageTypeContact:
		return "contact"
	default:
		return "unknown"
	}
}
