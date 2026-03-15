package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	AccessTokenExpiry  = 15 * time.Minute
	RefreshTokenExpiry = 7 * 24 * time.Hour
	TokenByteLength    = 32
)

type Account struct {
	ID           int64  `json:"id"`
	Login        string `json:"login"`
	PasswordHash string `json:"-"`
	RoleID       int    `json:"role_id"`
}

type SessionDB struct {
	ID                    int64
	AccountID             int64
	AccessTokenHash       string
	AccessTokenExpiresAt  time.Time
	RefreshTokenHash      string
	RefreshTokenExpiresAt time.Time
	CreatedAt             time.Time
	LastUsedAt            sql.NullTime
	RevokedAt             sql.NullTime
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func generateRandomToken(byteLength int) (string, error) {
	buffer := make([]byte, byteLength)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func sha256Hash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func checkPassword(password, passwordHash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	return err == nil
}

func parseBasicAuth(authHeader string) (username, password string, err error) {
	if !strings.HasPrefix(authHeader, "Basic ") {
		return "", "", errors.New("invalid authorization header format")
	}

	base64Credentials := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(base64Credentials)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode base64: %w", err)
	}

	credentials := string(decoded)
	colonPos := strings.Index(credentials, ":")
	if colonPos == -1 {
		return "", "", errors.New("invalid basic auth format")
	}

	return credentials[:colonPos], credentials[colonPos+1:], nil
}

func getAccountByLogin(login string) (*Account, error) {
	var acc Account
	err := db.QueryRow(`
		SELECT id, login, password_hash, role_id
		FROM auth."Accounts"
		WHERE login = $1
	`, login).Scan(&acc.ID, &acc.Login, &acc.PasswordHash, &acc.RoleID)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query account: %w", err)
	}
	return &acc, nil
}

