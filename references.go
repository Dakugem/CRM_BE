package main

import (
	"log"
	"net/http"
)

type Product struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ReferenceItem struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// GET /api/references/products
func RefsProductsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, name FROM products ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("refs products: %v", err)
		return
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		products = append(products, p)
	}
	writeJSON(w, http.StatusOK, products)
}

// GET /api/references/statuses
func RefsStatusesHandler(w http.ResponseWriter, r *http.Request) {
	statuses := []ReferenceItem{
		{Name: "open", Label: "Открыто"},
		{Name: "in_progress", Label: "В работе"},
		{Name: "pending", Label: "Ожидание"},
		{Name: "resolved", Label: "Решено"},
		{Name: "closed", Label: "Закрыто"},
	}
	writeJSON(w, http.StatusOK, statuses)
}

// GET /api/references/priorities
func RefsPrioritiesHandler(w http.ResponseWriter, r *http.Request) {
	priorities := []ReferenceItem{
		{Name: "low", Label: "Низкий"},
		{Name: "medium", Label: "Средний"},
		{Name: "high", Label: "Высокий"},
		{Name: "critical", Label: "Критический"},
	}
	writeJSON(w, http.StatusOK, priorities)
}

// GET /api/references/roles
func RefsRolesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT name, label FROM roles ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	roles := []ReferenceItem{}
	for rows.Next() {
		var item ReferenceItem
		if err := rows.Scan(&item.Name, &item.Label); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		roles = append(roles, item)
	}
	writeJSON(w, http.StatusOK, roles)
}
