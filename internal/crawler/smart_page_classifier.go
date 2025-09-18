package crawler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// SmartPageClassifier usa padrões aprendidos para classificar páginas com alta precisão
type SmartPageClassifier struct {
	urlTrainer       *ReferenceURLTrainer
	contentValidator *ContentValidator
}

// ContentValidator valida se o conteúdo da página é realmente um anúncio
type ContentValidator struct {
	pricePatterns      []*regexp.Regexp
	propertyIndicators []string
	excludeIndicators  []string
}

// NewSmartPageClassifier cria um novo classificador inteligente
func NewSmartPageClassifier(referenceURLs []string) *SmartPageClassifier {
	return &SmartPageClassifier{
		urlTrainer:       NewReferenceURLTrainer(referenceURLs),
		contentValidator: NewContentValidator(),
	}
}

// NewContentValidator cria um novo validador de conteúdo
func NewContentValidator() *ContentValidator {
	// Padrões para identificar preços
	pricePatterns := []*regexp.Regexp{
		regexp.MustCompile(`R\$\s*[\d.,]+`),
		regexp.MustCompile(`\$\s*[\d.,]+`),
		regexp.MustCompile(`[\d.,]+\s*reais?`),
		regexp.MustCompile(`Valor:?\s*R\$\s*[\d.,]+`),
		regexp.MustCompile(`Preço:?\s*R\$\s*[\d.,]+`),
	}

	return &ContentValidator{
		pricePatterns: pricePatterns,
		// Indicadores de que é um anúncio específico
		propertyIndicators: []string{
			"quartos", "dormitórios", "suítes", "banheiros",
			"área construída", "área total", "área útil",
			"garagem", "vagas", "estacionamento",
			"condomínio", "iptu", "escritura",
			"financiamento", "entrada", "parcelas",
			"código do imóvel", "ref:", "código:",
			"localização", "endereço", "bairro",
			"aceita financiamento", "documentação em ordem",
		},
		// Indicadores de que NÃO é um anúncio específico
		excludeIndicators: []string{
			"todos os bairros", "todas as regiões",
			"diversos imóveis", "várias opções",
			"encontre seu imóvel", "busque por",
			"filtros de busca", "ordenar por",
			"página anterior", "próxima página",
			"resultados encontrados", "nenhum resultado",
			"nossa equipe", "sobre nós", "contato",
			"política de privacidade", "termos de uso",
		},
	}
}

// ClassifyPageFromSelection classifica uma página usando URL e Selection do Colly
func (s *SmartPageClassifier) ClassifyPageFromSelection(url string, selection *goquery.Selection) (bool, float64, string) {
	// Criar um documento temporário a partir da selection
	doc := &goquery.Document{Selection: selection}
	return s.ClassifyPage(url, doc)
}

// ClassifyPage classifica uma página usando URL e conteúdo
func (s *SmartPageClassifier) ClassifyPage(url string, doc *goquery.Document) (bool, float64, string) {
	// 1. Classificação por URL
	isPropertyURL, urlConfidence, urlReason := s.urlTrainer.IsPropertyURL(url)

	if !isPropertyURL {
		return false, urlConfidence, fmt.Sprintf("URL rejeitada: %s", urlReason)
	}

	// 2. Validação por conteúdo
	isPropertyContent, contentConfidence, contentReason := s.contentValidator.ValidateContent(doc)

	// 3. Combinar as duas análises
	finalConfidence := (urlConfidence + contentConfidence) / 2.0

	// Só aceitar se ambas as validações passaram E confiança alta
	isProperty := isPropertyURL && isPropertyContent && finalConfidence >= 0.8 && urlConfidence >= 0.7

	reason := fmt.Sprintf("URL: %s (%.2f) | Conteúdo: %s (%.2f) | Final: %.2f",
		urlReason, urlConfidence, contentReason, contentConfidence, finalConfidence)

	return isProperty, finalConfidence, reason
}

