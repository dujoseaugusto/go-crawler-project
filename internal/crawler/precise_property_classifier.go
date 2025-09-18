package crawler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
)

// PrecisePropertyClassifier classificador rigoroso para anúncios individuais específicos
type PrecisePropertyClassifier struct {
	logger           *logger.Logger
	pricePatterns    []*regexp.Regexp
	excludePatterns  []*regexp.Regexp
	requiredPatterns []*regexp.Regexp
}

// PreciseClassificationResult resultado da classificação precisa
type PreciseClassificationResult struct {
	IsIndividualProperty bool           `json:"is_individual_property"`
	Confidence           float64        `json:"confidence"`
	Reason               string         `json:"reason"`
	Score                float64        `json:"score"`
	MaxScore             float64        `json:"max_score"`
	Details              PreciseDetails `json:"details"`
}

// PreciseDetails detalhes da classificação precisa
type PreciseDetails struct {
	HasSpecificPrice   bool     `json:"has_specific_price"`
	HasPropertyDetails bool     `json:"has_property_details"`
	HasContactInfo     bool     `json:"has_contact_info"`
	HasSpecificAddress bool     `json:"has_specific_address"`
	WordCount          int      `json:"word_count"`
	IsGenericPage      bool     `json:"is_generic_page"`
	IsInstitutional    bool     `json:"is_institutional"`
	FoundExclusions    []string `json:"found_exclusions"`
	FoundRequirements  []string `json:"found_requirements"`
}

