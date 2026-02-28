package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Appeals struct {
	ID          int64     `json:"id,omitempty"`
	Type        string    `json:"type,omitempty"`
	Status      string    `json:"status,omitempty"`
	Criticality string    `json:"criticality,omitempty"`
	Product     string    `json:"product,omitempty"`
	Description string    `json:"description,omitempty"`
	Client      string    `json:"client,omitempty"`
	Venue       string    `json:"venue,omitempty"`
	Responsible string    `json:"responsible,omitempty"`
	Deadline    time.Time `json:"deadline"`
	Updated     time.Time `json:"updated"`
}

func getAllAppeals() ([]Appeals, error) {
	var appeals []Appeals

	query := `SELECT id, type, status, criticality, product, description, client, venue, responsible, deadline, updated FROM appeals ORDER BY updated DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var a Appeals
		err := rows.Scan(&a.ID, &a.Type, &a.Status, &a.Criticality, &a.Product, &a.Description, &a.Client, &a.Venue, &a.Responsible, &a.Deadline, &a.Updated)
		if err != nil {
			log.Printf("failed to scan appeal row: %v", err)
			return nil, err
		}
		appeals = append(appeals, a)
	}

	return appeals, nil
}

func createAppeal(a *Appeals) (int64, error) {
	var id int64
	query := `
		INSERT INTO appeals (type, status, criticality, product, description, client, venue, responsible, deadline, updated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`

	err := db.QueryRow(query,
		a.Type, a.Status, a.Criticality, a.Product, a.Description,
		a.Client, a.Venue, a.Responsible, a.Deadline, a.Updated,
	).Scan(&id)
	if err != nil {
		return -1, fmt.Errorf("failed to insert appeal: %w", err)
	}
	return id, nil
}

func AppealsGetAllHandler(w http.ResponseWriter, r *http.Request) {
	appeals, err := getAllAppeals()
	if err != nil {
		http.Error(w, "failed to get appeals", http.StatusInternalServerError)
		log.Printf("failed to get appeals: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(appeals)

	log.Printf("get all appeals")
}

func AppealsCreateHandler(w http.ResponseWriter, r *http.Request) {
	var a Appeals
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// defaults
	a.Updated = time.Now()

	id, err := createAppeal(&a)
	if err != nil {
		http.Error(w, "failed to create appeal", http.StatusInternalServerError)
		log.Printf("failed to create appeal: %v", err)
		return
	}

	a.ID = id
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a)

	log.Printf("created new appeal: id=%d type=%q client=%q responsible=%q deadline=%s", a.ID, a.Type, a.Client, a.Responsible, a.Deadline.Format("2006-01-02"))
}
