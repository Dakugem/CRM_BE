package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
)

type contextKey string

const sessionContextKey contextKey = "auth_session"

// AuthSession представляет активную сессию пользователя
type AuthSession struct {
	SessionID int64
	AccountID int64
	RoleID    int
}

// GetSessionFromRequest извлекает сессию из Bearer токена
func GetSessionFromRequest(r *http.Request) (*AuthSession, error) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, errors.New("missing or invalid authorization header")
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")
	accessTokenHash := sha256Hash(accessToken)

	// Получаем сессию из БД
	session, err := getSessionByAccessToken(accessTokenHash)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.New("session not found or expired")
	}

	// Получаем информацию об аккаунте
	var roleID int
	err = db.QueryRow(`
		SELECT role_id
		FROM auth."Accounts"
		WHERE id = $1
	`, session.AccountID).Scan(&roleID)
	if err != nil {
		return nil, err
	}

	// Обновляем время последнего использования сессии (в фоне)
	go func() {
		if err := updateSessionLastUsed(session.ID); err != nil {
			log.Printf("Failed to update session last_used_at: %v", err)
		}
	}()

	return &AuthSession{
		SessionID: session.ID,
		AccountID: session.AccountID,
		RoleID:    roleID,
	}, nil
}

// RequireAuth - middleware для проверки аутентификации
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := GetSessionFromRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), sessionContextKey, session)
		next(w, r.WithContext(ctx))
	}
}

// SessionFromContext извлекает сессию из контекста
func SessionFromContext(ctx context.Context) *AuthSession {
	s, _ := ctx.Value(sessionContextKey).(*AuthSession)
	return s
}
