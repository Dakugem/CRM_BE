package main

import (
	"bytes"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// responseWriter wrapper для перехвата status code и body
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	rw.body.Write(b) // Копируем в буфер
	return rw.ResponseWriter.Write(b)
}

// truncateString обрезает строку до maxLen символов
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// sanitizeForTelegram заменяет служебные символы HTML
func sanitizeForTelegram(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// loggedEndpoints содержит список эндпоинтов, которые нужно логировать в Telegram
var loggedEndpoints = []string{
	"/api/auth/login",
	"/api/auth/logout",
	"/api/tickets",
	"/api/employees",
	"/api/clients",
	"/api/sites",
	"/api/equipment",
	"/api/dashboards",
	"/api/profile",
}

// shouldLogEndpoint проверяет, нужно ли логировать этот эндпоинт
func shouldLogEndpoint(path string) bool {
	for _, endpoint := range loggedEndpoints {
		if strings.HasPrefix(path, endpoint) {
			return true
		}
	}
	return false
}

// shouldSkipLogging проверяет, нужно ли пропустить логирование (боты, сканеры)
func shouldSkipLogging(r *http.Request) bool {
	// Игнорируем известные пути атак
	suspiciousPaths := []string{
		"/vendor/phpunit",
		"eval-stdin.php",
		"/wp-",
		".php",
		".env",
		"/.git",
		"/config",
		"/etc/passwd",
		"invokefunction",
		"pearcmd",
		"/containers/json",
		"/actuator",
		"/.aws",
	}

	path := r.URL.Path
	query := r.URL.RawQuery

	for _, sus := range suspiciousPaths {
		if strings.Contains(path, sus) || strings.Contains(query, sus) {
			return true
		}
	}

	// Игнорируем запросы к несуществующим расширениям
	if strings.HasSuffix(path, ".php") ||
	   strings.HasSuffix(path, ".asp") ||
	   strings.HasSuffix(path, ".aspx") ||
	   strings.HasSuffix(path, ".jsp") {
		return true
	}

	return false
}

// loggingMiddleware логирует все HTTP запросы с телом
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Пропускаем логирование подозрительных запросов (атаки/боты)
		skipLogging := shouldSkipLogging(r)

		// Читаем тело запроса
		var requestBody []byte
		if r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody)) // Восстанавливаем body
		}

		// Оборачиваем ResponseWriter для перехвата ответа
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     0,
			body:           &bytes.Buffer{},
		}

		// Обрабатываем запрос
		next.ServeHTTP(wrapped, r)

		// Вычисляем время выполнения
		duration := time.Since(start)

		// Определяем IP клиента
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}

		// Формируем эмодзи по статусу
		statusEmoji := "✅"
		if wrapped.statusCode >= 400 && wrapped.statusCode < 500 {
			statusEmoji = "⚠️"
		} else if wrapped.statusCode >= 500 {
			statusEmoji = "🔴"
		}

		// Базовая информация о запросе
		logHeader := fmt.Sprintf("%s %s %s %d (%s) from %s",
			statusEmoji,
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			duration.Round(time.Millisecond),
			clientIP,
		)

		// Формируем детальный лог
		var logBuilder strings.Builder
		logBuilder.WriteString(logHeader)

		// Добавляем query параметры если есть
		if r.URL.RawQuery != "" {
			logBuilder.WriteString("\n📋 Query: ")
			logBuilder.WriteString(sanitizeForTelegram(r.URL.RawQuery))
		}

		// Добавляем заголовки запроса (только важные)
		if auth := r.Header.Get("Authorization"); auth != "" {
			logBuilder.WriteString("\n🔑 Auth: ")
			if len(auth) > 20 {
				logBuilder.WriteString(auth[:20] + "...")
			} else {
				logBuilder.WriteString(auth)
			}
		}

		// Добавляем тело запроса (если есть и это JSON)
		if len(requestBody) > 0 && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			logBuilder.WriteString("\n📤 Request: ")
			reqStr := string(requestBody)
			// Маскируем пароли
			reqStr = strings.ReplaceAll(reqStr, `"password":"`, `"password":"***`)
			if i := strings.Index(reqStr, `"password":"***`); i != -1 {
				// Находим конец значения пароля
				if j := strings.Index(reqStr[i+18:], `"`); j != -1 {
					reqStr = reqStr[:i+18] + "***" + reqStr[i+18+j:]
				}
			}
			logBuilder.WriteString(sanitizeForTelegram(truncateString(reqStr, 500)))
		}

		// Добавляем тело ответа (если это JSON)
		responseBody := wrapped.body.String()
		if len(responseBody) > 0 && strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			logBuilder.WriteString("\n📥 Response: ")
			logBuilder.WriteString(sanitizeForTelegram(truncateString(responseBody, 500)))
		}

		fullLog := logBuilder.String()

		// Пропускаем логирование если это атака/бот
		if skipLogging {
			return
		}

		// Логируем в консоль всегда
		log.Println(fullLog)

		// Логируем в Telegram только если эндпоинт в списке
		if tgLogger != nil && tgLogger.enabled && shouldLogEndpoint(r.URL.Path) {
			if wrapped.statusCode >= 500 {
				tgLogger.SendLogSync(fullLog) // Критические ошибки - сразу
			} else {
				tgLogger.SendLog(fullLog) // Остальное - в батч
			}
		}
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func StartsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func HashPassword(password string) string {
	hash := md5.Sum([]byte(password))
	return hex.EncodeToString(hash[:])
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func pathInt(r *http.Request, name string) (int64, error) {
	s := r.PathValue(name)
	if s == "" {
		return 0, fmt.Errorf("missing path parameter %s", name)
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %q", name, s)
	}
	return id, nil
}

func nullStr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}

func nullInt64(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

func nullTime(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	return &n.Time
}
