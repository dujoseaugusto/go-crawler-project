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

// IsValidForSaving verifica se a propriedade tem dados mínimos para ser salva
func (pv *PropertyValidator) IsValidForSaving(property *repository.Property) bool {
	// Critérios mínimos para salvar
	hasBasicInfo := property.URL != "" &&
		(property.Endereco != "" || property.Valor > 1000)

	// Não deve ter erros críticos
	result := pv.ValidateProperty(property)

	return hasBasicInfo && result.IsValid
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
