package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GET /api/profile
func ProfileGetHandler(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	emp, err := getEmployeeByID(session.AccountID)
	if err != nil || emp == nil {
		writeError(w, http.StatusNotFound, "employee not found")
		return
	}
	writeJSON(w, http.StatusOK, emp)
}

// PATCH /api/profile/photo
func ProfilePhotoHandler(w http.ResponseWriter, r *http.Request) {
	session := SessionFromContext(r.Context())
	id := session.AccountID

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
	writeJSON(w, http.StatusOK, emp)
}
