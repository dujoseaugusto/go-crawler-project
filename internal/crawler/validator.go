package crawler

import (
	"strings"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
)

// PropertyValidator valida se os dados extraídos são válidos
type PropertyValidator struct {
	logger *logger.Logger
}

// NewPropertyValidator cria um novo validador de propriedades
func NewPropertyValidator() *PropertyValidator {
	return &PropertyValidator{
		logger: logger.NewLogger("property_validator"),
	}
}

// ValidationResult representa o resultado da validação
type ValidationResult struct {
	IsValid  bool
	Errors   []string
	Warnings []string
}

// ValidateProperty valida uma propriedade extraída
func (pv *PropertyValidator) ValidateProperty(property *repository.Property) ValidationResult {
	result := ValidationResult{
		IsValid:  true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Validações obrigatórias
	if err := pv.validateRequired(property); err != nil {
		result.Errors = append(result.Errors, err...)
		result.IsValid = false
	}

	// Validações de qualidade dos dados
	if warnings := pv.validateQuality(property); len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	// Validações de dados suspeitos
	if suspicious := pv.validateSuspiciousData(property); len(suspicious) > 0 {
		result.Errors = append(result.Errors, suspicious...)
		result.IsValid = false
	}

	// Log do resultado
	pv.logger.WithFields(map[string]interface{}{
		"url":      property.URL,
		"is_valid": result.IsValid,
		"errors":   len(result.Errors),
		"warnings": len(result.Warnings),
		"endereco": property.Endereco,
		"valor":    property.Valor,
	}).Debug("Property validation completed")

	return result
}

// validateRequired valida campos obrigatórios
func (pv *PropertyValidator) validateRequired(property *repository.Property) []string {
	var errors []string

	// URL é sempre obrigatória
	if property.URL == "" {
		errors = append(errors, "URL é obrigatória")
	}

	// Pelo menos endereço OU valor deve estar presente
	if property.Endereco == "" && property.Valor <= 0 {
		errors = append(errors, "Endereço ou valor deve estar presente")
	}

	// Se tem valor, deve ser razoável
	if property.Valor > 0 && property.Valor < 1000 {
		errors = append(errors, "Valor muito baixo, provavelmente inválido")
	}

	if property.Valor > 100000000 { // 100 milhões
		errors = append(errors, "Valor muito alto, provavelmente inválido")
	}

	return errors
}

// validateQuality valida a qualidade dos dados
func (pv *PropertyValidator) validateQuality(property *repository.Property) []string {
	var warnings []string

	// Verifica se a descrição é muito curta
	if len(property.Descricao) > 0 && len(property.Descricao) < 20 {
		warnings = append(warnings, "Descrição muito curta")
	}

	// Verifica se o endereço é muito genérico
	if property.Endereco != "" && len(property.Endereco) < 10 {
		warnings = append(warnings, "Endereço muito genérico")
	}

	// Verifica se faltam informações importantes
	if property.Quartos == 0 && property.TipoImovel != "Terreno" {
		warnings = append(warnings, "Número de quartos não informado")
	}

	if property.AreaTotal == 0 {
		warnings = append(warnings, "Área total não informada")
	}

	if property.Cidade == "" {
		warnings = append(warnings, "Cidade não identificada")
	}

	return warnings
}

// validateSuspiciousData verifica dados suspeitos que indicam problemas
func (pv *PropertyValidator) validateSuspiciousData(property *repository.Property) []string {
	var errors []string

	// Verifica se o endereço contém muitas vírgulas (dados agregados)
	if strings.Count(property.Endereco, ",") > 5 {
		errors = append(errors, "Endereço suspeito de conter dados agregados")
	}

	// Verifica se o valor contém múltiplos valores
	if strings.Count(property.ValorTexto, "R$") > 2 {
		errors = append(errors, "Valor suspeito de conter múltiplos preços")
	}

	// Verifica frases que indicam catálogo ou dados agregados
	suspiciousPhrases := []string{
		"show room de negócios imobiliários",
		"gedaim - gerenciamento",
		"lista de imóveis",
		"catálogo de propriedades",
		"múltiplas opções",
	}

	descLower := strings.ToLower(property.Descricao)
	for _, phrase := range suspiciousPhrases {
		if strings.Contains(descLower, phrase) {
			errors = append(errors, "Descrição contém frases suspeitas de catálogo")
			break
		}
	}

	// Verifica se tem valores numéricos impossíveis
	if property.Quartos > 50 {
		errors = append(errors, "Número de quartos impossível")
	}

	if property.Banheiros > 20 {
		errors = append(errors, "Número de banheiros impossível")
	}

	if property.AreaTotal > 0 && property.AreaTotal < 10 && property.TipoImovel != "Terreno" {
		errors = append(errors, "Área muito pequena para o tipo de imóvel")
	}

	return errors
}

// PropertyCompleteness representa a completude de uma propriedade
type PropertyCompleteness struct {
	Score           float64 `json:"score"`
	MaxScore        float64 `json:"max_score"`
	Percentage      float64 `json:"percentage"`
	IsValidCatalog  bool    `json:"is_valid_catalog"`
	MissingFields   []string `json:"missing_fields"`
	WeightedScores  map[string]float64 `json:"weighted_scores"`
}

// CalculateCompleteness calcula a completude de uma propriedade com pesos
func (pv *PropertyValidator) CalculateCompleteness(property *repository.Property) PropertyCompleteness {
	// Definir pesos para cada campo (total = 100)
	weights := map[string]float64{
		"endereco":   25.0, // Peso maior para endereço
		"descricao":  20.0, // Peso maior para descrição  
		"cidade":     20.0, // Peso maior para cidade
		"valor":      15.0, // Peso maior para valor
		"quartos":    5.0,
		"banheiros":  3.0,
		"area_total": 4.0,
		"tipo_imovel": 3.0,
		"bairro":     2.0,
		"cep":        2.0,
		"caracteristicas": 1.0,
	}

	scores := make(map[string]float64)
	var missingFields []string
	totalScore := 0.0
	maxScore := 0.0

	// Calcular score para cada campo
	for field, weight := range weights {
		maxScore += weight
		fieldScore := pv.calculateFieldScore(property, field) * weight
		scores[field] = fieldScore
		totalScore += fieldScore
		
		if fieldScore == 0 {
			missingFields = append(missingFields, field)
		}
	}

	percentage := (totalScore / maxScore) * 100
	isValidCatalog := percentage >= 60.0 // 60% de completude mínima

	return PropertyCompleteness{
		Score:          totalScore,
		MaxScore:       maxScore,
		Percentage:     percentage,
		IsValidCatalog: isValidCatalog,
		MissingFields:  missingFields,
		WeightedScores: scores,
	}
}

// calculateFieldScore calcula o score de um campo específico (0.0 a 1.0)
func (pv *PropertyValidator) calculateFieldScore(property *repository.Property, field string) float64 {
	switch field {
	case "endereco":
		if property.Endereco == "" {
			return 0.0
		}
		// Score baseado no tamanho e qualidade do endereço
		if len(property.Endereco) < 10 {
			return 0.3 // Endereço muito curto
		}
		if len(property.Endereco) >= 20 {
			return 1.0 // Endereço completo
		}
		return 0.7 // Endereço médio
		
	case "descricao":
		if property.Descricao == "" {
			return 0.0
		}
		// Score baseado no tamanho da descrição
		if len(property.Descricao) < 20 {
			return 0.2
		}
		if len(property.Descricao) < 50 {
			return 0.5
		}
		if len(property.Descricao) >= 100 {
			return 1.0
		}
		return 0.8
		
	case "cidade":
		if property.Cidade == "" {
			return 0.0
		}
		return 1.0
		
	case "valor":
		if property.Valor <= 0 {
			return 0.0
		}
		// Valores muito baixos ou altos são suspeitos
		if property.Valor < 10000 || property.Valor > 50000000 {
			return 0.3
		}
		return 1.0
		
	case "quartos":
		if property.Quartos <= 0 {
			return 0.0
		}
		// Valores razoáveis de quartos
		if property.Quartos > 10 {
			return 0.3 // Suspeito
		}
		return 1.0
		
	case "banheiros":
		if property.Banheiros <= 0 {
			return 0.0
		}
		if property.Banheiros > 8 {
			return 0.3 // Suspeito
		}
		return 1.0
		
	case "area_total":
		if property.AreaTotal <= 0 {
			return 0.0
		}
		// Área muito pequena ou muito grande é suspeita
		if property.AreaTotal < 20 || property.AreaTotal > 10000 {
			return 0.3
		}
		return 1.0
		
	case "tipo_imovel":
		if property.TipoImovel == "" || property.TipoImovel == "Outro" {
			return 0.0
		}
		return 1.0
		
	case "bairro":
		if property.Bairro == "" {
			return 0.0
		}
		return 1.0
		
	case "cep":
		if property.CEP == "" {
			return 0.0
		}
		// Validar formato do CEP
		if len(property.CEP) == 8 || len(property.CEP) == 9 {
			return 1.0
		}
		return 0.5
		
	case "caracteristicas":
		if len(property.Caracteristicas) == 0 {
			return 0.0
		}
		if len(property.Caracteristicas) >= 3 {
			return 1.0
		}
		return 0.5
		
	default:
		return 0.0
	}
}

// IsValidForSaving verifica se a propriedade tem dados mínimos para ser salva
func (pv *PropertyValidator) IsValidForSaving(property *repository.Property) bool {
	// Usar o novo sistema de completude
	completeness := pv.CalculateCompleteness(property)
	
	// Verificar se não tem erros críticos
	result := pv.ValidateProperty(property)
	
	// Log da completude para debug
	pv.logger.WithFields(map[string]interface{}{
		"url":              property.URL,
		"completeness":     completeness.Percentage,
		"is_valid_catalog": completeness.IsValidCatalog,
		"missing_fields":   completeness.MissingFields,
		"has_errors":       !result.IsValid,
	}).Info("Property completeness calculated")

	return completeness.IsValidCatalog && result.IsValid
}

// EnhanceProperty melhora os dados da propriedade com base em validações
func (pv *PropertyValidator) EnhanceProperty(property *repository.Property) *repository.Property {
	enhanced := *property

	// Se não tem tipo definido, tenta inferir do endereço/descrição
	if enhanced.TipoImovel == "" || enhanced.TipoImovel == "Outro" {
		enhanced.TipoImovel = pv.inferPropertyType(enhanced.Endereco + " " + enhanced.Descricao)
	}

	// Se não tem cidade, tenta extrair do endereço
	if enhanced.Cidade == "" {
		enhanced.Cidade = pv.inferCity(enhanced.Endereco)
	}

	// Limpa e normaliza textos
	enhanced.Endereco = pv.cleanText(enhanced.Endereco)
	enhanced.Descricao = pv.cleanText(enhanced.Descricao)
	enhanced.Cidade = pv.cleanText(enhanced.Cidade)
	enhanced.Bairro = pv.cleanText(enhanced.Bairro)

	return &enhanced
}

// inferPropertyType tenta inferir o tipo de propriedade
func (pv *PropertyValidator) inferPropertyType(text string) string {
	textLower := strings.ToLower(text)

	if strings.Contains(textLower, "casa") || strings.Contains(textLower, "residência") {
		return "Casa"
	}
	if strings.Contains(textLower, "apartamento") || strings.Contains(textLower, "apto") {
		return "Apartamento"
	}
	if strings.Contains(textLower, "terreno") || strings.Contains(textLower, "lote") {
		return "Terreno"
	}
	if strings.Contains(textLower, "comercial") || strings.Contains(textLower, "loja") {
		return "Comercial"
	}

	return "Outro"
}

// inferCity tenta inferir a cidade do endereço
func (pv *PropertyValidator) inferCity(address string) string {
	cities := []string{
		"Muzambinho", "Alfenas", "Guaxupé", "Juruaia", "Monte Belo",
		"Poços de Caldas", "Varginha", "Três Pontas", "Machado",
	}

	addressUpper := strings.ToUpper(address)
	for _, city := range cities {
		if strings.Contains(addressUpper, strings.ToUpper(city)) {
			return city
		}
	}

	return ""
}

// cleanText limpa e normaliza texto
func (pv *PropertyValidator) cleanText(text string) string {
	if text == "" {
		return ""
	}

	// Remove espaços extras
	text = strings.Join(strings.Fields(text), " ")

	// Remove caracteres de controle
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	return strings.TrimSpace(text)
}
