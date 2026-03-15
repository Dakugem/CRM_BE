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

type Ticket struct {
	ID            int64      `json:"id"`
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	Priority      string     `json:"priority"`
	ProductID     *int64     `json:"product_id"`
	Description   string     `json:"description"`
	ClientID      *int64     `json:"client_id"`
	SiteID        *int64     `json:"site_id"`
	ResponsibleID *int64     `json:"responsible_id"`
	Deadline      *time.Time `json:"deadline"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type TicketComment struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticket_id"`
	AuthorID  *int64    `json:"author_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type TicketHistoryEntry struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticket_id"`
	AuthorID  *int64    `json:"author_id"`
	Field     string    `json:"field"`
	OldValue  *string   `json:"old_value"`
	NewValue  *string   `json:"new_value"`
	CreatedAt time.Time `json:"created_at"`
}

const ticketSelect = `SELECT id, ticket_type, status, priority, product_id, description, client_id, site_id, responsible_id, deadline, created_at, updated_at FROM tickets`

func scanTicket(row interface{ Scan(...interface{}) error }) (*Ticket, error) {
	var t Ticket
	var productID, clientID, siteID, responsibleID sql.NullInt64
	var deadline sql.NullTime
	err := row.Scan(
		&t.ID, &t.Type, &t.Status, &t.Priority,
		&productID, &t.Description,
		&clientID, &siteID, &responsibleID,
		&deadline, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	t.ProductID = nullInt64(productID)
	t.ClientID = nullInt64(clientID)
	t.SiteID = nullInt64(siteID)
	t.ResponsibleID = nullInt64(responsibleID)
	t.Deadline = nullTime(deadline)
	return &t, nil
}

func getTicketByID(id int64) (*Ticket, error) {
	row := db.QueryRow(ticketSelect+` WHERE id = $1`, id)
	t, err := scanTicket(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func addTicketHistory(ticketID int64, authorID *int64, field, oldVal, newVal string) {
	_, err := db.Exec(`
		INSERT INTO ticket_history (ticket_id, author_id, field, old_value, new_value)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''))`,
		ticketID, authorID, field, oldVal, newVal,
	)
	if err != nil {
		log.Printf("ticket history insert: %v", err)
	}
}

// GET /api/tickets
func TicketsListHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	where := []string{}
	args := []interface{}{}
	i := 1

	if v := q.Get("status"); v != "" {
		where = append(where, fmt.Sprintf("status = $%d", i))
		args = append(args, v)
		i++
	}
	if v := q.Get("priority"); v != "" {
		where = append(where, fmt.Sprintf("priority = $%d", i))
		args = append(args, v)
		i++
	}
	if v := q.Get("type"); v != "" {
		where = append(where, fmt.Sprintf("ticket_type = $%d", i))
		args = append(args, v)
		i++
	}
	if v := q.Get("client_id"); v != "" {
		where = append(where, fmt.Sprintf("client_id = $%d", i))
		args = append(args, v)
		i++
	}
	if v := q.Get("responsible_id"); v != "" {
		where = append(where, fmt.Sprintf("responsible_id = $%d", i))
		args = append(args, v)
		i++
	}

	query := ticketSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY updated_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("tickets list: %v", err)
		return
	}
	defer rows.Close()

	tickets := []Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		tickets = append(tickets, *t)
	}
	writeJSON(w, http.StatusOK, tickets)
}

