package crawler

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
)

// ContentBasedClassifier classifica páginas baseado em padrões de conteúdo aprendidos
type ContentBasedClassifier struct {
	patterns *ContentPatterns
	logger   *logger.Logger
}

// ClassificationResult resultado da classificação
type ClassificationResult struct {
	IsProperty bool                  `json:"is_property"`
	Confidence float64               `json:"confidence"`
	Reason     string                `json:"reason"`
	Score      float64               `json:"score"`
	MaxScore   float64               `json:"max_score"`
	Details    ClassificationDetails `json:"details"`
}

// ClassificationDetails detalhes da classificação
type ClassificationDetails struct {
	HasPrice            bool     `json:"has_price"`
	HasDetails          bool     `json:"has_details"`
	HasRequiredElements bool     `json:"has_required_elements"`
	WordCount           int      `json:"word_count"`
	KeywordsFound       []string `json:"keywords_found"`
	SelectorsFound      []string `json:"selectors_found"`
}

// NewContentBasedClassifier cria um novo classificador baseado em conteúdo
func NewContentBasedClassifier(patterns *ContentPatterns) *ContentBasedClassifier {
	return &ContentBasedClassifier{
		patterns: patterns,
		logger:   logger.NewLogger("content_based_classifier"),
	}
}

// ClassifyPage classifica uma página baseado nos padrões aprendidos
func (cbc *ContentBasedClassifier) ClassifyPage(doc *goquery.Document, url string) ClassificationResult {
	if doc == nil {
		return ClassificationResult{
			IsProperty: false,
			Confidence: 0.0,
			Reason:     "Documento HTML inválido",
		}
	}

	result := ClassificationResult{
		Details: ClassificationDetails{
			KeywordsFound:  []string{},
			SelectorsFound: []string{},
		},
	}

	// Extrair texto da página
	pageText := strings.ToLower(doc.Text())
	words := strings.Fields(pageText)
	result.Details.WordCount = len(words)

	score := 0.0
	maxScore := 0.0

	// 1. REMOVIDO: Verificação de palavras proibidas
	// Foco apenas em detectar características POSITIVAS de anúncios
	maxScore += 10.0
	score += 10.0 // Score base para todas as páginas

	// 2. Verificar contagem de palavras
	maxScore += 5.0
	if result.Details.WordCount >= cbc.patterns.MinTextLength &&
		result.Details.WordCount <= cbc.patterns.MaxTextLength {
		score += 5.0
	} else if result.Details.WordCount >= 50 { // Mínimo absoluto
		score += 2.0
	}

	// 3. Verificar palavras-chave obrigatórias
	maxScore += 15.0
	keywordsFound := cbc.checkRequiredKeywords(pageText)
	result.Details.KeywordsFound = keywordsFound
	keywordScore := float64(len(keywordsFound)) * 2.0
	if keywordScore > 15.0 {
		keywordScore = 15.0
	}
	score += keywordScore

	// 4. Verificar elementos de preço
	maxScore += 20.0
	hasPrice, priceScore := cbc.checkPriceElements(doc, pageText)
	result.Details.HasPrice = hasPrice
	score += priceScore

	// 5. Verificar elementos de detalhes
	maxScore += 15.0
	hasDetails, detailScore := cbc.checkDetailElements(doc, pageText)
	result.Details.HasDetails = hasDetails
	score += detailScore

	// 6. Verificar seletores comuns
	maxScore += 10.0
	selectorsFound := cbc.checkCommonSelectors(doc)
	result.Details.SelectorsFound = selectorsFound
	selectorScore := float64(len(selectorsFound)) * 1.0
	if selectorScore > 10.0 {
		selectorScore = 10.0
	}
	score += selectorScore

	// 7. Verificar elementos obrigatórios
	maxScore += 10.0
	hasRequiredElements := cbc.checkRequiredElements(doc)
	result.Details.HasRequiredElements = hasRequiredElements
	if hasRequiredElements {
		score += 10.0
	}

	// 8. Verificar estrutura geral da página
	maxScore += 15.0
	structureScore := cbc.checkPageStructure(doc)
	score += structureScore

	// Calcular confiança final
	confidence := score / maxScore
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Critérios MUITO MENOS RESTRITIVOS para coletar mais anúncios
	minRequirements := result.Details.WordCount >= 30 // Conteúdo mínimo reduzido

	// Aceitar se tem qualquer evidência de anúncio
	hasAnyEvidence := hasPrice || hasDetails || len(keywordsFound) >= 1

	// Ou se tem confiança baixa mas elementos básicos
	hasBasicConfidence := confidence >= 0.1 && hasRequiredElements

	// Ou se simplesmente tem palavras relacionadas a imóveis
	hasPropertyWords := len(keywordsFound) > 0

	isProperty := minRequirements && (hasAnyEvidence || hasBasicConfidence || hasPropertyWords)

	// Gerar razão
	reason := cbc.generateReason(result.Details, confidence, score, maxScore)

	result.IsProperty = isProperty
	result.Confidence = confidence
	result.Reason = reason
	result.Score = score
	result.MaxScore = maxScore

	cbc.logger.WithFields(map[string]interface{}{
		"url":         url,
		"is_property": isProperty,
		"confidence":  confidence,
		"score":       fmt.Sprintf("%.1f/%.1f", score, maxScore),
		"word_count":  result.Details.WordCount,
		"keywords":    len(keywordsFound),
		"has_price":   hasPrice,
		"has_details": hasDetails,
	}).Debug("Content classification completed")

	return result
}

// checkForbiddenKeywords REMOVIDO - não usar palavras proibidas
// Foco apenas em características positivas de anúncios

