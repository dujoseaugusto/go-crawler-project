package crawler

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/gocolly/colly"
)

// PersistentURLManager gerencia URLs com persistência no banco de dados
type PersistentURLManager struct {
	visitedURLs map[string]bool
	mutex       sync.RWMutex
	logger      *logger.Logger
	urlRepo     repository.URLRepository
	config      PersistentURLConfig
}

// PersistentURLConfig configurações para o gerenciador de URLs
type PersistentURLConfig struct {
	MaxAge               time.Duration // Idade máxima antes de reprocessar
	CleanupInterval      time.Duration // Intervalo para limpeza automática
	EnableFingerprinting bool          // Habilita fingerprinting de páginas
	AIThreshold          time.Duration // Tempo mínimo antes de usar IA novamente
}

// PropertyPageDetection armazena informações sobre detecção de páginas de imóveis
type PropertyPageDetection struct {
	IsPropertyPage bool
	Confidence     float64
	Indicators     []string
}

// NewPersistentURLManager cria um novo gerenciador persistente de URLs
func NewPersistentURLManager(urlRepo repository.URLRepository, config PersistentURLConfig) *PersistentURLManager {
	if config.MaxAge == 0 {
		config.MaxAge = 24 * time.Hour // Padrão: 24 horas
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 7 * 24 * time.Hour // Padrão: 7 dias
	}
	if config.AIThreshold == 0 {
		config.AIThreshold = 6 * time.Hour // Padrão: 6 horas
	}

	return &PersistentURLManager{
		visitedURLs: make(map[string]bool),
		logger:      logger.NewLogger("persistent_url_manager"),
		urlRepo:     urlRepo,
		config:      config,
	}
}

// LoadVisitedURLs carrega URLs visitadas recentemente do banco
func (pum *PersistentURLManager) LoadVisitedURLs(ctx context.Context) error {
	pum.mutex.Lock()
	defer pum.mutex.Unlock()

	since := time.Now().Add(-pum.config.MaxAge)
	urls, err := pum.urlRepo.GetProcessedURLsSince(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to load visited URLs: %v", err)
	}

	loadedCount := 0
	for _, url := range urls {
		if url.Status == "success" {
			pum.visitedURLs[url.URL] = true
			loadedCount++
		}
	}

	pum.logger.WithFields(map[string]interface{}{
		"loaded_urls": loadedCount,
		"total_urls":  len(urls),
		"since":       since,
	}).Info("Loaded visited URLs from database")

	return nil
}

// ShouldProcessURL determina se uma URL deve ser processada
func (pum *PersistentURLManager) ShouldProcessURL(ctx context.Context, url string) (*ProcessingDecision, error) {
	// Verifica se foi processada recentemente
	isRecent, err := pum.urlRepo.IsURLProcessedRecently(ctx, url, pum.config.MaxAge)
	if err != nil {
		return nil, fmt.Errorf("failed to check if URL processed recently: %v", err)
	}

	if isRecent {
		return &ProcessingDecision{
			ShouldProcess: false,
			Reason:        "recently_processed",
			LastProcessed: time.Now().Add(-pum.config.MaxAge / 2), // Estimativa
		}, nil
	}

	// Se fingerprinting está habilitado, verifica mudanças
	if pum.config.EnableFingerprinting {
		fingerprint, err := pum.urlRepo.GetFingerprint(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("failed to get fingerprint: %v", err)
		}

		if fingerprint != nil {
			// Verifica se precisa ser reprocessada baseado na idade e mudanças
			timeSinceLastCrawl := time.Since(fingerprint.LastCrawled)

			if timeSinceLastCrawl < pum.config.MaxAge {
				return &ProcessingDecision{
					ShouldProcess: false,
					Reason:        "fingerprint_fresh",
					LastProcessed: fingerprint.LastCrawled,
					Fingerprint:   fingerprint,
				}, nil
			}

			// Se é antiga mas não mudou muito, pode esperar mais
			if !fingerprint.ChangeDetected && timeSinceLastCrawl < pum.config.MaxAge*2 {
				return &ProcessingDecision{
					ShouldProcess: false,
					Reason:        "no_changes_detected",
					LastProcessed: fingerprint.LastCrawled,
					Fingerprint:   fingerprint,
				}, nil
			}
		}
	}

	return &ProcessingDecision{
		ShouldProcess: true,
		Reason:        "should_process",
	}, nil
}

