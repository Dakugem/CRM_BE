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

type Dashboard struct {
	ID         int64           `json:"id"`
	EmployeeID int64           `json:"employee_id"`
	Name       string          `json:"name"`
	Filters    json.RawMessage `json:"filters"`
	Position   int             `json:"position"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func scanDashboard(row interface{ Scan(...interface{}) error }) (*Dashboard, error) {
	var d Dashboard
	var filters []byte
	err := row.Scan(&d.ID, &d.EmployeeID, &d.Name, &filters, &d.Position, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(filters) > 0 {
		d.Filters = json.RawMessage(filters)
	} else {
		d.Filters = json.RawMessage(`{}`)
	}
	return &d, nil
}

func getDashboardByID(id, employeeID int64) (*Dashboard, error) {
	row := db.QueryRow(`
		SELECT id, employee_id, name, filters, position, created_at, updated_at
		FROM dashboards WHERE id = $1 AND employee_id = $2`, id, employeeID)
	d, err := scanDashboard(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

// GET /api/dashboards
func DashboardsListHandler(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())

	rows, err := db.Query(`
		SELECT id, employee_id, name, filters, position, created_at, updated_at
		FROM dashboards WHERE employee_id = $1 ORDER BY position, id`, session.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("dashboards list: %v", err)
		return
	}
	defer rows.Close()

	dashboards := []Dashboard{}
	for rows.Next() {
		d, err := scanDashboard(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		dashboards = append(dashboards, *d)
	}
	writeJSON(w, http.StatusOK, dashboards)
}

// POST /api/dashboards
func DashboardsCreateHandler(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())

	var body struct {
		Name    string          `json:"name"`
		Filters json.RawMessage `json:"filters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(body.Filters) == 0 {
		body.Filters = json.RawMessage(`{}`)
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO dashboards (employee_id, name, filters)
		VALUES ($1, $2, $3) RETURNING id`,
		session.AccountID, body.Name, []byte(body.Filters),
	).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create dashboard")
		log.Printf("dashboards create: %v", err)
		return
	}

	d, _ := getDashboardByID(id, session.AccountID)
	writeJSON(w, http.StatusCreated, d)
}

// GET /api/dashboards/:id
func DashboardsGetHandler(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())

	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	d, err := getDashboardByID(id, session.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if d == nil {
		writeError(w, http.StatusNotFound, "dashboard not found")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// PATCH /api/dashboards/:id
func DashboardsUpdateHandler(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())

	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Name    *string         `json:"name"`
		Filters json.RawMessage `json:"filters"`
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

	if body.Name != nil {
		add("name", *body.Name)
	}
	if len(body.Filters) > 0 {
		add("filters", []byte(body.Filters))
	}

	args = append(args, id, session.AccountID)
	query := fmt.Sprintf("UPDATE dashboards SET %s WHERE id = $%d AND employee_id = $%d",
		strings.Join(sets, ", "), i, i+1)

	res, err := db.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("dashboards update: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "dashboard not found")
		return
	}

	d, _ := getDashboardByID(id, session.AccountID)
	writeJSON(w, http.StatusOK, d)
}

// DELETE /api/dashboards/:id
func DashboardsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())

	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := db.Exec(`DELETE FROM dashboards WHERE id = $1 AND employee_id = $2`, id, session.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "dashboard not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/dashboards/reorder
func DashboardsReorderHandler(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())

	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback()

	for pos, id := range body.IDs {
		_, err := tx.Exec(`UPDATE dashboards SET position = $1 WHERE id = $2 AND employee_id = $3`,
			pos, id, session.AccountID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "reordered"})
}
