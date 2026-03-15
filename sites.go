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

type Site struct {
	ID        int64     `json:"id"`
	ClientID  int64     `json:"client_id"`
	ProductID *int64    `json:"product_id"`
	Name      string    `json:"name"`
	Address   *string   `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SiteEquipmentItem struct {
	SiteID      int64 `json:"site_id"`
	EquipmentID int64 `json:"equipment_id"`
	Quantity    int   `json:"quantity"`
}

func scanSite(row interface{ Scan(...interface{}) error }) (*Site, error) {
	var s Site
	var productID sql.NullInt64
	var address sql.NullString
	err := row.Scan(&s.ID, &s.ClientID, &productID, &s.Name, &address, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.ProductID = nullInt64(productID)
	s.Address = nullStr(address)
	return &s, nil
}

func getSiteByID(id int64) (*Site, error) {
	row := db.QueryRow(`SELECT id, client_id, product_id, name, address, created_at, updated_at FROM sites WHERE id = $1`, id)
	s, err := scanSite(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// GET /api/sites
func SitesListHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	where := []string{}
	args := []interface{}{}
	i := 1

	if v := q.Get("client_id"); v != "" {
		where = append(where, fmt.Sprintf("client_id = $%d", i))
		args = append(args, v)
		i++
	}
	if v := q.Get("product_id"); v != "" {
		where = append(where, fmt.Sprintf("product_id = $%d", i))
		args = append(args, v)
		i++
	}

	query := `SELECT id, client_id, product_id, name, address, created_at, updated_at FROM sites`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY id"

	rows, err := db.Query(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("sites list: %v", err)
		return
	}
	defer rows.Close()

	sites := []Site{}
	for rows.Next() {
		s, err := scanSite(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		sites = append(sites, *s)
	}
	writeJSON(w, http.StatusOK, sites)
}

// POST /api/sites
func SitesCreateHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientID  int64  `json:"client_id"`
		ProductID *int64 `json:"product_id"`
		Name      string `json:"name"`
		Address   string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.ClientID == 0 || body.Name == "" {
		writeError(w, http.StatusBadRequest, "client_id and name are required")
		return
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO sites (client_id, product_id, name, address)
		VALUES ($1, $2, $3, NULLIF($4,''))
		RETURNING id`,
		body.ClientID, body.ProductID, body.Name, body.Address,
	).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create site")
		log.Printf("sites create: %v", err)
		return
	}

	s, _ := getSiteByID(id)
	writeJSON(w, http.StatusCreated, s)
}

// GET /api/sites/:id
func SitesGetHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s, err := getSiteByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if s == nil {
		writeError(w, http.StatusNotFound, "site not found")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// PATCH /api/sites/:id
func SitesUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		ClientID  *int64  `json:"client_id"`
		ProductID *int64  `json:"product_id"`
		Name      *string `json:"name"`
		Address   *string `json:"address"`
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

	if body.ClientID != nil {
		add("client_id", *body.ClientID)
	}
	if body.ProductID != nil {
		add("product_id", *body.ProductID)
	}
	if body.Name != nil {
		add("name", *body.Name)
	}
	if body.Address != nil {
		add("address", *body.Address)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE sites SET %s WHERE id = $%d", strings.Join(sets, ", "), i)

	res, err := db.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("sites update: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "site not found")
		return
	}

	s, _ := getSiteByID(id)
	writeJSON(w, http.StatusOK, s)
}

// DELETE /api/sites/:id
func SitesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := db.Exec(`DELETE FROM sites WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "site not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Site Equipment ───────────────────────────────────────────────────────────

// GET /api/sites/:id/equipment
func SitesEquipmentListHandler(w http.ResponseWriter, r *http.Request) {
	siteID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := db.Query(`SELECT site_id, equipment_id, quantity FROM site_equipment WHERE site_id = $1`, siteID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	items := []SiteEquipmentItem{}
	for rows.Next() {
		var item SiteEquipmentItem
		if err := rows.Scan(&item.SiteID, &item.EquipmentID, &item.Quantity); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

// POST /api/sites/:id/equipment
func SitesEquipmentAddHandler(w http.ResponseWriter, r *http.Request) {
	siteID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		EquipmentID int64 `json:"equipment_id"`
		Quantity    int   `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.EquipmentID == 0 {
		writeError(w, http.StatusBadRequest, "equipment_id is required")
		return
	}
	if body.Quantity <= 0 {
		body.Quantity = 1
	}

	_, err = db.Exec(`
		INSERT INTO site_equipment (site_id, equipment_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (site_id, equipment_id) DO UPDATE SET quantity = EXCLUDED.quantity`,
		siteID, body.EquipmentID, body.Quantity,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("site equipment add: %v", err)
		return
	}

	item := SiteEquipmentItem{SiteID: siteID, EquipmentID: body.EquipmentID, Quantity: body.Quantity}
	writeJSON(w, http.StatusCreated, item)
}

// PATCH /api/sites/:id/equipment/:deviceId
func SitesEquipmentUpdateHandler(w http.ResponseWriter, r *http.Request) {
	siteID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	deviceID, err := pathInt(r, "deviceId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "quantity must be > 0")
		return
	}

	res, err := db.Exec(`UPDATE site_equipment SET quantity = $1 WHERE site_id = $2 AND equipment_id = $3`,
		body.Quantity, siteID, deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "equipment not found on this site")
		return
	}

	item := SiteEquipmentItem{SiteID: siteID, EquipmentID: deviceID, Quantity: body.Quantity}
	writeJSON(w, http.StatusOK, item)
}

// DELETE /api/sites/:id/equipment/:deviceId
func SitesEquipmentRemoveHandler(w http.ResponseWriter, r *http.Request) {
	siteID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	deviceID, err := pathInt(r, "deviceId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := db.Exec(`DELETE FROM site_equipment WHERE site_id = $1 AND equipment_id = $2`, siteID, deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "equipment not found on this site")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