// ProcessingDecision contém a decisão sobre processar uma URL
type ProcessingDecision struct {
	ShouldProcess bool                        `json:"should_process"`
	Reason        string                      `json:"reason"`
	LastProcessed time.Time                   `json:"last_processed,omitempty"`
	Fingerprint   *repository.PageFingerprint `json:"fingerprint,omitempty"`
}

// GeneratePageFingerprint gera fingerprint de uma página
func (pum *PersistentURLManager) GeneratePageFingerprint(e *colly.HTMLElement) string {
	// Extrai elementos relevantes para criar fingerprint
	prices := pum.extractAllPrices(e)
	addresses := pum.extractAllAddresses(e)
	descriptions := pum.extractAllDescriptions(e)

	return repository.GenerateContentHash(prices, addresses, descriptions)
}

// extractAllPrices extrai todos os preços da página
func (pum *PersistentURLManager) extractAllPrices(e *colly.HTMLElement) []string {
	var prices []string

	// Padrões para detectar preços
	priceSelectors := []string{
		".price", ".valor", ".preco", "[class*=price]", "[class*=valor]",
		"[data-price]", ".property-price", ".listing-price",
	}

	for _, selector := range priceSelectors {
		e.ForEach(selector, func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" && pum.looksLikePrice(text) {
				prices = append(prices, text)
			}
		})
	}

	// Busca por padrões de preço no texto
	priceRegex := regexp.MustCompile(`R\$\s*[\d.,]+`)
	allText := e.Text
	matches := priceRegex.FindAllString(allText, -1)
	prices = append(prices, matches...)

	return pum.uniqueStrings(prices)
}

// extractAllAddresses extrai todos os endereços da página
func (pum *PersistentURLManager) extractAllAddresses(e *colly.HTMLElement) []string {
	var addresses []string

	// Seletores para endereços
	addressSelectors := []string{
		".address", ".endereco", ".location", ".localizacao",
		"[class*=address]", "[class*=endereco]", "[class*=location]",
		".property-address", ".listing-address",
	}

	for _, selector := range addressSelectors {
		e.ForEach(selector, func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" && len(text) > 10 { // Endereços geralmente são longos
				addresses = append(addresses, text)
			}
		})
	}

	// Busca por padrões de endereço
	addressRegex := regexp.MustCompile(`(?i)(R(?:ua|\.)\s+[\w\s]+,\s*\d+|Av(?:enida|\.)\s+[\w\s]+,\s*\d+)`)
	matches := addressRegex.FindAllString(e.Text, -1)
	addresses = append(addresses, matches...)

	return pum.uniqueStrings(addresses)
}

// extractAllDescriptions extrai todas as descrições da página
func (pum *PersistentURLManager) extractAllDescriptions(e *colly.HTMLElement) []string {
	var descriptions []string

	// Seletores para descrições
	descSelectors := []string{
		".description", ".descricao", ".details", ".detalhes",
		"[class*=description]", "[class*=descricao]", "[class*=details]",
		".property-description", ".listing-description", "p",
	}

	for _, selector := range descSelectors {
		e.ForEach(selector, func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" && len(text) > 50 && len(text) < 2000 { // Descrições têm tamanho médio
				descriptions = append(descriptions, text)
			}
		})
	}

	return pum.uniqueStrings(descriptions)
}

// looksLikePrice verifica se um texto parece ser um preço
func (pum *PersistentURLManager) looksLikePrice(text string) bool {
	// Remove espaços e converte para minúsculo
	clean := strings.ToLower(strings.ReplaceAll(text, " ", ""))

	// Verifica padrões de preço
	pricePatterns := []string{"r$", "rs", "real", "reais"}
	for _, pattern := range pricePatterns {
		if strings.Contains(clean, pattern) {
			return true
		}
	}

	// Verifica se tem números e vírgulas/pontos (formato de dinheiro)
	hasNumbers := regexp.MustCompile(`\d`).MatchString(text)
	hasCurrency := regexp.MustCompile(`[.,]`).MatchString(text)

	return hasNumbers && hasCurrency
}

// uniqueStrings remove duplicatas de uma slice de strings
func (pum *PersistentURLManager) uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	var unique []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			unique = append(unique, item)
		}
	}

	return unique
}

// MarkURLProcessed marca uma URL como processada
func (pum *PersistentURLManager) MarkURLProcessed(ctx context.Context, url string, status string, errorMsg string) error {
	pum.mutex.Lock()
	defer pum.mutex.Unlock()

	processedURL := repository.ProcessedURL{
		URL:         url,
		ProcessedAt: time.Now(),
		Status:      status,
		ErrorMsg:    errorMsg,
	}

	if err := pum.urlRepo.SaveProcessedURL(ctx, processedURL); err != nil {
		return fmt.Errorf("failed to mark URL as processed: %v", err)
	}

	if status == "success" {
		pum.visitedURLs[url] = true
	}

	return nil
}

