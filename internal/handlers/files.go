package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cryptowave2-0/webhosting-goapi/internal/middleware"
	"github.com/go-chi/chi"
)

// resolveFilePath vérifie que le chemin demandé est bien dans le dossier du script (anti path traversal)
func resolveFilePath(dirPath, relPath string) (string, error) {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(absDir, filepath.FromSlash(relPath))
	if !strings.HasPrefix(abs, absDir+string(os.PathSeparator)) && abs != absDir {
		return "", fmt.Errorf("invalid path")
	}
	return abs, nil
}

// getScriptDir récupère le file_path du script en vérifiant l'ownership
func getScriptDir(scriptID string, userID int) (string, error) {
	var dirPath string
	err := db.QueryRow(
		`SELECT file_path FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&dirPath)
	if err != nil {
		return "", fmt.Errorf("script not found")
	}
	return dirPath, nil
}

// ── GET /scripts/{id}/files/*filepath ───────────────────────────────────────
// Télécharger un fichier du projet
func DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")
	relPath := chi.URLParam(r, "*")

	dirPath, err := getScriptDir(scriptID, userID)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	absPath, err := resolveFilePath(dirPath, relPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(absPath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, absPath)
}

// ── GET /scripts/{id}/files/{path}?raw=1 ────────────────────────────────────
// Lire le contenu texte d'un fichier (pour l'éditeur)
func ReadFileHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")
	relPath := chi.URLParam(r, "*")

	dirPath, err := getScriptDir(scriptID, userID)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	absPath, err := resolveFilePath(dirPath, relPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Limite 1 MB pour l'éditeur
	if info.Size() > 1<<20 {
		http.Error(w, "File too large for editor (max 1MB)", http.StatusRequestEntityTooLarge)
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		http.Error(w, "Cannot read file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    relPath,
		"content": string(content),
		"size":    info.Size(),
	})
}

// ── PUT /scripts/{id}/files/*filepath ───────────────────────────────────────
// Sauvegarder le contenu d'un fichier (éditeur)
func WriteFileHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")
	relPath := chi.URLParam(r, "*")

	dirPath, err := getScriptDir(scriptID, userID)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	absPath, err := resolveFilePath(dirPath, relPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Vérifier que le fichier existe déjà (pas de création libre)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if err := os.WriteFile(absPath, []byte(body.Content), 0644); err != nil {
		http.Error(w, "Cannot write file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    relPath,
		"size":    len(body.Content),
		"message": "File saved",
	})
}

// ── DELETE /scripts/{id}/files/*filepath ────────────────────────────────────
// Supprimer un fichier
func DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")
	relPath := chi.URLParam(r, "*")

	dirPath, err := getScriptDir(scriptID, userID)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	absPath, err := resolveFilePath(dirPath, relPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	if info.IsDir() {
		http.Error(w, "Cannot delete a directory", http.StatusBadRequest)
		return
	}

	if err := os.Remove(absPath); err != nil {
		http.Error(w, "Cannot delete file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── GET /scripts/{id}/archive ────────────────────────────────────────────────
// Télécharger tout le projet en .zip (hors .git)
func DownloadArchiveHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	var dirPath, scriptName string
	err := db.QueryRow(
		`SELECT file_path, name FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&dirPath, &scriptName)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	zipName := strings.ReplaceAll(scriptName, " ", "_") + ".zip"
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, zipName))
	w.Header().Set("Content-Type", "application/zip")

	zw := zip.NewWriter(w)
	defer zw.Close()

	absDir, _ := filepath.Abs(dirPath)

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Exclure .git
		rel, _ := filepath.Rel(absDir, path)
		if strings.HasPrefix(rel, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		f, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		io.Copy(f, src)
		return nil
	})

	if err != nil {
		// Le zip est déjà partiellement envoyé, on ne peut pas changer le status code
		fmt.Fprintf(w, "zip error: %v", err)
	}
}

// ── POST /scripts/{id}/files/*filepath ──────────────────────────────────────
// Créer un nouveau fichier
func CreateFileHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")
	relPath := chi.URLParam(r, "*")

	dirPath, err := getScriptDir(scriptID, userID)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	absPath, err := resolveFilePath(dirPath, relPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(absPath); err == nil {
		http.Error(w, "File already exists", http.StatusConflict)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		http.Error(w, "Cannot create directory", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(absPath, []byte(body.Content), 0644); err != nil {
		http.Error(w, "Cannot create file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    relPath,
		"message": "File created",
	})
}

// ── PATCH /scripts/{id}/files/*filepath ─────────────────────────────────────
// Déplacer ou renommer un fichier
func MoveFileHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")
	relPath := chi.URLParam(r, "*")

	dirPath, err := getScriptDir(scriptID, userID)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	var body struct {
		Destination string `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Destination == "" {
		http.Error(w, "Missing destination", http.StatusBadRequest)
		return
	}

	srcAbs, err := resolveFilePath(dirPath, relPath)
	if err != nil {
		http.Error(w, "Invalid source path", http.StatusBadRequest)
		return
	}

	dstAbs, err := resolveFilePath(dirPath, body.Destination)
	if err != nil {
		http.Error(w, "Invalid destination path", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(srcAbs); os.IsNotExist(err) {
		http.Error(w, "Source file not found", http.StatusNotFound)
		return
	}

	if _, err := os.Stat(dstAbs); err == nil {
		http.Error(w, "Destination already exists", http.StatusConflict)
		return
	}

	// Créer les dossiers parents si besoin
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
		http.Error(w, "Cannot create destination directory", http.StatusInternalServerError)
		return
	}

	if err := os.Rename(srcAbs, dstAbs); err != nil {
		http.Error(w, "Move failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Si l'entrypoint pointait sur l'ancien chemin → le mettre à jour automatiquement
	var currentEntrypoint string
	db.QueryRow(`SELECT entrypoint FROM scripts WHERE id = ?`, scriptID).Scan(&currentEntrypoint)
	if currentEntrypoint == relPath {
		db.Exec(`UPDATE scripts SET entrypoint = ? WHERE id = ?`, body.Destination, scriptID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"from":    relPath,
		"to":      body.Destination,
		"message": "File moved",
	})
}
