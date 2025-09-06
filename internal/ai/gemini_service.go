package ai

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// CacheEntry representa uma entrada no cache
type CacheEntry struct {
	Property  repository.Property
	Timestamp time.Time
}

// PropertyCache é um cache em memória para propriedades processadas
type PropertyCache struct {
	cache map[string]CacheEntry
	mutex sync.RWMutex
	ttl   time.Duration
}

// GeminiService é responsável por processar dados usando a API do Gemini
type GeminiService struct {
	model       *genai.GenerativeModel
	cache       *PropertyCache
	batchSize   int
	batchBuffer []repository.Property
	bufferMutex sync.Mutex
}

// NewGeminiService cria uma nova instância do serviço Gemini
func NewGeminiService(ctx context.Context) (*GeminiService, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY não encontrada no ambiente")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %v", err)
	}

	// Usando o modelo Gemini 1.5 Flash para melhor desempenho
	model := client.GenerativeModel("gemini-1.5-flash")

	// Configurações otimizadas para menor consumo
	model.SetTemperature(0.0)     // Máxima determinismo
	model.SetTopK(1)              // Apenas a melhor opção
	model.SetTopP(0.9)            // Alta probabilidade
	model.SetMaxOutputTokens(512) // Reduzido para economizar tokens

	// Inicializar cache
	cache := &PropertyCache{
		cache: make(map[string]CacheEntry),
		ttl:   24 * time.Hour, // Cache por 24 horas
	}

	return &GeminiService{
		model:       model,
		cache:       cache,
		batchSize:   5, // Processar 5 propriedades por vez
		batchBuffer: make([]repository.Property, 0, 5),
	}, nil
}

// generateCacheKey gera uma chave de cache baseada no conteúdo da propriedade
func (s *GeminiService) generateCacheKey(property repository.Property) string {
	// Cria hash baseado nos campos principais que afetam o processamento
	content := fmt.Sprintf("%s|%s|%s|%s",
		property.Endereco, property.Descricao, property.ValorTexto, property.URL)
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// getFromCache recupera uma propriedade do cache se existir e não estiver expirada
func (s *GeminiService) getFromCache(key string) (repository.Property, bool) {
	s.cache.mutex.RLock()
	defer s.cache.mutex.RUnlock()

	entry, exists := s.cache.cache[key]
	if !exists {
		return repository.Property{}, false
	}

	// Verifica se não expirou
	if time.Since(entry.Timestamp) > s.cache.ttl {
		return repository.Property{}, false
	}

	return entry.Property, true
}

// setCache armazena uma propriedade no cache
func (s *GeminiService) setCache(key string, property repository.Property) {
	s.cache.mutex.Lock()
	defer s.cache.mutex.Unlock()

	s.cache.cache[key] = CacheEntry{
		Property:  property,
		Timestamp: time.Now(),
	}
}

// needsAIProcessing verifica se a propriedade precisa de processamento IA
func (s *GeminiService) needsAIProcessing(property repository.Property) bool {
	// Se já tem dados básicos bem estruturados, pode não precisar de IA
	hasBasicInfo := property.Endereco != "" && property.Valor > 0 && property.TipoImovel != ""
	hasDetailedInfo := property.Quartos > 0 && property.Banheiros > 0 && property.AreaTotal > 0

	// Só usa IA se faltam informações importantes ou se os dados parecem sujos
	return !hasBasicInfo || !hasDetailedInfo ||
		strings.Contains(property.Endereco, "\n") || // Endereço com quebras de linha
		len(property.Descricao) > 1000 || // Descrição muito longa
		property.TipoImovel == "Outro" // Tipo não identificado
}

// ProcessPropertyData processa os dados brutos de uma propriedade usando o Gemini
func (s *GeminiService) ProcessPropertyData(ctx context.Context, rawData repository.Property) (repository.Property, error) {
	// Verifica se precisa de processamento IA
	if !s.needsAIProcessing(rawData) {
		return rawData, nil
	}

	// Gera chave de cache
	cacheKey := s.generateCacheKey(rawData)

	// Verifica cache primeiro
	if cached, found := s.getFromCache(cacheKey); found {
		return cached, nil
	}

	// Adiciona ao buffer para processamento em lote
	return s.addToBatch(ctx, rawData, cacheKey)
}

// addToBatch adiciona propriedade ao buffer de processamento em lote
func (s *GeminiService) addToBatch(ctx context.Context, property repository.Property, cacheKey string) (repository.Property, error) {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	// Adiciona ao buffer
	s.batchBuffer = append(s.batchBuffer, property)

	// Se o buffer está cheio, processa o lote
	if len(s.batchBuffer) >= s.batchSize {
		return s.processBatch(ctx, cacheKey)
	}

	// Se não está cheio, processa individualmente (fallback)
	return s.processSingle(ctx, property, cacheKey)
}

// processBatch processa um lote de propriedades
func (s *GeminiService) processBatch(ctx context.Context, targetCacheKey string) (repository.Property, error) {
	if len(s.batchBuffer) == 0 {
		return repository.Property{}, fmt.Errorf("buffer vazio")
	}

	// Cria prompt para lote
	prompt := createBatchPrompt(s.batchBuffer)

	// Chama API do Gemini
	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		// Em caso de erro, processa individualmente
		return s.processSingle(ctx, s.batchBuffer[len(s.batchBuffer)-1], targetCacheKey)
	}

	// Processa resposta do lote
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return s.processSingle(ctx, s.batchBuffer[len(s.batchBuffer)-1], targetCacheKey)
	}

	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)
	processedProperties, err := parseBatchResponse(string(responseText), s.batchBuffer)
	if err != nil {
		return s.processSingle(ctx, s.batchBuffer[len(s.batchBuffer)-1], targetCacheKey)
	}

	// Armazena todas no cache
	var targetProperty repository.Property
	for i, processed := range processedProperties {
		if i < len(s.batchBuffer) {
			key := s.generateCacheKey(s.batchBuffer[i])
			s.setCache(key, processed)

			if key == targetCacheKey {
				targetProperty = processed
			}
		}
	}

	// Limpa o buffer
	s.batchBuffer = s.batchBuffer[:0]

	return targetProperty, nil
}