// NormalizeURL normaliza uma URL removendo parâmetros desnecessários e caracteres problemáticos
func (pum *PersistentURLManager) NormalizeURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // Retorna original se não conseguir parsear
	}

	// Remove espaços codificados e outros caracteres problemáticos do path
	path := parsedURL.Path
	path = strings.ReplaceAll(path, "%20", "")
	path = strings.ReplaceAll(path, "/%20/", "/")
	path = strings.ReplaceAll(path, "//", "/")
	path = strings.TrimSuffix(path, "/")

	// Remove parâmetros de query desnecessários (mantém apenas essenciais)
	query := parsedURL.Query()
	essentialParams := url.Values{}
	
	// Mantém apenas parâmetros que identificam o imóvel
	for key, values := range query {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "id") || 
		   strings.Contains(lowerKey, "imovel") || 
		   strings.Contains(lowerKey, "property") ||
		   strings.Contains(lowerKey, "codigo") {
			essentialParams[key] = values
		}
	}

	// Reconstrói a URL normalizada
	normalizedURL := &url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
		Path:   path,
	}

	if len(essentialParams) > 0 {
		normalizedURL.RawQuery = essentialParams.Encode()
	}

	return normalizedURL.String()
}

// DetectPropertyPage analisa se uma página é de anúncio de imóvel
func (pum *PersistentURLManager) DetectPropertyPage(e *colly.HTMLElement) PropertyPageDetection {
	var indicators []string
	confidence := 0.0

	// Verifica indicadores na URL
	urlStr := e.Request.URL.String()
	urlLower := strings.ToLower(urlStr)
	
	if strings.Contains(urlLower, "imovel") || strings.Contains(urlLower, "property") {
		indicators = append(indicators, "url_contains_property_keyword")
		confidence += 0.3
	}

	// Verifica presença de preços
	priceSelectors := []string{
		"*[class*='price']", "*[class*='preco']", "*[class*='valor']",
		"*[id*='price']", "*[id*='preco']", "*[id*='valor']",
	}
	
	for _, selector := range priceSelectors {
		e.ForEach(selector, func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if pum.looksLikePrice(text) {
				indicators = append(indicators, "price_found")
				confidence += 0.4
				return
			}
		})
	}

	// Verifica características de imóveis
	propertyKeywords := []string{
		"quarto", "dormitório", "banheiro", "garagem", "área", "m²", "m2",
		"suite", "suíte", "sala", "cozinha", "varanda", "terraço",
	}

	bodyText := strings.ToLower(e.Text)
	keywordCount := 0
	for _, keyword := range propertyKeywords {
		if strings.Contains(bodyText, keyword) {
			keywordCount++
		}
	}

	if keywordCount >= 3 {
		indicators = append(indicators, fmt.Sprintf("property_keywords_found_%d", keywordCount))
		confidence += float64(keywordCount) * 0.05
	}

	// Verifica estrutura típica de página de imóvel
	structureSelectors := []string{
		"*[class*='caracteristica']", "*[class*='feature']", "*[class*='detail']",
		"*[class*='info']", "*[class*='specification']",
	}

	structureCount := 0
	for _, selector := range structureSelectors {
		e.ForEach(selector, func(_ int, el *colly.HTMLElement) {
			structureCount++
		})
	}

	if structureCount > 0 {
		indicators = append(indicators, "property_structure_found")
		confidence += 0.2
	}

	// Verifica se tem galeria de fotos ou imagens
	e.ForEach("img", func(_ int, el *colly.HTMLElement) {
		src := strings.ToLower(el.Attr("src"))
		alt := strings.ToLower(el.Attr("alt"))
		
		if strings.Contains(src, "imovel") || strings.Contains(alt, "imovel") ||
		   strings.Contains(src, "property") || strings.Contains(alt, "property") {
			indicators = append(indicators, "property_images_found")
			confidence += 0.1
		}
	})

	// Determina se é página de imóvel (threshold: 0.5)
	isPropertyPage := confidence >= 0.5

	return PropertyPageDetection{
		IsPropertyPage: isPropertyPage,
		Confidence:     confidence,
		Indicators:     indicators,
	}
}

