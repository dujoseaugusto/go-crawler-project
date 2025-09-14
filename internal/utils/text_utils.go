package utils

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeText normaliza texto removendo acentos, convertendo para minúsculo e removendo caracteres especiais
func NormalizeText(text string) string {
	if text == "" {
		return ""
	}

	// Converter para minúsculo
	text = strings.ToLower(text)

	// Remover acentos usando transform
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	normalized, _, _ := transform.String(t, text)

	// Remover caracteres especiais extras, manter apenas letras, números e espaços
	reg := regexp.MustCompile(`[^a-z0-9\s]`)
	normalized = reg.ReplaceAllString(normalized, "")

	// Remover espaços extras
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

	return strings.TrimSpace(normalized)
}

// CreateSearchTerms cria termos de busca a partir de uma query
func CreateSearchTerms(query string) []string {
	if query == "" {
		return []string{}
	}

	// Normalizar a query
	normalized := NormalizeText(query)

	// Dividir em palavras
	words := strings.Fields(normalized)

	// Filtrar palavras muito pequenas (stopwords básicas)
	var terms []string
	stopwords := map[string]bool{
		"a": true, "o": true, "e": true, "de": true, "da": true, "do": true,
		"em": true, "na": true, "no": true, "com": true, "por": true, "para": true,
		"um": true, "uma": true, "os": true, "as": true, "dos": true, "das": true,
	}

	for _, word := range words {
		if len(word) >= 2 && !stopwords[word] {
			terms = append(terms, word)
		}
	}

	return terms
}

// BuildSearchRegex cria uma regex para busca flexível
func BuildSearchRegex(terms []string) string {
	if len(terms) == 0 {
		return ""
	}

	// Criar padrão que busca qualquer um dos termos
	var patterns []string
	for _, term := range terms {
		// Escapar caracteres especiais da regex
		escaped := regexp.QuoteMeta(term)
		patterns = append(patterns, escaped)
	}

	// Criar regex que busca qualquer termo (OR)
	return strings.Join(patterns, "|")
}

// CalculateRelevanceScore calcula pontuação de relevância para um texto
func CalculateRelevanceScore(text string, searchTerms []string) float64 {
	if len(searchTerms) == 0 {
		return 0
	}

	normalizedText := NormalizeText(text)
	score := 0.0

	for _, term := range searchTerms {
		// Contar ocorrências do termo
		count := strings.Count(normalizedText, term)
		if count > 0 {
			// Pontuação baseada na frequência e tamanho do termo
			termScore := float64(count) * (1.0 + float64(len(term))/10.0)
			score += termScore
		}
	}

	// Normalizar pela quantidade de termos
	return score / float64(len(searchTerms))
}

// IsTextMatch verifica se um texto contém algum dos termos de busca
func IsTextMatch(text string, searchTerms []string) bool {
	if len(searchTerms) == 0 {
		return true
	}

	normalizedText := NormalizeText(text)
	
	for _, term := range searchTerms {
		if strings.Contains(normalizedText, term) {
			return true
		}
	}
	
	return false
}

// HighlightMatches destaca os termos encontrados no texto (para futuro uso em UI)
func HighlightMatches(text string, searchTerms []string) string {
	if len(searchTerms) == 0 {
		return text
	}

	result := text
	for _, term := range searchTerms {
		// Criar regex case-insensitive
		pattern := "(?i)" + regexp.QuoteMeta(term)
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			return "<mark>" + match + "</mark>"
		})
	}

	return result
}