func createSession(accountID int64, accessTokenHash, refreshTokenHash string) (*SessionDB, error) {
	var session SessionDB
	err := db.QueryRow(`
		INSERT INTO auth."Sessions"
		(account_id, access_token_hash, access_token_expires_at, refresh_token_hash, refresh_token_expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, account_id, access_token_hash, access_token_expires_at,
		          refresh_token_hash, refresh_token_expires_at, created_at
	`,
		accountID,
		accessTokenHash,
		time.Now().Add(AccessTokenExpiry),
		refreshTokenHash,
		time.Now().Add(RefreshTokenExpiry),
	).Scan(
		&session.ID,
		&session.AccountID,
		&session.AccessTokenHash,
		&session.AccessTokenExpiresAt,
		&session.RefreshTokenHash,
		&session.RefreshTokenExpiresAt,
		&session.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	return &session, nil
}

func getSessionByAccessToken(accessTokenHash string) (*SessionDB, error) {
	var session SessionDB
	err := db.QueryRow(`
		SELECT id, account_id, access_token_hash, access_token_expires_at,
		       refresh_token_hash, refresh_token_expires_at, created_at, last_used_at, revoked_at
		FROM auth."Sessions"
		WHERE access_token_hash = $1
		  AND revoked_at IS NULL
		  AND access_token_expires_at > NOW()
	`, accessTokenHash).Scan(
		&session.ID,
		&session.AccountID,
		&session.AccessTokenHash,
		&session.AccessTokenExpiresAt,
		&session.RefreshTokenHash,
		&session.RefreshTokenExpiresAt,
		&session.CreatedAt,
		&session.LastUsedAt,
		&session.RevokedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}
	return &session, nil
}

func getSessionByRefreshToken(refreshTokenHash string) (*SessionDB, error) {
	var session SessionDB
	err := db.QueryRow(`
		SELECT id, account_id, access_token_hash, access_token_expires_at,
		       refresh_token_hash, refresh_token_expires_at, created_at, last_used_at, revoked_at
		FROM auth."Sessions"
		WHERE refresh_token_hash = $1
		  AND revoked_at IS NULL
		  AND refresh_token_expires_at > NOW()
	`, refreshTokenHash).Scan(
		&session.ID,
		&session.AccountID,
		&session.AccessTokenHash,
		&session.AccessTokenExpiresAt,
		&session.RefreshTokenHash,
		&session.RefreshTokenExpiresAt,
		&session.CreatedAt,
		&session.LastUsedAt,
		&session.RevokedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}
	return &session, nil
}

func updateSessionLastUsed(sessionID int64) error {
	_, err := db.Exec(`
		UPDATE auth."Sessions"
		SET last_used_at = NOW()
		WHERE id = $1
	`, sessionID)
	return err
}

func revokeSession(accessTokenHash string) error {
	_, err := db.Exec(`
		UPDATE auth."Sessions"
		SET revoked_at = NOW()
		WHERE access_token_hash = $1
	`, accessTokenHash)
	return err
}

func revokeSessionByRefreshToken(refreshTokenHash string) error {
	_, err := db.Exec(`
		UPDATE auth."Sessions"
		SET revoked_at = NOW()
		WHERE refresh_token_hash = $1
	`, refreshTokenHash)
	return err
}

// curl -v -H "Authorization: Basic dXNlcjpwYXNz" -X POST http://localhost:8080/api/auth/login
func AuthLoginHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
		w.Header().Set("WWW-Authenticate", `Basic realm="Login"`)
		writeError(w, http.StatusUnauthorized, "Missing or invalid Authorization header. Expected Basic auth.")
		return
	}

	login, password, err := parseBasicAuth(authHeader)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Basic auth format")
		return
	}

	// Получаем аккаунт из БД
	account, err := getAccountByLogin(login)
	if err != nil {
		LogError("Failed to get account: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if account == nil {
		writeError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	// Проверяем пароль
	if !checkPassword(password, account.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	// Генерируем токены
	accessToken, err := generateRandomToken(TokenByteLength)
	if err != nil {
		LogError("Failed to generate access token: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	refreshToken, err := generateRandomToken(TokenByteLength)
	if err != nil {
		LogError("Failed to generate refresh token: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Хешируем токены
	accessTokenHash := sha256Hash(accessToken)
	refreshTokenHash := sha256Hash(refreshToken)

	// Создаем сессию в БД
	_, err = createSession(account.ID, accessTokenHash, refreshTokenHash)
	if err != nil {
		LogError("Failed to create session: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Возвращаем токены
	writeJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	})

	LogSuccess("Login successful: account_id=%d, login=%s", account.ID, account.Login)
}

// curl -v -H "Content-Type: application/json" -d '{"refresh_token": "..."}' -X POST http://localhost:8080/api/auth/refresh
func AuthRefreshHandler(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "Missing refresh_token in body")
		return
	}

	// Хешируем refresh token
	refreshTokenHash := sha256Hash(req.RefreshToken)

	// Проверяем сессию по refresh token
	session, err := getSessionByRefreshToken(refreshTokenHash)
	if err != nil {
		LogError("Failed to get session by refresh token: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if session == nil {
		writeError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	// Отзываем старую сессию
	if err := revokeSessionByRefreshToken(refreshTokenHash); err != nil {
		LogError("Failed to revoke old session: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Генерируем новые токены
	newAccessToken, err := generateRandomToken(TokenByteLength)
	if err != nil {
		LogError("Failed to generate new access token: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	newRefreshToken, err := generateRandomToken(TokenByteLength)
	if err != nil {
		LogError("Failed to generate new refresh token: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Хешируем новые токены
	newAccessTokenHash := sha256Hash(newAccessToken)
	newRefreshTokenHash := sha256Hash(newRefreshToken)

	// Создаем новую сессию
	_, err = createSession(session.AccountID, newAccessTokenHash, newRefreshTokenHash)
	if err != nil {
		LogError("Failed to create new session: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Возвращаем новые токены
	writeJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
	})

	LogSuccess("Token refresh successful: account_id=%d", session.AccountID)
}

// curl -v -H "Authorization: Bearer ..." -X POST http://localhost:8080/api/auth/logout
func AuthLogoutHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "Missing or invalid Authorization header. Expected Bearer token.")
		return
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")
	accessTokenHash := sha256Hash(accessToken)

	// Отзываем сессию
	if err := revokeSession(accessTokenHash); err != nil {
		LogError("Failed to revoke session: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
	LogSuccess("Logout successful")
}
