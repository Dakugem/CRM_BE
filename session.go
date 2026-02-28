package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

type Session struct {
	ID     int64     `json:"id"`
	Expiry time.Time `json:"expiry"`
	User   User      `json:"user"`
	Token  string    `json:"token,omitempty"`
}

const OneWeek = time.Hour * 24 * 7

var (
	Sessions = make(map[string]*Session)
)

func GetSessionFromToken(token string) (*Session, error) {
	session, ok := Sessions[token]
	if !ok {
		return nil, errors.New("session for this token does not exist")
	}
	return session, nil
}

func GetSessionFromRequest(r *http.Request) (*Session, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, errors.New("missing or invalid authorization header")
	}
	return GetSessionFromToken(strings.TrimPrefix(auth, "Bearer "))
}

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := GetSessionFromRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func SendSessionTokenJSON(w http.ResponseWriter, token string) error {
	w.Header().Set("Content-Type", "application/json")

	response := struct {
		Token string `json:"token"`
	}{Token: token}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Println("failed send session token")
		return errors.New("failed send session token")
	}
	return nil
}
