package crawler

import (
	"regexp"
	"strings"

	"github.com/gocolly/colly"
)

// AdvancedPageClassifier é um classificador mais rigoroso para páginas
type AdvancedPageClassifier struct {
	// Padrões que GARANTEM que é catálogo
	strongCatalogPatterns []*regexp.Regexp
	// Padrões que GARANTEM que é anúncio individual
	strongPropertyPatterns []*regexp.Regexp
	// Frases que indicam catálogo
	catalogPhrases []string
	// Frases que indicam anúncio individual
	propertyPhrases []string
}

// NewAdvancedPageClassifier cria um classificador mais rigoroso
func NewAdvancedPageClassifier() *AdvancedPageClassifier {
	apc := &AdvancedPageClassifier{
		// Frases que GARANTEM que é catálogo
		catalogPhrases: []string{
			"você encontra diversos",
			"você encontra mais de",
			"imóveis encontrados",
			"resultados encontrados",
			"escolha o seu imóvel",
			"diversas opções",
			"várias opções",
			"mais opções",
			"outros imóveis",
			"imóveis similares",
			"ordenar por",
			"filtrar por",
			"refinar busca",
			"nova busca",
			"página anterior",
			"próxima página",
			"ver mais imóveis",
			"mais resultados",
			"todos os bairros",
			"todas as regiões",
		},

		// Frases que GARANTEM que é anúncio individual
		propertyPhrases: []string{
			"código do imóvel:",
			"ref:",
			"referência:",
			"agendar visita",
			"solicitar informações",
			"entre em contato",
			"características do imóvel",
			"detalhes do imóvel",
			"documentação regular",
			"financiamento disponível",
			"iptu:",
			"condomínio:",
			"área útil:",
			"área total:",
			"ano de construção:",
			"estado de conservação:",
			"aceita financiamento",
			"aceita fgts",
		},
	}

	// Padrões de URL que GARANTEM catálogo
	strongCatalogURLs := []string{
		`/venda/?$`,               // apenas /venda
		`/aluguel/?$`,             // apenas /aluguel
		`/casa/?$`,                // apenas /casa
		`/apartamento/?$`,         // apenas /apartamento
		`/comercial/?$`,           // apenas /comercial
		`/imoveis/?$`,             // apenas /imoveis
		`/busca`,                  // qualquer busca
		`/pesquisa`,               // qualquer pesquisa
		`/listagem`,               // qualquer listagem
		`/catalogo`,               // qualquer catálogo
		`/resultados`,             // qualquer resultado
		`[?&]page=`,               // parâmetro de página
		`[?&]pagina=`,             // parâmetro de página
		`[?&]filtro`,              // parâmetros de filtro
		`[?&]ordenar`,             // parâmetros de ordenação
		`/categoria/`,             // páginas de categoria
		`/bairro/[^/]+/?$`,        // listagem por bairro
		`/cidade/[^/]+/?$`,        // listagem por cidade
		`/regiao/[^/]+/?$`,        // listagem por região
		`/tipo/[^/]+/?$`,          // listagem por tipo
		`/preco/[^/]+/?$`,         // listagem por preço
		`/quartos/[^/]+/?$`,       // listagem por quartos
		`/venda/[^/]+/[^/]+/?$`,   // /venda/tipo/bairro
		`/aluguel/[^/]+/[^/]+/?$`, // /aluguel/tipo/bairro
	}

	// Padrões de URL que GARANTEM anúncio individual
	strongPropertyURLs := []string{
		`/imovel/\d+`,            // /imovel/123
		`/propriedade/\d+`,       // /propriedade/456
		`/anuncio/\d+`,           // /anuncio/789
		`/detalhes/\d+`,          // /detalhes/123
		`/ref-\d+`,               // /ref-123
		`/codigo-\d+`,            // /codigo-456
		`/id-\d+`,                // /id-789
		`-\d{5,}/?$`,             // termina com 5+ dígitos (mais específico)
		`/imovel-\d+`,            // /imovel-123
		`/casa-\d+`,              // /casa-456
		`/apartamento-\d+`,       // /apartamento-789
		`/[a-z]+-\d+-[a-z]+-\d+`, // padrão complexo com múltiplos números
	}

	// Compila padrões de catálogo
	for _, pattern := range strongCatalogURLs {
		if compiled, err := regexp.Compile(pattern); err == nil {
			apc.strongCatalogPatterns = append(apc.strongCatalogPatterns, compiled)
		}
	}

	// Compila padrões de anúncio
	for _, pattern := range strongPropertyURLs {
		if compiled, err := regexp.Compile(pattern); err == nil {
			apc.strongPropertyPatterns = append(apc.strongPropertyPatterns, compiled)
		}
	}

	return apc
}

