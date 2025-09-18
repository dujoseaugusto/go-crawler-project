package ai

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/google/generative-ai-go/genai"
)

// PageClassificationResult representa o resultado da classificação de página pela IA
type PageClassificationResult struct {
	IsPropertyPage bool     `json:"is_property_page"`
	Confidence     float64  `json:"confidence"`
	Reasoning      string   `json:"reasoning"`
	PageType       string   `json:"page_type"` // "property", "catalog", "homepage", "other"
	Indicators     []string `json:"indicators"`
}

// SelectorSuggestion representa uma sugestão de seletor CSS pela IA
type SelectorSuggestion struct {
	DataType   string  `json:"data_type"`  // "price", "address", "description", etc.
	Selector   string  `json:"selector"`   // CSS selector sugerido
	Confidence float64 `json:"confidence"` // Confiança na sugestão (0-1)
	Reasoning  string  `json:"reasoning"`  // Por que este seletor foi sugerido
}

// PatternAnalysisResult representa análise de padrões pela IA
type PatternAnalysisResult struct {
	URLPattern      string               `json:"url_pattern"`
	Selectors       []SelectorSuggestion `json:"selectors"`
	PageStructure   string               `json:"page_structure"`
	Recommendations []string             `json:"recommendations"`
}

// EnhancedGeminiService estende o GeminiService com funcionalidades de análise de padrões
type EnhancedGeminiService struct {
	*GeminiService
	classificationCache map[string]*PageClassificationResult
	patternCache        map[string]*PatternAnalysisResult
	cacheMutex          sync.RWMutex
}

// NewEnhancedGeminiService cria uma nova instância do serviço Gemini melhorado
func NewEnhancedGeminiService(ctx context.Context) (*EnhancedGeminiService, error) {
	baseService, err := NewGeminiService(ctx)
	if err != nil {
		return nil, err
	}

	return &EnhancedGeminiService{
		GeminiService:       baseService,
		classificationCache: make(map[string]*PageClassificationResult),
		patternCache:        make(map[string]*PatternAnalysisResult),
	}, nil
}

// ClassifyPageContent usa IA para classificar se uma página é de anúncio de imóvel
func (egs *EnhancedGeminiService) ClassifyPageContent(ctx context.Context, url, title, content string) (*PageClassificationResult, error) {
	// Gera chave de cache
	cacheKey := fmt.Sprintf("classify_%x", generateHash(url+title+content[:min(len(content), 500)]))

	// Verifica cache
	egs.cacheMutex.RLock()
	if cached, exists := egs.classificationCache[cacheKey]; exists {
		egs.cacheMutex.RUnlock()
		return cached, nil
	}
	egs.cacheMutex.RUnlock()

	// Cria prompt para classificação
	prompt := createClassificationPrompt(url, title, content)

	// Chama API do Gemini
	resp, err := egs.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao classificar página com Gemini: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia do Gemini para classificação")
	}

	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)
	result, err := parseClassificationResponse(string(responseText))
	if err != nil {
		return nil, err
	}

	// Armazena no cache
	egs.cacheMutex.Lock()
	egs.classificationCache[cacheKey] = result
	egs.cacheMutex.Unlock()

	return result, nil
}

// AnalyzePagePatterns usa IA para analisar padrões de uma página e sugerir seletores
func (egs *EnhancedGeminiService) AnalyzePagePatterns(ctx context.Context, url, htmlContent string) (*PatternAnalysisResult, error) {
	// Gera chave de cache
	cacheKey := fmt.Sprintf("pattern_%x", generateHash(url+htmlContent[:min(len(htmlContent), 1000)]))

	// Verifica cache
	egs.cacheMutex.RLock()
	if cached, exists := egs.patternCache[cacheKey]; exists {
		egs.cacheMutex.RUnlock()
		return cached, nil
	}
	egs.cacheMutex.RUnlock()

	// Cria prompt para análise de padrões
	prompt := createPatternAnalysisPrompt(url, htmlContent)

	// Chama API do Gemini
	resp, err := egs.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao analisar padrões com Gemini: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia do Gemini para análise de padrões")
	}

	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)
	result, err := parsePatternAnalysisResponse(string(responseText))
	if err != nil {
		return nil, err
	}

	// Armazena no cache
	egs.cacheMutex.Lock()
	egs.patternCache[cacheKey] = result
	egs.cacheMutex.Unlock()

	return result, nil
}

