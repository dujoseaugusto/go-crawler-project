package crawler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ReferenceURLTrainer analisa URLs de referência para aprender padrões de anúncios reais
type ReferenceURLTrainer struct {
	referenceURLs    []string
	propertyPatterns []URLPattern
	excludePatterns  []string
	requiredKeywords []string
}

// URLPattern representa um padrão de URL para anúncios
type URLPattern struct {
	PathPattern   string
	Keywords      []string
	RequiredParts []string
	Confidence    float64
}

// NewReferenceURLTrainer cria um novo treinador baseado em URLs de referência
func NewReferenceURLTrainer(referenceURLs []string) *ReferenceURLTrainer {
	trainer := &ReferenceURLTrainer{
		referenceURLs: referenceURLs,
		// Padrões que definitivamente NÃO são anúncios
		excludePatterns: []string{
			"/sobre", "/about", "/nossa-equipe", "/equipe", "/team",
			"/contato", "/contact", "/fale-conosco",
			"/politica-privacidade", "/privacy", "/termos", "/terms",
			"/servicos", "/services", "/como-funciona",
			"/venda$", "/aluguel$", "/comprar$", "/alugar$", // Páginas de categoria (sem ID)
			"/imoveis$", "/properties$", "/listings$",
			"/buscar", "/search", "/filtros", "/filters",
			"/login", "/cadastro", "/register", "/admin",
			"/blog", "/noticias", "/news", "/dicas",
			"/financiamento", "/financing", "/simulador",
			"/avaliar", "/avaliacao", "/evaluation",
			"/condominios$", "/condominio$", "/empreendimentos$",
			"/imoveis-favoritos", "/favoritos", "/wishlist",
			"/anuncie", "/anuncie-seu-imovel", "/cadastrar-imovel",
			"/documentos", "/docs", "/area-cliente", "/cliente",
		},
		// Palavras-chave que indicam anúncios reais
		requiredKeywords: []string{
			"imovel", "casa", "apartamento", "terreno", "sitio", "chacara",
			"sobrado", "cobertura", "loft", "studio", "comercial", "loja",
			"sala", "galpao", "barracao", "fazenda", "rural",
		},
	}

	trainer.analyzeReferenceURLs()
	return trainer
}

// analyzeReferenceURLs analisa as URLs de referência para extrair padrões
func (r *ReferenceURLTrainer) analyzeReferenceURLs() {
	pathPatterns := make(map[string]int)
	keywordCounts := make(map[string]int)

	for _, rawURL := range r.referenceURLs {
		if strings.TrimSpace(rawURL) == "" {
			continue
		}

		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			continue
		}

		// Analisar padrões do path
		path := strings.ToLower(parsedURL.Path)

		// Extrair padrões comuns
		pathPattern := r.extractPathPattern(path)
		if pathPattern != "" {
			pathPatterns[pathPattern]++
		}

		// Contar palavras-chave
		for _, keyword := range r.requiredKeywords {
			if strings.Contains(path, keyword) {
				keywordCounts[keyword]++
			}
		}
	}

	// Criar padrões baseados na análise
	for pattern, count := range pathPatterns {
		if count >= 2 { // Padrão deve aparecer pelo menos 2 vezes
			confidence := float64(count) / float64(len(r.referenceURLs))

			urlPattern := URLPattern{
				PathPattern:   pattern,
				Keywords:      r.extractKeywordsFromPattern(pattern),
				RequiredParts: r.extractRequiredParts(pattern),
				Confidence:    confidence,
			}

			r.propertyPatterns = append(r.propertyPatterns, urlPattern)
		}
	}
}

// extractPathPattern extrai um padrão generalizável do path
func (r *ReferenceURLTrainer) extractPathPattern(path string) string {
	// Remover números específicos e IDs, mantendo a estrutura
	pattern := path

	// Substituir números por placeholder
	numberRegex := regexp.MustCompile(`\d+`)
	pattern = numberRegex.ReplaceAllString(pattern, "{ID}")

	// Substituir códigos específicos (ex: CA0977-TEI5, AP0048_GUAXU)
	codeRegex := regexp.MustCompile(`[A-Z]{2}\d+[_-][A-Z0-9]+`)
	pattern = codeRegex.ReplaceAllString(pattern, "{CODE}")

	// Substituir GUIDs e hashes longos
	guidRegex := regexp.MustCompile(`[a-f0-9]{8,}`)
	pattern = guidRegex.ReplaceAllString(pattern, "{GUID}")

	return pattern
}

// extractKeywordsFromPattern extrai palavras-chave relevantes do padrão
func (r *ReferenceURLTrainer) extractKeywordsFromPattern(pattern string) []string {
	var keywords []string
	lowerPattern := strings.ToLower(pattern)

	for _, keyword := range r.requiredKeywords {
		if strings.Contains(lowerPattern, keyword) {
			keywords = append(keywords, keyword)
		}
	}

	return keywords
}