// processSingle processa uma propriedade individualmente (fallback)
func (s *GeminiService) processSingle(ctx context.Context, property repository.Property, cacheKey string) (repository.Property, error) {
	prompt := createOptimizedPrompt(property)

	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return property, fmt.Errorf("erro ao gerar conteúdo com Gemini: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return property, fmt.Errorf("resposta vazia do Gemini")
	}

	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)
	processed, err := parseGeminiResponse(string(responseText), property)
	if err != nil {
		return property, err
	}

	// Armazena no cache
	s.setCache(cacheKey, processed)

	return processed, nil
}

// createOptimizedPrompt cria um prompt otimizado para menor consumo de tokens
func createOptimizedPrompt(property repository.Property) string {
	return fmt.Sprintf(`Extraia dados estruturados do imóvel em JSON:
Dados: Endereço:%s | Descrição:%s | Valor:%s

JSON com: endereco, cidade, bairro, cep, descricao, valor, quartos, banheiros, area_total, area_util, tipo_imovel, caracteristicas`,
		truncateString(property.Endereco, 100),
		truncateString(property.Descricao, 200),
		truncateString(property.ValorTexto, 50))
}

// createBatchPrompt cria prompt para processamento em lote
func createBatchPrompt(properties []repository.Property) string {
	prompt := "Extraia dados de múltiplos imóveis em JSON array:\n"

	for i, prop := range properties {
		if i >= 5 { // Limita a 5 para não exceder tokens
			break
		}
		prompt += fmt.Sprintf("Imóvel %d: %s | %s | %s\n",
			i+1,
			truncateString(prop.Endereco, 80),
			truncateString(prop.Descricao, 150),
			truncateString(prop.ValorTexto, 30))
	}

	prompt += "\nRetorne array JSON com: endereco, cidade, bairro, valor, quartos, banheiros, area_total, tipo_imovel"
	return prompt
}

// truncateString trunca string para economizar tokens
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// parseBatchResponse processa resposta em lote do Gemini
func parseBatchResponse(response string, originalProperties []repository.Property) ([]repository.Property, error) {
	// Extrai array JSON da resposta
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("não foi possível extrair JSON da resposta em lote")
	}

	// Tenta fazer parse como array
	var batchResponse []map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &batchResponse)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer parse da resposta em lote: %v", err)
	}

	var results []repository.Property
	for i, item := range batchResponse {
		if i >= len(originalProperties) {
			break
		}

		original := originalProperties[i]
		processed := updatePropertyFromMap(original, item)
		results = append(results, processed)
	}

	return results, nil
}

// updatePropertyFromMap atualiza propriedade com dados do map
func updatePropertyFromMap(original repository.Property, data map[string]interface{}) repository.Property {
	result := original

	if endereco, ok := data["endereco"].(string); ok && endereco != "" {
		result.Endereco = endereco
	}
	if cidade, ok := data["cidade"].(string); ok && cidade != "" {
		result.Cidade = cidade
	}
	if bairro, ok := data["bairro"].(string); ok && bairro != "" {
		result.Bairro = bairro
	}
	if valor, ok := data["valor"].(float64); ok && valor > 0 {
		result.Valor = valor
	}
	if quartos, ok := data["quartos"].(float64); ok && quartos > 0 {
		result.Quartos = int(quartos)
	}
	if banheiros, ok := data["banheiros"].(float64); ok && banheiros > 0 {
		result.Banheiros = int(banheiros)
	}
	if area, ok := data["area_total"].(float64); ok && area > 0 {
		result.AreaTotal = area
	}
	if tipo, ok := data["tipo_imovel"].(string); ok && tipo != "" {
		result.TipoImovel = tipo
	}

	return result
}