// ClassifyPageStrict classifica uma página com critérios mais rigorosos
func (apc *AdvancedPageClassifier) ClassifyPageStrict(e *colly.HTMLElement) PageType {
	url := e.Request.URL.String()
	text := strings.ToLower(e.Text)
	title := strings.ToLower(e.ChildText("title"))

	// 1. PRIMEIRO: Verifica se é DEFINITIVAMENTE catálogo
	if apc.isDefinitelyCatalog(url, text, title) {
		return PageTypeCatalog
	}

	// 2. SEGUNDO: Verifica se é DEFINITIVAMENTE anúncio individual
	if apc.isDefinitelyProperty(url, text, title, e) {
		return PageTypeProperty
	}

	// 3. TERCEIRO: Se não tem certeza, considera UNKNOWN (não processa)
	return PageTypeUnknown
}

// isDefinitelyCatalog verifica se é DEFINITIVAMENTE uma página de catálogo
func (apc *AdvancedPageClassifier) isDefinitelyCatalog(url, text, title string) bool {
	urlLower := strings.ToLower(url)

	// Verifica padrões de URL que garantem catálogo
	for _, pattern := range apc.strongCatalogPatterns {
		if pattern.MatchString(urlLower) {
			return true
		}
	}

	// Verifica frases que garantem catálogo
	for _, phrase := range apc.catalogPhrases {
		if strings.Contains(text, phrase) || strings.Contains(title, phrase) {
			return true
		}
	}

	// Verifica múltiplos preços (forte indicador de catálogo)
	priceCount := strings.Count(text, "r$")
	if priceCount > 8 {
		return true
	}

	// Verifica múltiplas referências a quartos
	roomCount := strings.Count(text, "quartos")
	if roomCount > 5 {
		return true
	}

	// Verifica múltiplas referências a "imóveis"
	propertyCount := strings.Count(text, "imóveis")
	if propertyCount > 3 {
		return true
	}

	return false
}

// isDefinitelyProperty verifica se é DEFINITIVAMENTE um anúncio individual
func (apc *AdvancedPageClassifier) isDefinitelyProperty(url, text, title string, e *colly.HTMLElement) bool {
	urlLower := strings.ToLower(url)

	// Verifica padrões de URL que garantem anúncio individual
	for _, pattern := range apc.strongPropertyPatterns {
		if pattern.MatchString(urlLower) {
			return true
		}
	}

	// Verifica frases que garantem anúncio individual
	propertyPhraseCount := 0
	for _, phrase := range apc.propertyPhrases {
		if strings.Contains(text, phrase) || strings.Contains(title, phrase) {
			propertyPhraseCount++
		}
	}

	// Precisa de pelo menos 2 frases específicas de anúncio
	if propertyPhraseCount >= 2 {
		return true
	}

	// Verifica estrutura HTML específica de anúncio individual
	if apc.hasPropertyStructure(e) {
		return true
	}

	// Verifica se tem galeria de fotos (comum em anúncios individuais)
	photoCount := len(e.ChildAttrs(".gallery img", "src")) +
		len(e.ChildAttrs(".fotos img", "src")) +
		len(e.ChildAttrs(".property-photos img", "src")) +
		len(e.ChildAttrs(".imovel-fotos img", "src"))

	// Se tem muitas fotos E pelo menos 1 frase específica
	if photoCount > 5 && propertyPhraseCount >= 1 {
		return true
	}

	return false
}

// hasPropertyStructure verifica se tem estrutura HTML de anúncio individual
func (apc *AdvancedPageClassifier) hasPropertyStructure(e *colly.HTMLElement) bool {
	structureScore := 0

	// Elementos típicos de anúncio individual
	if e.ChildText(".property-details") != "" ||
		e.ChildText(".imovel-detalhes") != "" ||
		e.ChildText("#property-info") != "" ||
		e.ChildText(".detalhes-imovel") != "" {
		structureScore += 2
	}

	// Campos específicos de propriedade
	if e.ChildText(".property-price") != "" ||
		e.ChildText(".preco-imovel") != "" ||
		e.ChildText(".valor-imovel") != "" {
		structureScore++
	}

	if e.ChildText(".property-address") != "" ||
		e.ChildText(".endereco-imovel") != "" ||
		e.ChildText(".localizacao") != "" {
		structureScore++
	}

	// Informações de contato específicas
	if e.ChildText(".contact-form") != "" ||
		e.ChildText(".formulario-contato") != "" ||
		e.ChildText(".agendar-visita") != "" {
		structureScore++
	}

	return structureScore >= 3
}

// IsPropertyPageStrict verifica se é anúncio individual com critério rigoroso
func (apc *AdvancedPageClassifier) IsPropertyPageStrict(e *colly.HTMLElement) bool {
	return apc.ClassifyPageStrict(e) == PageTypeProperty
}

// IsCatalogPageStrict verifica se é catálogo com critério rigoroso
func (apc *AdvancedPageClassifier) IsCatalogPageStrict(e *colly.HTMLElement) bool {
	return apc.ClassifyPageStrict(e) == PageTypeCatalog
}
