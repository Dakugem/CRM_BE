package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// IPRateLimiter отслеживает запросы по IP
type IPRateLimiter struct {
	ips map[string]*rateLimiter
	mu  sync.RWMutex
}

// rateLimiter для отдельного IP
type rateLimiter struct {
	tokens         int
	lastRefill     time.Time
	requestCount   int
	firstRequest   time.Time
	blockedUntil   time.Time
}

var (
	globalRateLimiter *IPRateLimiter

	// Настройки rate limiting
	maxTokens       = 100    // Максимум токенов (запросов в burst)
	refillRate      = 10     // Токенов в секунду
	maxRequestsPer  = 200    // Максимум запросов
	perDuration     = 1 * time.Minute // За период
	blockDuration   = 5 * time.Minute // Время блокировки при превышении
	cleanupInterval = 10 * time.Minute // Очистка старых записей
)

// InitRateLimiter инициализирует rate limiter
func InitRateLimiter() {
	globalRateLimiter = &IPRateLimiter{
		ips: make(map[string]*rateLimiter),
	}

	// Запускаем периодическую очистку старых IP
	go globalRateLimiter.cleanup()

	LogInfo("Rate limiter initialized: %d req/sec, %d req/%v max",
		refillRate, maxRequestsPer, perDuration)
}

// cleanup периодически удаляет старые записи
func (ipl *IPRateLimiter) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		ipl.mu.Lock()
		now := time.Now()
		for ip, limiter := range ipl.ips {
			// Удаляем если IP не использовался > 30 минут
			if now.Sub(limiter.lastRefill) > 30*time.Minute {
				delete(ipl.ips, ip)
			}
		}
		ipl.mu.Unlock()
	}
}

// getLimiter получает или создаёт limiter для IP
func (ipl *IPRateLimiter) getLimiter(ip string) *rateLimiter {
	ipl.mu.Lock()
	defer ipl.mu.Unlock()

	limiter, exists := ipl.ips[ip]
	if !exists {
		limiter = &rateLimiter{
			tokens:       maxTokens,
			lastRefill:   time.Now(),
			firstRequest: time.Now(),
			requestCount: 0,
		}
		ipl.ips[ip] = limiter
	}

	return limiter
}

// allow проверяет, разрешён ли запрос для IP
func (rl *rateLimiter) allow() bool {
	now := time.Now()

	// Проверяем блокировку
	if now.Before(rl.blockedUntil) {
		return false
	}

	// Сбрасываем счётчик если прошёл период
	if now.Sub(rl.firstRequest) > perDuration {
		rl.requestCount = 0
		rl.firstRequest = now
	}

	// Проверяем превышение лимита за период
	rl.requestCount++
	if rl.requestCount > maxRequestsPer {
		rl.blockedUntil = now.Add(blockDuration)
		return false
	}

	// Token bucket algorithm
	elapsed := now.Sub(rl.lastRefill)
	rl.lastRefill = now

	// Добавляем токены
	tokensToAdd := int(elapsed.Seconds() * float64(refillRate))
	rl.tokens += tokensToAdd
	if rl.tokens > maxTokens {
		rl.tokens = maxTokens
	}

	// Проверяем наличие токена
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// rateLimitMiddleware проверяет rate limit для каждого запроса
func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем IP клиента
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}

		// Пропускаем rate limiting для локальных запросов
		if clientIP == "127.0.0.1" || clientIP == "::1" {
			next.ServeHTTP(w, r)
			return
		}

		// Получаем limiter для IP
		limiter := globalRateLimiter.getLimiter(clientIP)

		// Проверяем разрешён ли запрос
		if !limiter.allow() {
			// Логируем только в консоль, не в Telegram
			log.Printf("WARNING: Rate limit exceeded for IP: %s", clientIP)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "300") // 5 минут
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Too many requests. Please try again later."}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