// checkRequiredKeywords verifica palavras-chave obrigatórias
func (cbc *ContentBasedClassifier) checkRequiredKeywords(text string) []string {
	var found []string
	for _, keyword := range cbc.patterns.CommonKeywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			found = append(found, keyword)
		}
	}
	return found
}

// checkPriceElements verifica elementos de preço
func (cbc *ContentBasedClassifier) checkPriceElements(doc *goquery.Document, text string) (bool, float64) {
	score := 0.0

	// Verificar seletores de preço aprendidos
	for _, selector := range cbc.patterns.PriceSelectors {
		if doc.Find(selector).Length() > 0 {
			score += 5.0
			if score >= 15.0 {
				break
			}
		}
	}

	// Verificar padrões de preço no texto
	if score < 15.0 {
		for _, pattern := range cbc.patterns.PricePatterns {
			if pattern.MatchString(text) {
				score += 5.0
				break
			}
		}
	}

	// Verificar seletores genéricos de preço
	if score < 15.0 {
		genericPriceSelectors := []string{
			".price", ".valor", ".preco", ".value",
			"[class*='price']", "[class*='valor']",
		}

		for _, selector := range genericPriceSelectors {
			if doc.Find(selector).Length() > 0 {
				score += 2.0
				if score >= 15.0 {
					break
				}
			}
		}
	}

	if score > 20.0 {
		score = 20.0
	}

	return score > 0, score
}

// checkDetailElements verifica elementos de detalhes
func (cbc *ContentBasedClassifier) checkDetailElements(doc *goquery.Document, text string) (bool, float64) {
	score := 0.0

	// Verificar seletores de detalhes aprendidos
	for _, selector := range cbc.patterns.DetailsSelectors {
		if doc.Find(selector).Length() > 0 {
			score += 3.0
			if score >= 12.0 {
				break
			}
		}
	}

	// Verificar palavras-chave de detalhes no texto
	detailKeywords := []string{
		"quartos", "dormitórios", "suítes", "banheiros",
		"área", "m²", "m2", "metros", "garagem", "vagas",
	}

	for _, keyword := range detailKeywords {
		if strings.Contains(text, keyword) {
			score += 1.0
		}
	}

	// Verificar seletores genéricos de detalhes
	if score < 12.0 {
		genericDetailSelectors := []string{
			".details", ".detalhes", ".caracteristicas",
			".features", ".specs", ".info",
		}

		for _, selector := range genericDetailSelectors {
			if doc.Find(selector).Length() > 0 {
				score += 2.0
				if score >= 12.0 {
					break
				}
			}
		}
	}

	if score > 15.0 {
		score = 15.0
	}

	return score > 0, score
}

// checkCommonSelectors verifica seletores comuns aprendidos
func (cbc *ContentBasedClassifier) checkCommonSelectors(doc *goquery.Document) []string {
	var found []string
	for _, selector := range cbc.patterns.CommonSelectors {
		if doc.Find(selector).Length() > 0 {
			found = append(found, selector)
		}
	}
	return found
}

// checkRequiredElements verifica elementos obrigatórios
func (cbc *ContentBasedClassifier) checkRequiredElements(doc *goquery.Document) bool {
	for _, selector := range cbc.patterns.RequiredElements {
		if doc.Find(selector).Length() > 0 {
			return true
		}
	}
	return false
}

// checkPageStructure verifica estrutura geral da página
func (cbc *ContentBasedClassifier) checkPageStructure(doc *goquery.Document) float64 {
	score := 0.0

	// Verificar se tem título
	if doc.Find("h1, h2, h3").Length() > 0 {
		score += 3.0
	}

	// Verificar se tem imagens
	if doc.Find("img").Length() > 0 {
		score += 2.0
	}

	// Verificar se tem links
	if doc.Find("a").Length() > 0 {
		score += 1.0
	}

	// Verificar se tem formulários (contato)
	if doc.Find("form").Length() > 0 {
		score += 2.0
	}

	// Verificar se tem elementos de contato
	contactSelectors := []string{
		"[href*='tel:']", "[href*='mailto:']",
		"[href*='whatsapp']", "[href*='wa.me']",
	}

	for _, selector := range contactSelectors {
		if doc.Find(selector).Length() > 0 {
			score += 2.0
			break
		}
	}

	// Verificar meta tags
	if doc.Find("meta[property='og:title']").Length() > 0 {
		score += 1.0
	}

	// Verificar se tem breadcrumbs
	if doc.Find(".breadcrumb, .breadcrumbs, nav").Length() > 0 {
		score += 1.0
	}

	// Verificar divs com classes relacionadas a propriedades
	propertyClasses := []string{
		"[class*='property']", "[class*='imovel']",
		"[class*='listing']", "[class*='anuncio']",
	}

	for _, selector := range propertyClasses {
		if doc.Find(selector).Length() > 0 {
			score += 2.0
			break
		}
	}

	if score > 15.0 {
		score = 15.0
	}

	return score
}

// generateReason gera uma explicação da classificação
func (cbc *ContentBasedClassifier) generateReason(details ClassificationDetails, confidence float64, score, maxScore float64) string {
	var reasons []string

	reasons = append(reasons, fmt.Sprintf("Score: %.1f/%.1f (%.1f%%)", score, maxScore, confidence*100))

	if len(details.KeywordsFound) > 0 {
		reasons = append(reasons, fmt.Sprintf("%d palavras-chave encontradas", len(details.KeywordsFound)))
	}

	if details.HasPrice {
		reasons = append(reasons, "tem preço")
	}

	if details.HasDetails {
		reasons = append(reasons, "tem detalhes")
	}

	if details.HasRequiredElements {
		reasons = append(reasons, "tem elementos obrigatórios")
	}

	reasons = append(reasons, fmt.Sprintf("%d palavras", details.WordCount))

	return strings.Join(reasons, ", ")
}