// POST /api/tickets
func TicketsCreateHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type          string     `json:"type"`
		Status        string     `json:"status"`
		Priority      string     `json:"priority"`
		ProductID     *int64     `json:"product_id"`
		Description   string     `json:"description"`
		ClientID      *int64     `json:"client_id"`
		SiteID        *int64     `json:"site_id"`
		ResponsibleID *int64     `json:"responsible_id"`
		Deadline      *time.Time `json:"deadline"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if body.Type == "" {
		body.Type = "request"
	}
	if body.Status == "" {
		body.Status = "open"
	}
	if body.Priority == "" {
		body.Priority = "medium"
	}

	// Validate product_id if provided
	if body.ProductID != nil {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1)`, *body.ProductID).Scan(&exists)
		if err != nil || !exists {
			writeError(w, http.StatusBadRequest, "invalid product_id")
			return
		}
	}

	// Validate client_id if provided
	if body.ClientID != nil {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1)`, *body.ClientID).Scan(&exists)
		if err != nil || !exists {
			writeError(w, http.StatusBadRequest, "invalid client_id")
			return
		}
	}

	// Validate site_id if provided
	if body.SiteID != nil {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sites WHERE id = $1)`, *body.SiteID).Scan(&exists)
		if err != nil || !exists {
			writeError(w, http.StatusBadRequest, "invalid site_id")
			return
		}
	}

	// Validate responsible_id if provided
	if body.ResponsibleID != nil {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM employees WHERE id = $1)`, *body.ResponsibleID).Scan(&exists)
		if err != nil || !exists {
			writeError(w, http.StatusBadRequest, "invalid responsible_id")
			return
		}
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO tickets (ticket_type, status, priority, product_id, description, client_id, site_id, responsible_id, deadline)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		body.Type, body.Status, body.Priority,
		body.ProductID, body.Description,
		body.ClientID, body.SiteID, body.ResponsibleID,
		body.Deadline,
	).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create ticket")
		log.Printf("tickets create: %v", err)
		return
	}

	t, _ := getTicketByID(id)
	writeJSON(w, http.StatusCreated, t)
	log.Printf("ticket created: id=%d type=%s", id, body.Type)
}

// GET /api/tickets/:id
func TicketsGetHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	t, err := getTicketByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if t == nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// PATCH /api/tickets/:id
func TicketsUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	current, err := getTicketByID(id)
	if err != nil || current == nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	var body struct {
		Type          *string    `json:"type"`
		Status        *string    `json:"status"`
		Priority      *string    `json:"priority"`
		ProductID     *int64     `json:"product_id"`
		Description   *string    `json:"description"`
		SiteID        *int64     `json:"site_id"`
		ResponsibleID *int64     `json:"responsible_id"`
		Deadline      *time.Time `json:"deadline"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	session := SessionFromContext(r.Context())
	var authorID *int64
	if session != nil {
		authorID = &session.AccountID
	}

	sets := []string{"updated_at = NOW()"}
	args := []interface{}{}
	i := 1
	addSet := func(col string, val interface{}) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	record := func(field, oldVal, newVal string) {
		if oldVal != newVal {
			addTicketHistory(id, authorID, field, oldVal, newVal)
		}
	}

	if body.Status != nil {
		record("status", current.Status, *body.Status)
		addSet("status", *body.Status)
	}
	if body.Priority != nil {
		record("priority", current.Priority, *body.Priority)
		addSet("priority", *body.Priority)
	}
	if body.Type != nil {
		record("type", current.Type, *body.Type)
		addSet("ticket_type", *body.Type)
	}
	if body.ProductID != nil {
		addSet("product_id", *body.ProductID)
	}
	if body.Description != nil {
		addSet("description", *body.Description)
	}
	if body.SiteID != nil {
		old := ""
		if current.SiteID != nil {
			old = fmt.Sprintf("%d", *current.SiteID)
		}
		record("site_id", old, fmt.Sprintf("%d", *body.SiteID))
		addSet("site_id", *body.SiteID)
	}
	if body.ResponsibleID != nil {
		old := ""
		if current.ResponsibleID != nil {
			old = fmt.Sprintf("%d", *current.ResponsibleID)
		}
		record("responsible_id", old, fmt.Sprintf("%d", *body.ResponsibleID))
		addSet("responsible_id", *body.ResponsibleID)
	}
	if body.Deadline != nil {
		addSet("deadline", *body.Deadline)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE tickets SET %s WHERE id = $%d", strings.Join(sets, ", "), i)

	res, err := db.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("tickets update: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	t, _ := getTicketByID(id)
	writeJSON(w, http.StatusOK, t)
}

// PATCH /api/tickets/:id/assign
func TicketsAssignHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		ResponsibleID int64 `json:"responsible_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	current, _ := getTicketByID(id)
	if current == nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	session := SessionFromContext(r.Context())
	var authorID *int64
	if session != nil {
		authorID = &session.AccountID
	}

	old := ""
	if current.ResponsibleID != nil {
		old = fmt.Sprintf("%d", *current.ResponsibleID)
	}
	addTicketHistory(id, authorID, "responsible_id", old, fmt.Sprintf("%d", body.ResponsibleID))

	_, err = db.Exec(`UPDATE tickets SET responsible_id = $1, updated_at = NOW() WHERE id = $2`,
		body.ResponsibleID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	t, _ := getTicketByID(id)
	writeJSON(w, http.StatusOK, t)
}

// ── Comments ─────────────────────────────────────────────────────────────────

// GET /api/tickets/:id/comments
func TicketsCommentsListHandler(w http.ResponseWriter, r *http.Request) {
	ticketID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := db.Query(`
		SELECT id, ticket_id, author_id, text, created_at
		FROM ticket_comments WHERE ticket_id = $1 ORDER BY created_at`, ticketID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	comments := []TicketComment{}
	for rows.Next() {
		var c TicketComment
		var authorID sql.NullInt64
		if err := rows.Scan(&c.ID, &c.TicketID, &authorID, &c.Text, &c.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		c.AuthorID = nullInt64(authorID)
		comments = append(comments, c)
	}
	writeJSON(w, http.StatusOK, comments)
}

// POST /api/tickets/:id/comments
func TicketsCommentsCreateHandler(w http.ResponseWriter, r *http.Request) {
	ticketID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	session := SessionFromContext(r.Context())
	var authorID *int64
	if session != nil {
		authorID = &session.AccountID
	}

	var id int64
	err = db.QueryRow(`
		INSERT INTO ticket_comments (ticket_id, author_id, text)
		VALUES ($1, $2, $3) RETURNING id`,
		ticketID, authorID, body.Text,
	).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create comment")
		log.Printf("comment create: %v", err)
		return
	}

	comment := TicketComment{
		ID:        id,
		TicketID:  ticketID,
		AuthorID:  authorID,
		Text:      body.Text,
		CreatedAt: time.Now(),
	}
	writeJSON(w, http.StatusCreated, comment)
}

// ── History ───────────────────────────────────────────────────────────────────

// GET /api/tickets/:id/history
func TicketsHistoryListHandler(w http.ResponseWriter, r *http.Request) {
	ticketID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := db.Query(`
		SELECT id, ticket_id, author_id, field, old_value, new_value, created_at
		FROM ticket_history WHERE ticket_id = $1 ORDER BY created_at`, ticketID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	history := []TicketHistoryEntry{}
	for rows.Next() {
		var h TicketHistoryEntry
		var authorID sql.NullInt64
		var oldVal, newVal sql.NullString
		if err := rows.Scan(&h.ID, &h.TicketID, &authorID, &h.Field, &oldVal, &newVal, &h.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		h.AuthorID = nullInt64(authorID)
		h.OldValue = nullStr(oldVal)
		h.NewValue = nullStr(newVal)
		history = append(history, h)
	}
	writeJSON(w, http.StatusOK, history)
}

// ── Subtasks ──────────────────────────────────────────────────────────────────

// POST /api/tickets/:id/subtasks
func TicketsSubtasksAddHandler(w http.ResponseWriter, r *http.Request) {
	parentID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		ChildID int64 `json:"ticket_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.ChildID == 0 {
		writeError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}
	if body.ChildID == parentID {
		writeError(w, http.StatusBadRequest, "a ticket cannot be its own subtask")
		return
	}

	_, err = db.Exec(`
		INSERT INTO ticket_subtasks (parent_id, child_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`,
		parentID, body.ChildID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("subtask add: %v", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"parent_id": parentID,
		"child_id":  body.ChildID,
	})
}

// DELETE /api/tickets/:id/subtasks/:subtaskId
func TicketsSubtasksRemoveHandler(w http.ResponseWriter, r *http.Request) {
	parentID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	childID, err := pathInt(r, "subtaskId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := db.Exec(`DELETE FROM ticket_subtasks WHERE parent_id = $1 AND child_id = $2`, parentID, childID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "subtask link not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
