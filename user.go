package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"
)

type User struct {
	ID       int64
	Username string
	Password string
}

func GenerateSessionToken() (string, error) {
	const length = 10
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateUser(user *User) (int64, error) {
	var err error
	user.Password = HashPassword(user.Password)

	var query = `INSERT INTO users (username, password) values ($1, $2)`

	result, err := db.Exec(query, user.Username, user.Password)
	if err != nil {
		return -1, fmt.Errorf("failed to exec query: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("failed to retrieve last insert id: %w", err)
	}

	return id, nil
}

func GetUserByUsername(username string, user *User) error {
	var query = "SELECT id, username, password FROM users WHERE username = $1"
	return db.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.Password)
}

func UserSigninHandler(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Enter credentials"`)
		http.Error(w, "empty username or password", http.StatusUnauthorized)
		log.Printf("empty username or password")
		return
	}

	var user User
	if err := GetUserByUsername(username, &user); err != nil {
		log.Printf("failed to get user: %v", err)
		http.Error(w, "failed to get user", http.StatusUnauthorized)
		return
	}

	if HashPassword(password) != user.Password {
		log.Println("failed qi passwords")
		http.Error(w, "failed qi passwords", http.StatusUnauthorized)
		return
	}

	token, err := GenerateSessionToken()
	if err != nil {
		log.Fatal("failed get generate token: %w", err)
		return
	}
	expiry := time.Now().Add(OneWeek)

	session := &Session{
		ID:     user.ID,
		Expiry: expiry,
		User:   user,
	}

	Sessions[token] = session

	if err = SendSessionTokenJSON(w, token); err != nil {
		log.Fatal("failed send token: %w", err)
		return
	}
}