// extractRequiredParts extrai partes obrigatórias do padrão
func (r *ReferenceURLTrainer) extractRequiredParts(pattern string) []string {
	parts := strings.Split(pattern, "/")
	var requiredParts []string

	for _, part := range parts {
		if part != "" && !strings.Contains(part, "{") {
			// Partes que não são placeholders são obrigatórias
			requiredParts = append(requiredParts, part)
		}
	}

	return requiredParts
}

// IsPropertyURL verifica se uma URL é provavelmente um anúncio de imóvel
func (r *ReferenceURLTrainer) IsPropertyURL(rawURL string) (bool, float64, string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false, 0.0, "URL inválida"
	}

	path := strings.ToLower(parsedURL.Path)

	// 1. Verificar padrões de exclusão (páginas que definitivamente NÃO são anúncios)
	for _, excludePattern := range r.excludePatterns {
		matched, _ := regexp.MatchString(excludePattern, path)
		if matched {
			return false, 0.0, fmt.Sprintf("Excluído por padrão: %s", excludePattern)
		}
	}

	// 2. Verificar se tem pelo menos uma palavra-chave de imóvel
	hasKeyword := false
	for _, keyword := range r.requiredKeywords {
		if strings.Contains(path, keyword) {
			hasKeyword = true
			break
		}
	}

	if !hasKeyword {
		return false, 0.0, "Não contém palavras-chave de imóvel"
	}

	// 3. Verificar se corresponde aos padrões aprendidos
	bestMatch := 0.0
	bestReason := "Não corresponde aos padrões aprendidos"

	currentPattern := r.extractPathPattern(path)

	for _, pattern := range r.propertyPatterns {
		similarity := r.calculatePatternSimilarity(currentPattern, pattern.PathPattern)
		if similarity > bestMatch {
			bestMatch = similarity
			bestReason = fmt.Sprintf("Corresponde ao padrão: %s (%.2f)", pattern.PathPattern, similarity)
		}
	}

	// 4. Verificações adicionais para aumentar a confiança
	confidence := bestMatch

	// Bonus se tem ID/código no final
	if regexp.MustCompile(`/\d+/?$`).MatchString(path) ||
		regexp.MustCompile(`/[A-Z]{2}\d+[_-][A-Z0-9]+/?$`).MatchString(path) {
		confidence += 0.2
	}

	// Bonus se tem estrutura hierárquica típica de anúncio
	if strings.Count(path, "/") >= 3 {
		confidence += 0.1
	}

	// Penalty se é muito genérico
	if strings.Count(path, "/") <= 1 {
		confidence -= 0.3
	}

	// Considerar como anúncio se confiança >= 0.6
	isProperty := confidence >= 0.6

	return isProperty, confidence, bestReason
}

// calculatePatternSimilarity calcula similaridade entre dois padrões
func (r *ReferenceURLTrainer) calculatePatternSimilarity(pattern1, pattern2 string) float64 {
	if pattern1 == pattern2 {
		return 1.0
	}

	parts1 := strings.Split(pattern1, "/")
	parts2 := strings.Split(pattern2, "/")

	// Calcular similaridade baseada em partes comuns
	commonParts := 0
	maxParts := len(parts1)
	if len(parts2) > maxParts {
		maxParts = len(parts2)
	}

	minParts := len(parts1)
	if len(parts2) < minParts {
		minParts = len(parts2)
	}

	for i := 0; i < minParts; i++ {
		if parts1[i] == parts2[i] ||
			(strings.Contains(parts1[i], "{") && strings.Contains(parts2[i], "{")) {
			commonParts++
		}
	}

	return float64(commonParts) / float64(maxParts)
}

// GetLearnedPatterns retorna os padrões aprendidos
func (r *ReferenceURLTrainer) GetLearnedPatterns() []URLPattern {
	return r.propertyPatterns
}

// PrintAnalysis imprime análise dos padrões aprendidos
func (r *ReferenceURLTrainer) PrintAnalysis() {
	fmt.Printf("=== Análise dos Padrões de URLs de Referência ===\n")
	fmt.Printf("URLs de referência analisadas: %d\n", len(r.referenceURLs))
	fmt.Printf("Padrões identificados: %d\n\n", len(r.propertyPatterns))

	for i, pattern := range r.propertyPatterns {
		fmt.Printf("Padrão %d:\n", i+1)
		fmt.Printf("  Path: %s\n", pattern.PathPattern)
		fmt.Printf("  Keywords: %v\n", pattern.Keywords)
		fmt.Printf("  Required Parts: %v\n", pattern.RequiredParts)
		fmt.Printf("  Confidence: %.2f\n\n", pattern.Confidence)
	}

	fmt.Printf("Padrões de exclusão: %v\n", r.excludePatterns)
}