// NewPrecisePropertyClassifier cria um classificador rigoroso
func NewPrecisePropertyClassifier() *PrecisePropertyClassifier {
	classifier := &PrecisePropertyClassifier{
		logger: logger.NewLogger("precise_property_classifier"),
	}

	// Padrões de preço específicos (EXPANDIDOS)
	classifier.pricePatterns = []*regexp.Regexp{
		regexp.MustCompile(`R\$\s*[\d]{1,3}(?:\.[\d]{3})*(?:,[\d]{2})?`),   // R$ 150.000,00
		regexp.MustCompile(`[\d]{1,3}(?:\.[\d]{3})*(?:,[\d]{2})?\s*reais`), // 150.000,00 reais
		regexp.MustCompile(`Valor:?\s*R\$\s*[\d]{1,3}(?:\.[\d]{3})*`),      // Valor: R$ 150.000
		regexp.MustCompile(`Preço:?\s*R\$\s*[\d]{1,3}(?:\.[\d]{3})*`),      // Preço: R$ 150.000
		regexp.MustCompile(`R\$\s*[\d]+\.?[\d]*`),                          // R$ 150000 ou R$ 150.000
		regexp.MustCompile(`[\d]+\s*mil`),                                  // 150 mil
		regexp.MustCompile(`(?i)por\s*R\$\s*[\d]+`),                        // por R$ 150000
		regexp.MustCompile(`(?i)valor\s*[\d]+`),                            // valor 150000
	}

	// Padrões que EXCLUEM a página (indicam que NÃO é anúncio individual)
	classifier.excludePatterns = []*regexp.Regexp{
		// Páginas de categoria/listagem
		regexp.MustCompile(`(?i)diversos\s+im[óo]veis`),
		regexp.MustCompile(`(?i)mais\s+de\s+\d+\s+im[óo]veis`),
		regexp.MustCompile(`(?i)encontra\s+.*\s+im[óo]veis`),
		regexp.MustCompile(`(?i)escolha\s+o\s+seu\s+im[óo]vel`),
		regexp.MustCompile(`(?i)todos\s+os\s+bairros`),
		regexp.MustCompile(`(?i)todas\s+as\s+regi[õo]es`),

		// Páginas institucionais (MUITO REDUZIDAS - só títulos principais)
		// REMOVIDO: nossa equipe (aparece em rodapés)
		// REMOVIDO: sobre nós (aparece em rodapés)
		// REMOVIDO: política de privacidade (aparece em rodapés)
		// REMOVIDO: termos de uso (aparece em rodapés)
		// REMOVIDO: cadastre-se (muito restritivo - aparece em anúncios legítimos)
		// REMOVIDO: anuncie seu imóvel (pode aparecer em páginas de anúncios)
		// REMOVIDO: faça login (pode aparecer em páginas de anúncios)

		// Páginas de navegação/filtros (MUITO REDUZIDAS)
		regexp.MustCompile(`(?i)p[áa]gina\s+(anterior|pr[óo]xima)`),
		regexp.MustCompile(`(?i)nenhum\s+im[óo]vel\s+encontrado`),
		// REMOVIDO: ordenar por (muito restritivo)
		// REMOVIDO: filtros de busca (pode aparecer em anúncios)
		// REMOVIDO: resultados da busca (pode aparecer em anúncios)

		// Textos genéricos de categoria (MUITO REDUZIDOS)
		// REMOVIDO: apartamentos casa terreno (muito comum em sites legítimos)
		// REMOVIDO: comprar apartamentos casa (pode aparecer em anúncios)
		// REMOVIDO: alugar apartamentos casa (pode aparecer em anúncios)

		// EXCLUSÕES DE PÁGINAS DE CATÁLOGO - CRÍTICO!
		regexp.MustCompile(`(?i)todos\s+os\s+bairros`),
		regexp.MustCompile(`(?i)diversos\s+\w+\s+à\s+venda`),
		regexp.MustCompile(`(?i)diversos\s+\w+\s+para\s+alugar`),
		regexp.MustCompile(`(?i)você\s+encontra\s+diversos`),
		regexp.MustCompile(`(?i)escolha\s+o\s+seu\s+im[óo]vel`),
		regexp.MustCompile(`(?i)encontra\s+as\s+melhores`),
		regexp.MustCompile(`(?i)diversas\s+op[çc][õo]es`),
		
		// NOVAS EXCLUSÕES BASEADAS NO EXEMPLO PROBLEMÁTICO
		regexp.MustCompile(`(?i)\d+\s+im[óo]veis\s+ao\s+buscar`), // "8 imóveis ao buscar"
		regexp.MustCompile(`(?i)as\s+melhores\s+op[çc][õo]es\s+de\s+im[óo]veis`), // "As melhores opções de imóveis"
		regexp.MustCompile(`(?i)im[óo]veis\s+à\s+venda\s+em\s+\w+\s+em\s+\w+`), // "Casas à Venda em Muzambinho em Minas"
		regexp.MustCompile(`(?i)\w+\s+à\s+venda\s+em\s+\w+\s+-\s+\w+`), // "Casas à Venda em Muzambinho - MG"
	}

	// Padrões que REQUEREM para ser anúncio individual
	classifier.requiredPatterns = []*regexp.Regexp{
		// Deve ter pelo menos um destes indicadores de anúncio específico
		regexp.MustCompile(`(?i)\d+\s+quartos?`),                  // 3 quartos
		regexp.MustCompile(`(?i)\d+\s+dormit[óo]rios?`),           // 2 dormitórios
		regexp.MustCompile(`(?i)\d+\s+banheiros?`),                // 2 banheiros
		regexp.MustCompile(`(?i)\d+\s+su[íi]tes?`),                // 1 suíte
		regexp.MustCompile(`(?i)\d+\s+vagas?`),                    // 2 vagas
		regexp.MustCompile(`(?i)\d+\s*m[²2]`),                     // 80m²
		regexp.MustCompile(`(?i)[áa]rea\s+de\s+\d+`),              // área de 80
		regexp.MustCompile(`(?i)rua\s+[a-zA-ZÀ-ÿ\s]+,\s*\d+`),     // Rua das Flores, 123
		regexp.MustCompile(`(?i)avenida\s+[a-zA-ZÀ-ÿ\s]+,\s*\d+`), // Avenida Brasil, 456
		regexp.MustCompile(`(?i)c[óo]digo:?\s*[a-zA-Z0-9]+`),      // Código: ABC123
		regexp.MustCompile(`(?i)ref:?\s*[a-zA-Z0-9]+`),            // Ref: 12345
	}

	return classifier
}