// ShouldSkipInternalLinks determina se deve parar de seguir links internos desta página
func (pum *PersistentURLManager) ShouldSkipInternalLinks(e *colly.HTMLElement) bool {
	detection := pum.DetectPropertyPage(e)
	
	// Se detectou que é uma página de imóvel com alta confiança, para de seguir links internos
	if detection.IsPropertyPage && detection.Confidence >= 0.7 {
		pum.logger.WithFields(map[string]interface{}{
			"url":        e.Request.URL.String(),
			"confidence": detection.Confidence,
			"indicators": detection.Indicators,
		}).Info("Property page detected - stopping internal link crawling")
		
		return true
	}

	return false
}

// SavePageFingerprint salva o fingerprint de uma página
func (pum *PersistentURLManager) SavePageFingerprint(ctx context.Context, url string, contentHash string, propertyCount int, aiProcessed bool) error {
	// Verifica se já existe um fingerprint
	existing, err := pum.urlRepo.GetFingerprint(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to get existing fingerprint: %v", err)
	}

	now := time.Now()
	changeDetected := false

	if existing != nil {
		// Detecta se houve mudança no conteúdo
		changeDetected = existing.ContentHash != contentHash

		if changeDetected {
			pum.logger.WithFields(map[string]interface{}{
				"url":                url,
				"old_hash":           existing.ContentHash[:8],
				"new_hash":           contentHash[:8],
				"old_property_count": existing.PropertyCount,
				"new_property_count": propertyCount,
			}).Info("Content change detected")
		}
	} else {
		// Primeira vez que vemos esta página
		changeDetected = true
	}

	fingerprint := repository.PageFingerprint{
		URL:            url,
		ContentHash:    contentHash,
		LastModified:   now,
		PropertyCount:  propertyCount,
		LastCrawled:    now,
		ChangeDetected: changeDetected,
		AIProcessed:    aiProcessed,
		ProcessingTime: 0, // Será atualizado pelo caller se necessário
	}

	if existing != nil {
		// Preserva algumas informações do fingerprint anterior
		if !changeDetected {
			fingerprint.LastModified = existing.LastModified
		}
	}

	if err := pum.urlRepo.SaveFingerprint(ctx, fingerprint); err != nil {
		return fmt.Errorf("failed to save fingerprint: %v", err)
	}

	return nil
}

// ShouldUseAI determina se deve usar IA para processar uma página
func (pum *PersistentURLManager) ShouldUseAI(ctx context.Context, url string) (bool, string) {
	if !pum.config.EnableFingerprinting {
		return true, "fingerprinting_disabled"
	}

	fingerprint, err := pum.urlRepo.GetFingerprint(ctx, url)
	if err != nil || fingerprint == nil {
		return true, "no_fingerprint"
	}

	// Se nunca foi processado com IA, processar
	if !fingerprint.AIProcessed {
		return true, "never_ai_processed"
	}

	// Se houve mudança detectada, processar com IA
	if fingerprint.ChangeDetected {
		return true, "content_changed"
	}

	// Se foi processado com IA recentemente, pular
	if time.Since(fingerprint.LastCrawled) < pum.config.AIThreshold {
		return false, "ai_recently_processed"
	}

	// Se é uma página com muitas propriedades, processar mais frequentemente
	if fingerprint.PropertyCount > 10 {
		return true, "high_property_count"
	}

	// Caso padrão: não usar IA se não houve mudanças
	return false, "no_changes_no_ai_needed"
}

// GetStatistics retorna estatísticas do gerenciador
func (pum *PersistentURLManager) GetStatistics(ctx context.Context) (*repository.URLStatistics, error) {
	return pum.urlRepo.GetStatistics(ctx)
}

// CleanupOldRecords limpa registros antigos
func (pum *PersistentURLManager) CleanupOldRecords(ctx context.Context) error {
	return pum.urlRepo.CleanupOldRecords(ctx, pum.config.CleanupInterval)
}

// IsVisited verifica se uma URL foi visitada (compatibilidade com URLManager original)
func (pum *PersistentURLManager) IsVisited(url string) bool {
	pum.mutex.RLock()
	defer pum.mutex.RUnlock()
	return pum.visitedURLs[url]
}

// MarkVisited marca uma URL como visitada (compatibilidade com URLManager original)
func (pum *PersistentURLManager) MarkVisited(url string) {
	pum.mutex.Lock()
	defer pum.mutex.Unlock()
	pum.visitedURLs[url] = true
}

// GetVisitedCount retorna o número de URLs visitadas
func (pum *PersistentURLManager) GetVisitedCount() int {
	pum.mutex.RLock()
	defer pum.mutex.RUnlock()
	return len(pum.visitedURLs)
}

