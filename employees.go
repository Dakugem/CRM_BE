package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Employee struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Email        *string   `json:"email"`
	Phone        *string   `json:"phone"`
	Role         string    `json:"role"`
	PhotoURL     *string   `json:"photo_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func scanEmployee(row interface {
	Scan(...interface{}) error
}) (*Employee, error) {
	var emp Employee
	var email, phone, photoURL sql.NullString
	err := row.Scan(
		&emp.ID, &emp.Username, &emp.PasswordHash,
		&emp.FullName, &email, &phone, &emp.Role, &photoURL,
		&emp.CreatedAt, &emp.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	emp.Email = nullStr(email)
	emp.Phone = nullStr(phone)
	emp.PhotoURL = nullStr(photoURL)
	return &emp, nil
}

const employeeSelect = `SELECT id, username, password_hash, full_name, email, phone, role, photo_url, created_at, updated_at FROM employees`

func getEmployeeByUsername(username string) (*Employee, error) {
	row := db.QueryRow(employeeSelect+` WHERE username = $1`, username)
	return scanEmployee(row)
}

func getEmployeeByID(id int64) (*Employee, error) {
	row := db.QueryRow(employeeSelect+` WHERE id = $1`, id)
	emp, err := scanEmployee(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return emp, err
}

// GET /api/employees
func EmployeesListHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(employeeSelect + ` ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("employees list: %v", err)
		return
	}
	defer rows.Close()

	employees := []Employee{}
	for rows.Next() {
		emp, err := scanEmployee(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		employees = append(employees, *emp)
	}
	writeJSON(w, http.StatusOK, employees)
}

// POST /api/employees
func EmployeesCreateHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if body.Role == "" {
		body.Role = "employee"
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO employees (username, password_hash, full_name, email, phone, role)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6)
		RETURNING id`,
		body.Username, HashPassword(body.Password), body.FullName,
		body.Email, body.Phone, body.Role,
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "username already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create employee")
		log.Printf("employees create: %v", err)
		return
	}

	emp, _ := getEmployeeByID(id)
	writeJSON(w, http.StatusCreated, emp)
	log.Printf("employee created: id=%d username=%s", id, body.Username)
}

// GET /api/employees/:id
func EmployeesGetHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	emp, err := getEmployeeByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if emp == nil {
		writeError(w, http.StatusNotFound, "employee not found")
		return
	}
	writeJSON(w, http.StatusOK, emp)
}

// PATCH /api/employees/:id
func EmployeesUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		FullName *string `json:"full_name"`
		Email    *string `json:"email"`
		Phone    *string `json:"phone"`
		Role     *string `json:"role"`
		Password *string `json:"password"`
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
	if body.Role != nil {
		add("role", *body.Role)
	}
	if body.Password != nil {
		add("password_hash", HashPassword(*body.Password))
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE employees SET %s WHERE id = $%d",
		strings.Join(sets, ", "), i)

	res, err := db.Exec(query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		log.Printf("employees update: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "employee not found")
		return
	}

	emp, _ := getEmployeeByID(id)
	writeJSON(w, http.StatusOK, emp)
}

// DELETE /api/employees/:id
func EmployeesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := db.Exec(`DELETE FROM employees WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "employee not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/employees/:id/photo
func EmployeesPhotoHandler(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form (max 10MB)")
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "photo field is required")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		writeError(w, http.StatusBadRequest, "unsupported image format")
		return
	}

	filename := fmt.Sprintf("employee-%d-%d%s", id, time.Now().UnixNano(), ext)
	dst, err := os.Create(filepath.Join("uploads", filename))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save photo")
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save photo")
		return
	}

	photoURL := "/uploads/" + filename
	_, err = db.Exec(`UPDATE employees SET photo_url = $1, updated_at = NOW() WHERE id = $2`, photoURL, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	emp, _ := getEmployeeByID(id)
	if emp == nil {
		writeError(w, http.StatusNotFound, "employee not found")
		return
	}
	writeJSON(w, http.StatusOK, emp)
}