// ClassifyPage classifica uma página com rigor para anúncios individuais
func (ppc *PrecisePropertyClassifier) ClassifyPage(doc *goquery.Document, url string) PreciseClassificationResult {
	if doc == nil {
		return PreciseClassificationResult{
			IsIndividualProperty: false,
			Confidence:           0.0,
			Reason:               "Documento HTML inválido",
		}
	}

	result := PreciseClassificationResult{
		Details: PreciseDetails{
			FoundExclusions:   []string{},
			FoundRequirements: []string{},
		},
	}

	// Extrair texto da página
	pageText := strings.ToLower(doc.Text())
	words := strings.Fields(pageText)
	result.Details.WordCount = len(words)
	
	// VALIDAÇÃO ADICIONAL: URLs de catálogo/listagem
	urlLower := strings.ToLower(url)
	catalogURLPatterns := []string{
		"/venda/casa/mg-", "/venda/apartamento/mg-", "/venda/terreno/mg-",
		"/aluguel/casa/mg-", "/aluguel/apartamento/mg-", "/aluguel/terreno/mg-",
		"/buscar/imoveis/", "/imoveis/venda/", "/imoveis/aluguel/",
	}
	
	for _, pattern := range catalogURLPatterns {
		if strings.Contains(urlLower, pattern) {
			result.Details.IsGenericPage = true
			result.Details.FoundExclusions = append(result.Details.FoundExclusions, "URL de catálogo: "+pattern)
			break
		}
	}

	score := 0.0
	maxScore := 100.0

	// 1. VERIFICAR EXCLUSÕES (se encontrar, rejeitar imediatamente)
	for _, pattern := range ppc.excludePatterns {
		if matches := pattern.FindAllString(pageText, -1); len(matches) > 0 {
			result.Details.FoundExclusions = append(result.Details.FoundExclusions, matches...)
			result.Details.IsGenericPage = true
		}
	}

	// Se tem exclusões, rejeitar
	if len(result.Details.FoundExclusions) > 0 {
		result.IsIndividualProperty = false
		result.Confidence = 0.0
		result.Reason = fmt.Sprintf("Página genérica/institucional detectada: %v", result.Details.FoundExclusions)
		result.Score = 0.0
		result.MaxScore = maxScore
		return result
	}

	// 2. VERIFICAR REQUISITOS (deve ter pelo menos um)
	for _, pattern := range ppc.requiredPatterns {
		if matches := pattern.FindAllString(pageText, -1); len(matches) > 0 {
			result.Details.FoundRequirements = append(result.Details.FoundRequirements, matches...)
			score += 20.0 // Cada requisito vale 20 pontos
		}
	}

	// Deve ter pelo menos um requisito
	if len(result.Details.FoundRequirements) == 0 {
		result.IsIndividualProperty = false
		result.Confidence = 0.0
		result.Reason = "Não possui características de anúncio individual específico"
		result.Score = 0.0
		result.MaxScore = maxScore
		return result
	}

	// 3. VERIFICAR PREÇO ESPECÍFICO (não genérico)
	hasSpecificPrice := false
	for _, pattern := range ppc.pricePatterns {
		if pattern.MatchString(pageText) {
			// Verificar se não é preço genérico (como R$ 35.000 que aparece em tudo)
			priceMatches := pattern.FindAllString(pageText, -1)
			for _, priceMatch := range priceMatches {
				if !strings.Contains(priceMatch, "35.000") && !strings.Contains(priceMatch, "35000") {
					hasSpecificPrice = true
					score += 25.0
					break
				}
			}
		}
	}
	result.Details.HasSpecificPrice = hasSpecificPrice

	// 4. VERIFICAR DETALHES ESPECÍFICOS DA PROPRIEDADE
	hasPropertyDetails := ppc.checkPropertyDetails(doc, pageText)
	result.Details.HasPropertyDetails = hasPropertyDetails
	if hasPropertyDetails {
		score += 20.0
	}

	// 5. VERIFICAR INFORMAÇÕES DE CONTATO ESPECÍFICAS
	hasContactInfo := ppc.checkContactInfo(doc, pageText)
	result.Details.HasContactInfo = hasContactInfo
	if hasContactInfo {
		score += 15.0
	}

	// 6. VERIFICAR ENDEREÇO ESPECÍFICO
	hasSpecificAddress := ppc.checkSpecificAddress(pageText)
	result.Details.HasSpecificAddress = hasSpecificAddress
	if hasSpecificAddress {
		score += 20.0
	}

	// 7. VERIFICAR CONTEÚDO MÍNIMO E MÁXIMO
	if result.Details.WordCount < 50 {
		score -= 20.0 // Penalizar conteúdo muito pequeno
	} else if result.Details.WordCount > 2000 {
		score -= 10.0 // Penalizar conteúdo muito grande (pode ser listagem)
	}

	// Calcular confiança final
	confidence := score / maxScore
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	// CRITÉRIO AJUSTADO: Aceita anúncios com score alto mesmo sem preço específico
	isIndividualProperty := confidence >= 0.6 &&
		len(result.Details.FoundRequirements) >= 3 &&
		(hasSpecificPrice || (hasPropertyDetails && hasSpecificAddress)) &&
		!result.Details.IsGenericPage

	// Gerar razão detalhada
	reason := ppc.generateDetailedReason(result.Details, confidence, score, maxScore)

	// DEBUG: Adicionar informações sobre por que foi rejeitado
	if !isIndividualProperty {
		debugInfo := fmt.Sprintf(" [DEBUG: conf=%.2f>=0.6? req=%d>=3? price=%v details=%v addr=%v generic=%v]",
			confidence, len(result.Details.FoundRequirements), hasSpecificPrice, hasPropertyDetails, hasSpecificAddress, result.Details.IsGenericPage)
		reason += debugInfo
	}

	result.IsIndividualProperty = isIndividualProperty
	result.Confidence = confidence
	result.Reason = reason
	result.Score = score
	result.MaxScore = maxScore

	ppc.logger.WithFields(map[string]interface{}{
		"url":                    url,
		"is_individual_property": isIndividualProperty,
		"confidence":             confidence,
		"score":                  fmt.Sprintf("%.1f/%.1f", score, maxScore),
		"requirements_found":     len(result.Details.FoundRequirements),
		"exclusions_found":       len(result.Details.FoundExclusions),
		"has_specific_price":     hasSpecificPrice,
		"has_property_details":   hasPropertyDetails,
		"has_specific_address":   hasSpecificAddress,
	}).Debug("Precise classification completed")

	return result
}