// ValidateExtractedData usa IA para validar e melhorar dados extraídos
func (egs *EnhancedGeminiService) ValidateExtractedData(ctx context.Context, property repository.Property, originalHTML string) (repository.Property, error) {
	// Se os dados parecem completos e válidos, não precisa de validação IA
	if egs.isDataComplete(property) {
		return property, nil
	}

	prompt := createValidationPrompt(property, originalHTML)

	resp, err := egs.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return property, fmt.Errorf("erro ao validar dados com Gemini: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return property, fmt.Errorf("resposta vazia do Gemini para validação")
	}

	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)
	validated, err := parseValidationResponse(string(responseText), property)
	if err != nil {
		return property, err
	}

	return validated, nil
}

// SuggestSelectorsForSite usa IA para sugerir seletores CSS para um site específico
func (egs *EnhancedGeminiService) SuggestSelectorsForSite(ctx context.Context, domain, sampleHTML string) ([]SelectorSuggestion, error) {
	prompt := createSelectorSuggestionPrompt(domain, sampleHTML)

	resp, err := egs.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao sugerir seletores com Gemini: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia do Gemini para sugestão de seletores")
	}

	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)
	suggestions, err := parseSelectorSuggestions(string(responseText))
	if err != nil {
		return nil, err
	}

	return suggestions, nil
}

// createClassificationPrompt cria prompt para classificação de página
func createClassificationPrompt(url, title, content string) string {
	// Trunca conteúdo para economizar tokens
	truncatedContent := truncateString(content, 2000)

	return fmt.Sprintf(`Analise se esta página é um anúncio individual de imóvel para venda/aluguel.

URL: %s
Título: %s
Conteúdo: %s

Responda em JSON com:
{
  "is_property_page": boolean,
  "confidence": float (0-1),
  "page_type": "property|catalog|homepage|other",
  "reasoning": "explicação da decisão",
  "indicators": ["lista", "de", "indicadores", "encontrados"]
}

Indicadores de anúncio individual:
- Preço específico (não "a partir de")
- Endereço específico
- Características detalhadas (quartos, banheiros, área)
- Galeria de fotos
- Botão "agendar visita" ou contato
- Código/referência do imóvel

Indicadores de catálogo/listagem:
- Múltiplos preços
- "Resultados encontrados"
- Filtros de busca
- Paginação
- "Ver mais imóveis"`,
		truncateString(url, 200),
		truncateString(title, 200),
		truncatedContent)
}

// createPatternAnalysisPrompt cria prompt para análise de padrões
func createPatternAnalysisPrompt(url, htmlContent string) string {
	// Extrai apenas as classes CSS e estrutura relevante
	truncatedHTML := extractRelevantHTML(htmlContent, 3000)

	return fmt.Sprintf(`Analise esta página de imóvel e sugira seletores CSS para extrair dados.

URL: %s
HTML (estrutura): %s

Responda em JSON com:
{
  "url_pattern": "padrão da URL para identificar páginas similares",
  "selectors": [
    {
      "data_type": "price|address|description|rooms|bathrooms|area|features",
      "selector": "seletor CSS sugerido",
      "confidence": float (0-1),
      "reasoning": "por que este seletor"
    }
  ],
  "page_structure": "descrição da estrutura da página",
  "recommendations": ["sugestões", "para", "melhorar", "extração"]
}

Foque em seletores específicos e únicos que identifiquem claramente cada tipo de dado.`,
		truncateString(url, 200),
		truncatedHTML)
}

// createValidationPrompt cria prompt para validação de dados
func createValidationPrompt(property repository.Property, originalHTML string) string {
	htmlSnippet := extractRelevantHTML(originalHTML, 1500)

	return fmt.Sprintf(`Valide e corrija os dados extraídos deste imóvel usando o HTML original.

Dados extraídos:
- Endereço: %s
- Cidade: %s
- Bairro: %s
- Valor: %.2f
- Quartos: %d
- Banheiros: %d
- Área: %.2f
- Tipo: %s
- Descrição: %s

HTML original: %s

Responda em JSON com os dados corrigidos:
{
  "endereco": "endereço corrigido",
  "cidade": "cidade corrigida", 
  "bairro": "bairro corrigido",
  "valor": float,
  "quartos": int,
  "banheiros": int,
  "area_total": float,
  "tipo_imovel": "tipo corrigido",
  "descricao": "descrição limpa",
  "corrections_made": ["lista", "de", "correções", "feitas"]
}`,
		property.Endereco, property.Cidade, property.Bairro,
		property.Valor, property.Quartos, property.Banheiros,
		property.AreaTotal, property.TipoImovel,
		truncateString(property.Descricao, 300),
		htmlSnippet)
}

