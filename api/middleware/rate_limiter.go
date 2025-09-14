package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ClientInfo armazena informações sobre requisições de um cliente
type ClientInfo struct {
	RequestCount int
	WindowStart  time.Time
	LastRequest  time.Time
}

// RateLimiter implementa rate limiting baseado em IP
type RateLimiter struct {
	clients    map[string]*ClientInfo
	mutex      sync.RWMutex
	maxReqs    int           // Máximo de requisições por janela
	window     time.Duration // Duração da janela
	cleanupInt time.Duration // Intervalo de limpeza
}

// NewRateLimiter cria um novo rate limiter
func NewRateLimiter(maxReqs int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:    make(map[string]*ClientInfo),
		maxReqs:    maxReqs,
		window:     window,
		cleanupInt: window * 2, // Limpa clientes inativos a cada 2 janelas
	}

	// Inicia goroutine de limpeza
	go rl.cleanup()

	return rl
}

// Middleware retorna o middleware do Gin para rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if !rl.allowRequest(clientIP) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Muitas requisições. Tente novamente em alguns minutos.",
				"code":    http.StatusTooManyRequests,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// allowRequest verifica se uma requisição deve ser permitida
func (rl *RateLimiter) allowRequest(clientIP string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	client, exists := rl.clients[clientIP]

	if !exists {
		// Novo cliente
		rl.clients[clientIP] = &ClientInfo{
			RequestCount: 1,
			WindowStart:  now,
			LastRequest:  now,
		}
		return true
	}

	// Verifica se a janela expirou
	if now.Sub(client.WindowStart) > rl.window {
		// Nova janela
		client.RequestCount = 1
		client.WindowStart = now
		client.LastRequest = now
		return true
	}

	// Dentro da janela atual
	client.LastRequest = now

	if client.RequestCount >= rl.maxReqs {
		return false // Rate limit excedido
	}

	client.RequestCount++
	return true
}

// cleanup remove clientes inativos periodicamente
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInt)
	defer ticker.Stop()

	for range ticker.C {
		rl.mutex.Lock()
		now := time.Now()

		for ip, client := range rl.clients {
			// Remove clientes inativos há mais de 2 janelas
			if now.Sub(client.LastRequest) > rl.cleanupInt {
				delete(rl.clients, ip)
			}
		}

		rl.mutex.Unlock()
	}
}

// GetStats retorna estatísticas do rate limiter
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	return map[string]interface{}{
		"active_clients": len(rl.clients),
		"max_requests":   rl.maxReqs,
		"window_minutes": rl.window.Minutes(),
	}
}
