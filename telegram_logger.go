package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// TelegramLogger отправляет логи в Telegram
type TelegramLogger struct {
	botToken      string
	chatID        string
	threadID      string // ID топика/треда в супергруппе (опционально)
	enabled       bool
	mu            sync.Mutex
	buffer        []string
	ticker        *time.Ticker
}

var tgLogger *TelegramLogger

// InitTelegramLogger инициализирует Telegram логгер
func InitTelegramLogger() {
	// Проверяем, включен ли Telegram логгер (по умолчанию "true", если не указано)
	telegramEnabled := os.Getenv("TELEGRAM_ENABLED")
	if telegramEnabled == "false" || telegramEnabled == "0" {
		log.Println("✗ Telegram logger disabled (TELEGRAM_ENABLED=false)")
		tgLogger = &TelegramLogger{enabled: false}
		return
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	threadID := os.Getenv("TELEGRAM_THREAD_ID") // Опционально для топиков в супергруппах

	tgLogger = &TelegramLogger{
		botToken: botToken,
		chatID:   chatID,
		threadID: threadID,
		enabled:  botToken != "" && chatID != "",
		buffer:   make([]string, 0),
	}

	if tgLogger.enabled {
		log.Println("✓ Telegram logger enabled")
		// Отправляем сообщение о старте
		tgLogger.SendLog("🚀 CRM Backend started")

		// Запускаем батчинг (отправка каждые 5 секунд)
		tgLogger.ticker = time.NewTicker(5 * time.Second)
		go tgLogger.flushLoop()
	} else {
		log.Println("✗ Telegram logger disabled (no credentials)")
	}
}

// SendLog отправляет лог в Telegram (с батчингом)
func (tl *TelegramLogger) SendLog(message string) {
	if !tl.enabled {
		return
	}

	tl.mu.Lock()
	defer tl.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	tl.buffer = append(tl.buffer, fmt.Sprintf("[%s] %s", timestamp, message))

	// Если буфер достиг 10 сообщений, отправляем сразу
	if len(tl.buffer) >= 10 {
		tl.flush()
	}
}

// SendLogSync отправляет лог в Telegram синхронно (для критичных сообщений)
func (tl *TelegramLogger) SendLogSync(message string) {
	if !tl.enabled {
		return
	}

	timestamp := time.Now().Format("15:04:05")
	formattedMsg := fmt.Sprintf("[%s] %s", timestamp, message)
	tl.sendToTelegram(formattedMsg)
}

// flush отправляет накопленные логи (вызывается под mutex)
func (tl *TelegramLogger) flush() {
	if len(tl.buffer) == 0 {
		return
	}

	// Объединяем все сообщения
	var combined string
	for _, msg := range tl.buffer {
		combined += msg + "\n"
	}

	// Очищаем буфер
	tl.buffer = tl.buffer[:0]

	// Отправляем в отдельной горутине
	go tl.sendToTelegram(combined)
}

// flushLoop периодически отправляет накопленные логи
func (tl *TelegramLogger) flushLoop() {
	for range tl.ticker.C {
		tl.mu.Lock()
		tl.flush()
		tl.mu.Unlock()
	}
}

// sendToTelegram отправляет сообщение в Telegram API
func (tl *TelegramLogger) sendToTelegram(message string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tl.botToken)

	// Ограничиваем длину сообщения (Telegram limit 4096)
	if len(message) > 4000 {
		message = message[:4000] + "\n... (truncated)"
	}

	payload := map[string]interface{}{
		"chat_id":    tl.chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	// Если указан thread_id, добавляем его для отправки в конкретный топик
	if tl.threadID != "" {
		payload["message_thread_id"] = tl.threadID
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal telegram message: %v", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to send telegram message: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Telegram API returned status %d", resp.StatusCode)
	}
}

// LogInfo логирует информационное сообщение (в консоль и в бота)
func LogInfo(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	log.Println(message) // Всегда пишем в консоль
	if tgLogger != nil && tgLogger.enabled {
		tgLogger.SendLog("ℹ️ " + message)
	}
}

// LogError логирует ошибку (в консоль и в бота)
func LogError(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	log.Printf("ERROR: %s", message) // Всегда пишем в консоль
	if tgLogger != nil && tgLogger.enabled {
		tgLogger.SendLogSync("🔴 ERROR: " + message)
	}
}

// LogWarning логирует предупреждение (в консоль и в бота)
func LogWarning(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	log.Printf("WARNING: %s", message) // Всегда пишем в консоль
	if tgLogger != nil && tgLogger.enabled {
		tgLogger.SendLog("⚠️ WARNING: " + message)
	}
}

// LogSuccess логирует успешную операцию (в консоль и в бота)
func LogSuccess(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	log.Println(message) // Всегда пишем в консоль
	if tgLogger != nil && tgLogger.enabled {
		tgLogger.SendLog("✅ " + message)
	}
}

// Close закрывает логгер
func (tl *TelegramLogger) Close() {
	if tl.enabled && tl.ticker != nil {
		tl.ticker.Stop()
		tl.mu.Lock()
		tl.flush()
		tl.mu.Unlock()
		tl.SendLogSync("🛑 CRM Backend stopped")
	}
}
