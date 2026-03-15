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

type Client struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	INN       *string   `json:"inn"`
	Email     *string   `json:"email"`
	Phone     *string   `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ClientRepresentative struct {
	ID        int64     `json:"id"`
	ClientID  int64     `json:"client_id"`
	FullName  string    `json:"full_name"`
	Email     *string   `json:"email"`
	Phone     *string   `json:"phone"`
	Position  *string   `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func scanClient(row interface{ Scan(...interface{}) error }) (*Client, error) {
	var c Client
	var inn, email, phone sql.NullString
	err := row.Scan(&c.ID, &c.Name, &inn, &email, &phone, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.INN = nullStr(inn)
	c.Email = nullStr(email)
	c.Phone = nullStr(phone)
	return &c, nil
}

func getClientByID(id int64) (*Client, error) {
	row := db.QueryRow(`SELECT id, name, inn, email, phone, created_at, updated_at FROM clients WHERE id = $1`, id)
	c, err := scanClient(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// GET /api/clients
func ClientsListHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, name, inn, email, phone, created_at, updated_at FROM clients ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("clients list: %v", err)
		return
	}
	defer rows.Close()

	clients := []Client{}
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		clients = append(clients, *c)
	}
	writeJSON(w, http.StatusOK, clients)
}

// POST /api/clients
func ClientsCreateHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		INN   string `json:"inn"`
		Email string `json:"email"`
		Phone string `json:"phone"`
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
		INSERT INTO clients (name, inn, email, phone)
		VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''))
		RETURNING id`,
		body.Name, body.INN, body.Email, body.Phone,
	).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create client")
		log.Printf("clients create: %v", err)
		return
	}

	c, _ := getClientByID(id)
	writeJSON(w, http.StatusCreated, c)
}

// GET /api/clients/:id
func ClientsGetHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := getClientByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// PATCH /api/clients/:id
func ClientsUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Name  *string `json:"name"`
		INN   *string `json:"inn"`
		Email *string `json:"email"`
		Phone *string `json:"phone"`
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
	if body.INN != nil {
		add("inn", *body.INN)
	}
	if body.Email != nil {
		add("email", *body.Email)
	}
	if body.Phone != nil {
		add("phone", *body.Phone)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE clients SET %s WHERE id = $%d", strings.Join(sets, ", "), i)

	res, err := db.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("clients update: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}

	c, _ := getClientByID(id)
	writeJSON(w, http.StatusOK, c)
}

// DELETE /api/clients/:id
func ClientsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := db.Exec(`DELETE FROM clients WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Representatives ─────────────────────────────────────────────────────────

func scanRep(row interface{ Scan(...interface{}) error }) (*ClientRepresentative, error) {
	var rep ClientRepresentative
	var email, phone, position sql.NullString
	err := row.Scan(&rep.ID, &rep.ClientID, &rep.FullName, &email, &phone, &position, &rep.CreatedAt, &rep.UpdatedAt)
	if err != nil {
		return nil, err
	}
	rep.Email = nullStr(email)
	rep.Phone = nullStr(phone)
	rep.Position = nullStr(position)
	return &rep, nil
}

func getRepByID(clientID, repID int64) (*ClientRepresentative, error) {
	row := db.QueryRow(`
		SELECT id, client_id, full_name, email, phone, position, created_at, updated_at
		FROM client_representatives WHERE id = $1 AND client_id = $2`, repID, clientID)
	rep, err := scanRep(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rep, err
}

// GET /api/clients/:id/representatives
func ClientsRepsListHandler(w http.ResponseWriter, r *http.Request) {
	clientID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := db.Query(`
		SELECT id, client_id, full_name, email, phone, position, created_at, updated_at
		FROM client_representatives WHERE client_id = $1 ORDER BY id`, clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	reps := []ClientRepresentative{}
	for rows.Next() {
		rep, err := scanRep(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		reps = append(reps, *rep)
	}
	writeJSON(w, http.StatusOK, reps)
}

// POST /api/clients/:id/representatives
func ClientsRepsCreateHandler(w http.ResponseWriter, r *http.Request) {
	clientID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Position string `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.FullName == "" {
		writeError(w, http.StatusBadRequest, "full_name is required")
		return
	}

	var id int64
	err = db.QueryRow(`
		INSERT INTO client_representatives (client_id, full_name, email, phone, position)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''))
		RETURNING id`,
		clientID, body.FullName, body.Email, body.Phone, body.Position,
	).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create representative")
		log.Printf("reps create: %v", err)
		return
	}

	rep, _ := getRepByID(clientID, id)
	writeJSON(w, http.StatusCreated, rep)
}

// GET /api/clients/:id/representatives/:repId
func ClientsRepsGetHandler(w http.ResponseWriter, r *http.Request) {
	clientID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	repID, err := pathInt(r, "repId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rep, err := getRepByID(clientID, repID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if rep == nil {
		writeError(w, http.StatusNotFound, "representative not found")
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// PATCH /api/clients/:id/representatives/:repId
func ClientsRepsUpdateHandler(w http.ResponseWriter, r *http.Request) {
	clientID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	repID, err := pathInt(r, "repId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		FullName *string `json:"full_name"`
		Email    *string `json:"email"`
		Phone    *string `json:"phone"`
		Position *string `json:"position"`
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

	if body.FullName != nil {
		add("full_name", *body.FullName)
	}
	if body.Email != nil {
		add("email", *body.Email)
	}
	if body.Phone != nil {
		add("phone", *body.Phone)
	}
	if body.Position != nil {
		add("position", *body.Position)
	}

	args = append(args, repID, clientID)
	query := fmt.Sprintf("UPDATE client_representatives SET %s WHERE id = $%d AND client_id = $%d",
		strings.Join(sets, ", "), i, i+1)

	res, err := db.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "representative not found")
		return
	}

	rep, _ := getRepByID(clientID, repID)
	writeJSON(w, http.StatusOK, rep)
}

// DELETE /api/clients/:id/representatives/:repId
func ClientsRepsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	clientID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	repID, err := pathInt(r, "repId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := db.Exec(`DELETE FROM client_representatives WHERE id = $1 AND client_id = $2`, repID, clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "representative not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
