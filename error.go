package main

import "errors"

var (
	ErrUserNotFound      = errors.New("user doesn't exists")
	ErrUserAlreadyExists = errors.New("user with such username already exists")
)

/*
http.Error(w, err.Error(), http.StatusBadRequest)
fmt.Errorf("failed to exec query: %w", err)
errors.New("user doesn't exists")
*/