// checkPropertyDetails verifica detalhes específicos da propriedade
func (ppc *PrecisePropertyClassifier) checkPropertyDetails(doc *goquery.Document, text string) bool {
	// Procurar por elementos HTML específicos de detalhes
	detailSelectors := []string{
		".property-details", ".imovel-detalhes", ".caracteristicas",
		".features", ".specs", ".informacoes",
	}

	for _, selector := range detailSelectors {
		if doc.Find(selector).Length() > 0 {
			return true
		}
	}

	// Procurar por padrões de detalhes no texto
	detailPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)condom[íi]nio:?\s*R\$\s*[\d.,]+`),
		regexp.MustCompile(`(?i)iptu:?\s*R\$\s*[\d.,]+`),
		regexp.MustCompile(`(?i)piscina`),
		regexp.MustCompile(`(?i)churrasqueira`),
		regexp.MustCompile(`(?i)jardim`),
		regexp.MustCompile(`(?i)sacada`),
		regexp.MustCompile(`(?i)varanda`),
		regexp.MustCompile(`(?i)elevador`),
		regexp.MustCompile(`(?i)portaria\s+24h`),
	}

	detailCount := 0
	for _, pattern := range detailPatterns {
		if pattern.MatchString(text) {
			detailCount++
		}
	}

	return detailCount >= 2 // Pelo menos 2 características específicas
}

// checkContactInfo verifica informações de contato específicas
func (ppc *PrecisePropertyClassifier) checkContactInfo(doc *goquery.Document, text string) bool {
	// Procurar por telefones específicos (não genéricos)
	phonePattern := regexp.MustCompile(`\(\d{2}\)\s*\d{4,5}-?\d{4}`)
	phones := phonePattern.FindAllString(text, -1)

	// Procurar por WhatsApp específico
	whatsappPattern := regexp.MustCompile(`(?i)whatsapp:?\s*\(\d{2}\)\s*\d{4,5}-?\d{4}`)
	hasWhatsapp := whatsappPattern.MatchString(text)

	// Procurar por email específico do corretor
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	emails := emailPattern.FindAllString(text, -1)

	return len(phones) > 0 || hasWhatsapp || len(emails) > 0
}

// checkSpecificAddress verifica se tem endereço específico
func (ppc *PrecisePropertyClassifier) checkSpecificAddress(text string) bool {
	// Padrões de endereço específico (EXPANDIDOS)
	addressPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)rua\s+[a-zA-ZÀ-ÿ\s]+,\s*\d+`),
		regexp.MustCompile(`(?i)avenida\s+[a-zA-ZÀ-ÿ\s]+,\s*\d+`),
		regexp.MustCompile(`(?i)pra[çc]a\s+[a-zA-ZÀ-ÿ\s]+,\s*\d+`),
		regexp.MustCompile(`(?i)alameda\s+[a-zA-ZÀ-ÿ\s]+,\s*\d+`),
		regexp.MustCompile(`(?i)cep:?\s*\d{5}-?\d{3}`),
		regexp.MustCompile(`(?i)bairro:?\s+[a-zA-ZÀ-ÿ\s]{3,}`),            // Bairro: Centro
		regexp.MustCompile(`(?i)cidade:?\s+[a-zA-ZÀ-ÿ\s]{3,}`),            // Cidade: Muzambinho
		regexp.MustCompile(`(?i)endere[çc]o:?\s+[a-zA-ZÀ-ÿ\s,\d]+`),       // Endereço: Rua X
		regexp.MustCompile(`(?i)[a-zA-ZÀ-ÿ\s]+-\s*MG`),                    // Muzambinho - MG
		regexp.MustCompile(`(?i)localiza[çc][ãa]o:?\s+[a-zA-ZÀ-ÿ\s]{3,}`), // Localização: Centro
	}

	for _, pattern := range addressPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}

	return false
}

// generateDetailedReason gera uma explicação detalhada da classificação
func (ppc *PrecisePropertyClassifier) generateDetailedReason(details PreciseDetails, confidence float64, score, maxScore float64) string {
	var reasons []string

	reasons = append(reasons, fmt.Sprintf("Score: %.1f/%.1f (%.1f%%)", score, maxScore, confidence*100))

	if len(details.FoundExclusions) > 0 {
		return fmt.Sprintf("REJEITADO - Página genérica/institucional: %v", details.FoundExclusions)
	}

	if len(details.FoundRequirements) == 0 {
		return "REJEITADO - Sem características de anúncio individual"
	}

	reasons = append(reasons, fmt.Sprintf("%d requisitos encontrados", len(details.FoundRequirements)))

	if details.HasSpecificPrice {
		reasons = append(reasons, "preço específico")
	}

	if details.HasPropertyDetails {
		reasons = append(reasons, "detalhes da propriedade")
	}

	if details.HasContactInfo {
		reasons = append(reasons, "contato específico")
	}

	if details.HasSpecificAddress {
		reasons = append(reasons, "endereço específico")
	}

	reasons = append(reasons, fmt.Sprintf("%d palavras", details.WordCount))

	return strings.Join(reasons, ", ")
}