// FlushBatch processa qualquer propriedade restante no buffer
func (s *GeminiService) FlushBatch(ctx context.Context) error {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	if len(s.batchBuffer) == 0 {
		return nil
	}

	// Processa o buffer restante
	prompt := createBatchPrompt(s.batchBuffer)
	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		// Em caso de erro, processa individualmente
		for _, property := range s.batchBuffer {
			cacheKey := s.generateCacheKey(property)
			s.processSingle(ctx, property, cacheKey)
		}
		s.batchBuffer = s.batchBuffer[:0]
		return err
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)
		processedProperties, err := parseBatchResponse(string(responseText), s.batchBuffer)
		if err == nil {
			// Armazena todas no cache
			for i, processed := range processedProperties {
				if i < len(s.batchBuffer) {
					key := s.generateCacheKey(s.batchBuffer[i])
					s.setCache(key, processed)
				}
			}
		}
	}

	// Limpa o buffer
	s.batchBuffer = s.batchBuffer[:0]
	return nil
}

// GetCacheStats retorna estatísticas do cache
func (s *GeminiService) GetCacheStats() (int, int) {
	s.cache.mutex.RLock()
	defer s.cache.mutex.RUnlock()

	total := len(s.cache.cache)
	expired := 0

	now := time.Now()
	for _, entry := range s.cache.cache {
		if now.Sub(entry.Timestamp) > s.cache.ttl {
			expired++
		}
	}

	return total, expired
}

// parseGeminiResponse analisa a resposta do Gemini e atualiza os dados da propriedade
func parseGeminiResponse(response string, originalProperty repository.Property) (repository.Property, error) {
	// Extrair apenas o JSON da resposta
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return originalProperty, fmt.Errorf("não foi possível extrair JSON da resposta")
	}

	// Criar uma estrutura temporária para o parsing
	type PropertyResponse struct {
		Endereco        string   `json:"endereco"`
		Cidade          string   `json:"cidade"`
		Bairro          string   `json:"bairro"`
		CEP             string   `json:"cep"`
		Descricao       string   `json:"descricao"`
		Valor           float64  `json:"valor"`
		Quartos         int      `json:"quartos"`
		Banheiros       int      `json:"banheiros"`
		AreaTotal       float64  `json:"area_total"`
		AreaUtil        float64  `json:"area_util"`
		TipoImovel      string   `json:"tipo_imovel"`
		Caracteristicas []string `json:"caracteristicas"`
	}

	var parsedResponse PropertyResponse
	err := json.Unmarshal([]byte(jsonStr), &parsedResponse)
	if err != nil {
		return originalProperty, fmt.Errorf("erro ao fazer parsing da resposta JSON: %v", err)
	}

	// Atualizar a propriedade original com os dados processados
	result := originalProperty

	// Só atualiza campos se tiverem valores válidos
	if parsedResponse.Endereco != "" {
		result.Endereco = parsedResponse.Endereco
	}
	if parsedResponse.Cidade != "" {
		result.Cidade = parsedResponse.Cidade
	}
	if parsedResponse.Bairro != "" {
		result.Bairro = parsedResponse.Bairro
	}
	if parsedResponse.CEP != "" {
		result.CEP = parsedResponse.CEP
	}
	if parsedResponse.Descricao != "" {
		result.Descricao = parsedResponse.Descricao
	}
	if parsedResponse.Valor > 0 {
		result.Valor = parsedResponse.Valor
	}
	if parsedResponse.Quartos > 0 {
		result.Quartos = parsedResponse.Quartos
	}
	if parsedResponse.Banheiros > 0 {
		result.Banheiros = parsedResponse.Banheiros
	}
	if parsedResponse.AreaTotal > 0 {
		result.AreaTotal = parsedResponse.AreaTotal
	}
	if parsedResponse.AreaUtil > 0 {
		result.AreaUtil = parsedResponse.AreaUtil
	}
	if parsedResponse.TipoImovel != "" {
		result.TipoImovel = parsedResponse.TipoImovel
	}
	if len(parsedResponse.Caracteristicas) > 0 {
		result.Caracteristicas = parsedResponse.Caracteristicas
	}

	return result, nil
}

// extractJSON extrai o JSON da resposta do Gemini
func extractJSON(text string) string {
	// Procura por um bloco JSON entre chaves
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start >= 0 && end > start {
		return text[start : end+1]
	}

	return ""
}

// optimizePrompt otimiza o prompt para reduzir o número de tokens
func optimizePrompt(prompt string) string {
	// Remover espaços extras
	prompt = strings.Join(strings.Fields(prompt), " ")

	// Remover linhas vazias extras
	lines := strings.Split(prompt, "\n")
	var filteredLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			filteredLines = append(filteredLines, line)
		}
	}

	return strings.Join(filteredLines, "\n")
}