// ValidateContent valida se o conteúdo da página é um anúncio específico
func (c *ContentValidator) ValidateContent(doc *goquery.Document) (bool, float64, string) {
	if doc == nil {
		return false, 0.0, "Documento HTML inválido"
	}

	// Extrair todo o texto da página
	pageText := strings.ToLower(doc.Text())

	// 1. Verificar indicadores de exclusão (páginas que NÃO são anúncios)
	for _, indicator := range c.excludeIndicators {
		if strings.Contains(pageText, strings.ToLower(indicator)) {
			return false, 0.0, fmt.Sprintf("Contém indicador de exclusão: %s", indicator)
		}
	}

	// 2. Contar indicadores positivos
	positiveScore := 0.0
	foundIndicators := []string{}

	for _, indicator := range c.propertyIndicators {
		if strings.Contains(pageText, strings.ToLower(indicator)) {
			positiveScore += 1.0
			foundIndicators = append(foundIndicators, indicator)
		}
	}

	// 3. Verificar se tem preço
	hasPrice := false
	for _, pattern := range c.pricePatterns {
		if pattern.MatchString(pageText) {
			hasPrice = true
			positiveScore += 2.0 // Preço é um indicador forte
			break
		}
	}

	// 4. Verificar estrutura HTML específica de anúncios
	structureScore := c.analyzeHTMLStructure(doc)
	positiveScore += structureScore

	// 5. Calcular confiança final
	maxScore := 10.0 // Score máximo possível
	confidence := positiveScore / maxScore
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Critérios mínimos para ser considerado anúncio
	isProperty := confidence >= 0.4 && (hasPrice || len(foundIndicators) >= 3)

	reason := fmt.Sprintf("Score: %.1f/%.1f, Preço: %v, Indicadores: %v",
		positiveScore, maxScore, hasPrice, foundIndicators)

	return isProperty, confidence, reason
}

// analyzeHTMLStructure analisa a estrutura HTML para identificar padrões de anúncios
func (c *ContentValidator) analyzeHTMLStructure(doc *goquery.Document) float64 {
	score := 0.0

	// Verificar elementos típicos de anúncios
	if doc.Find(".price, .valor, .preco").Length() > 0 {
		score += 1.0
	}

	if doc.Find(".details, .detalhes, .caracteristicas").Length() > 0 {
		score += 1.0
	}

	if doc.Find(".gallery, .galeria, .fotos, .images").Length() > 0 {
		score += 1.0
	}

	if doc.Find(".contact, .contato, .telefone, .whatsapp").Length() > 0 {
		score += 0.5
	}

	// Verificar se tem formulário de contato específico
	if doc.Find("form").Length() > 0 {
		formText := strings.ToLower(doc.Find("form").Text())
		if strings.Contains(formText, "interesse") ||
			strings.Contains(formText, "contato") ||
			strings.Contains(formText, "visita") {
			score += 1.0
		}
	}

	// Verificar meta tags específicas
	if doc.Find("meta[property='og:type'][content*='property']").Length() > 0 {
		score += 2.0
	}

	return score
}

// GetTrainer retorna o treinador de URLs para análise
func (s *SmartPageClassifier) GetTrainer() *ReferenceURLTrainer {
	return s.urlTrainer
}

// PrintAnalysis imprime análise completa do classificador
func (s *SmartPageClassifier) PrintAnalysis() {
	fmt.Println("=== Análise do Classificador Inteligente ===")
	s.urlTrainer.PrintAnalysis()

	fmt.Println("=== Validador de Conteúdo ===")
	fmt.Printf("Padrões de preço: %d\n", len(s.contentValidator.pricePatterns))
	fmt.Printf("Indicadores positivos: %d\n", len(s.contentValidator.propertyIndicators))
	fmt.Printf("Indicadores de exclusão: %d\n", len(s.contentValidator.excludeIndicators))
}
