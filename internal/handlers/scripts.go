package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Cryptowave2-0/webhosting-goapi/api"
	"github.com/Cryptowave2-0/webhosting-goapi/internal/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

// Map langage -> image Docker
var languageImages = map[string]string{
	"python": "python:3.11-alpine",
	"bash":   "bash:5-alpine",
	"nodejs": "node:20-alpine",
	"js":     "node:20-alpine",
}

// Map langage -> extension fichier
var languageExtensions = map[string]string{
	"python": ".py",
	"bash":   ".sh",
	"nodejs": ".js",
	"js":     ".js",
}

// UploadScriptHandler — POST /scripts/upload
// multipart/form-data : name, description, language, file
func UploadScriptHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)

	// Limite à 10MB
	r.ParseMultipartForm(10 << 20)

	name := r.FormValue("name")
	description := r.FormValue("description")
	language := r.FormValue("language")

	if name == "" || language == "" {
		api.RequestErrorHandler(w, fmt.Errorf("name and language are required"))
		return
	}

	dockerImage, ok := languageImages[language]
	if !ok {
		api.RequestErrorHandler(w, fmt.Errorf("unsupported language: %s (supported: python, bash, nodejs, js)", language))
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		api.RequestErrorHandler(w, fmt.Errorf("file is required"))
		return
	}
	defer file.Close()

	// Créer le dossier du script
	scriptID := uuid.New().String()
	ext := languageExtensions[language]
	dirPath := filepath.Join("data", "scripts", scriptID)
	filePath := filepath.Join(dirPath, "script"+ext)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		api.InternalErrorHandler(w)
		return
	}

	// Écrire le fichier sur disque
	dst, err := os.Create(filePath)
	if err != nil {
		api.InternalErrorHandler(w)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	// Insérer en base
	_, err = db.Exec(
		`INSERT INTO scripts (id, user_id, name, description, language, docker_image, file_path)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		scriptID, userID, name, description, language, dockerImage, filePath,
	)
	if err != nil {
		os.RemoveAll(dirPath) // rollback fichier
		api.InternalErrorHandler(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":      scriptID,
		"message": "Script uploaded successfully",
	})
}

// ListScriptsHandler — GET /scripts
func ListScriptsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)

	rows, err := db.Query(
		`SELECT id, name, description, language, docker_image, created_at FROM scripts WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		api.InternalErrorHandler(w)
		return
	}
	defer rows.Close()

	type ScriptRow struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Language    string `json:"language"`
		DockerImage string `json:"docker_image"`
		CreatedAt   string `json:"created_at"`
	}

	scripts := []ScriptRow{}
	for rows.Next() {
		var s ScriptRow
		rows.Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.DockerImage, &s.CreatedAt)
		scripts = append(scripts, s)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scripts)
}

// GetScriptHandler — GET /scripts/{id}
func GetScriptHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	type ScriptDetail struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Language    string `json:"language"`
		DockerImage string `json:"docker_image"`
		FilePath    string `json:"file_path"`
		CreatedAt   string `json:"created_at"`
	}

	var s ScriptDetail
	err := db.QueryRow(
		`SELECT id, name, description, language, docker_image, file_path, created_at FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.DockerImage, &s.FilePath, &s.CreatedAt)

	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

// DeleteScriptHandler — DELETE /scripts/{id}
func DeleteScriptHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	var filePath string
	err := db.QueryRow(
		`SELECT file_path FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&filePath)

	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	// Supprimer le dossier sur disque
	dirPath := filepath.Dir(filePath)
	os.RemoveAll(dirPath)

	// Supprimer en base (cascade supprimera aussi executions + logs)
	db.Exec(`DELETE FROM scripts WHERE id = ?`, scriptID)

	w.WriteHeader(http.StatusNoContent)
}
