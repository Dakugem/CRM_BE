package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type Equipment struct {
	ID          int64     `json:"id"`
	ProductID   *int64    `json:"product_id"`
	Name        string    `json:"name"`
	Model       *string   `json:"model"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func scanEquipment(row interface{ Scan(...interface{}) error }) (*Equipment, error) {
	var e Equipment
	var productID sql.NullInt64
	var model, description sql.NullString
	err := row.Scan(&e.ID, &productID, &e.Name, &model, &description, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	e.ProductID = nullInt64(productID)
	e.Model = nullStr(model)
	e.Description = nullStr(description)
	return &e, nil
}

func getEquipmentByID(id int64) (*Equipment, error) {
	row := db.QueryRow(`SELECT id, product_id, name, model, description, created_at, updated_at FROM equipment WHERE id = $1`, id)
	e, err := scanEquipment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// GET /api/equipment
func EquipmentListHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	where := []string{}
	args := []interface{}{}
	i := 1

	if v := q.Get("product_id"); v != "" {
		where = append(where, fmt.Sprintf("product_id = $%d", i))
		args = append(args, v)
		i++
	}

	query := `SELECT id, product_id, name, model, description, created_at, updated_at FROM equipment`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY id"

	rows, err := db.Query(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("equipment list: %v", err)
		return
	}
	defer rows.Close()

	list := []Equipment{}
	for rows.Next() {
		e, err := scanEquipment(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		list = append(list, *e)
	}
	writeJSON(w, http.StatusOK, list)
}

// POST /api/equipment
func EquipmentCreateHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProductID   *int64 `json:"product_id"`
		Name        string `json:"name"`
		Model       string `json:"model"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO equipment (product_id, name, model, description)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))
		RETURNING id`,
		body.ProductID, body.Name, body.Model, body.Description,
	).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create equipment")
		log.Printf("equipment create: %v", err)
		return
	}

	e, _ := getEquipmentByID(id)
	writeJSON(w, http.StatusCreated, e)
}

// GET /api/equipment/:id
func EquipmentGetHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	e, err := getEquipmentByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if e == nil {
		writeError(w, http.StatusNotFound, "equipment not found")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// PATCH /api/equipment/:id
func EquipmentUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		ProductID   *int64  `json:"product_id"`
		Name        *string `json:"name"`
		Model       *string `json:"model"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	sets := []string{"updated_at = NOW()"}
	args := []interface{}{}
	i := 1
	add := func(col string, val interface{}) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}

	if body.ProductID != nil {
		add("product_id", *body.ProductID)
	}
	if body.Name != nil {
		add("name", *body.Name)
	}
	if body.Model != nil {
		add("model", *body.Model)
	}
	if body.Description != nil {
		add("description", *body.Description)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE equipment SET %s WHERE id = $%d", strings.Join(sets, ", "), i)

	res, err := db.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("equipment update: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "equipment not found")
		return
	}

	e, _ := getEquipmentByID(id)
	writeJSON(w, http.StatusOK, e)
}

// DELETE /api/equipment/:id
func EquipmentDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := db.Exec(`DELETE FROM equipment WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "equipment not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