// createSelectorSuggestionPrompt cria prompt para sugestão de seletores
func createSelectorSuggestionPrompt(domain, sampleHTML string) string {
	htmlSnippet := extractRelevantHTML(sampleHTML, 2500)

	return fmt.Sprintf(`Analise este HTML de %s e sugira os melhores seletores CSS para extrair dados de imóveis.

HTML: %s

Responda em JSON array com:
[
  {
    "data_type": "price|address|description|rooms|bathrooms|area|features|title",
    "selector": "seletor CSS específico",
    "confidence": float (0-1),
    "reasoning": "justificativa para este seletor"
  }
]

Priorize seletores:
1. Específicos (classes únicas)
2. Estáveis (não dependem de posição)
3. Semânticos (nomes descritivos)
4. Testáveis (únicos na página)`,
		domain, htmlSnippet)
}

// parseClassificationResponse processa resposta de classificação
func parseClassificationResponse(response string) (*PageClassificationResult, error) {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("não foi possível extrair JSON da resposta de classificação")
	}

	var result PageClassificationResult
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer parse da resposta de classificação: %v", err)
	}

	return &result, nil
}

// parsePatternAnalysisResponse processa resposta de análise de padrões
func parsePatternAnalysisResponse(response string) (*PatternAnalysisResult, error) {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("não foi possível extrair JSON da resposta de análise")
	}

	var result PatternAnalysisResult
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer parse da resposta de análise: %v", err)
	}

	return &result, nil
}

// parseValidationResponse processa resposta de validação
func parseValidationResponse(response string, original repository.Property) (repository.Property, error) {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return original, fmt.Errorf("não foi possível extrair JSON da resposta de validação")
	}

	var validationData map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &validationData)
	if err != nil {
		return original, fmt.Errorf("erro ao fazer parse da resposta de validação: %v", err)
	}

	// Atualiza propriedade com dados validados
	result := updatePropertyFromMap(original, validationData)
	return result, nil
}

// parseSelectorSuggestions processa resposta de sugestões de seletores
func parseSelectorSuggestions(response string) ([]SelectorSuggestion, error) {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("não foi possível extrair JSON da resposta de seletores")
	}

	var suggestions []SelectorSuggestion
	err := json.Unmarshal([]byte(jsonStr), &suggestions)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer parse das sugestões de seletores: %v", err)
	}

	return suggestions, nil
}

// extractRelevantHTML extrai partes relevantes do HTML para análise
func extractRelevantHTML(html string, maxLen int) string {
	// Remove scripts e styles
	html = removeScriptsAndStyles(html)

	// Foca em elementos com classes que parecem relevantes para imóveis
	relevantClasses := []string{
		"price", "preco", "valor", "value",
		"address", "endereco", "location", "localizacao",
		"description", "descricao", "details", "detalhes",
		"property", "imovel", "listing", "anuncio",
		"features", "caracteristicas", "amenities",
		"rooms", "quartos", "bedrooms", "dormitorios",
		"bathrooms", "banheiros", "area", "size",
	}

	var relevantParts []string
	lines := strings.Split(html, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Verifica se a linha contém classes relevantes
		for _, class := range relevantClasses {
			if strings.Contains(strings.ToLower(line), class) {
				relevantParts = append(relevantParts, line)
				break
			}
		}

		// Para quando atingir o limite
		if len(strings.Join(relevantParts, "\n")) > maxLen {
			break
		}
	}

	result := strings.Join(relevantParts, "\n")
	if len(result) > maxLen {
		result = result[:maxLen] + "..."
	}

	return result
}

// removeScriptsAndStyles remove scripts e styles do HTML
func removeScriptsAndStyles(html string) string {
	// Remove scripts
	scriptRegex := regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`)
	html = scriptRegex.ReplaceAllString(html, "")

	// Remove styles
	styleRegex := regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)
	html = styleRegex.ReplaceAllString(html, "")

	return html
}

// isDataComplete verifica se os dados estão completos o suficiente
func (egs *EnhancedGeminiService) isDataComplete(property repository.Property) bool {
	hasBasicInfo := property.Endereco != "" && property.Valor > 0 && property.TipoImovel != ""
	hasLocationInfo := property.Cidade != "" && property.Bairro != ""
	hasDetailedInfo := property.Quartos > 0 && property.Banheiros > 0
	hasDescription := len(property.Descricao) > 50

	return hasBasicInfo && hasLocationInfo && hasDetailedInfo && hasDescription
}

// generateHash gera hash simples para cache
func generateHash(content string) string {
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// min retorna o menor entre dois inteiros
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
