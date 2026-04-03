package handlers

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cryptowave2-0/webhosting-goapi/api"
	"github.com/Cryptowave2-0/webhosting-goapi/internal/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

var languageImages = map[string]string{
	"python": "python:3.11-alpine",
	"bash":   "bash:5-alpine",
	"nodejs": "node:20-alpine",
	"js":     "node:20-alpine",
}

// UploadScriptHandler — POST /scripts/upload
// Form fields: name (required), language (required), description, entrypoint
// Files: file=@script.py  ou  file=@project.zip  ou plusieurs file=@...
func UploadScriptHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)

	r.ParseMultipartForm(50 << 20) // 50MB max

	name := r.FormValue("name")
	description := r.FormValue("description")
	language := r.FormValue("language")
	entrypoint := r.FormValue("entrypoint")

	if name == "" || language == "" {
		api.RequestErrorHandler(w, fmt.Errorf("name and language are required"))
		return
	}
	if _, ok := languageImages[language]; !ok {
		api.RequestErrorHandler(w, fmt.Errorf("unsupported language: %s (supported: python, bash, nodejs, js)", language))
		return
	}

	dockerImage := languageImages[language]
	scriptID := uuid.New().String()
	dirPath := filepath.Join("data", "scripts", scriptID)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		api.InternalErrorHandler(w)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		os.RemoveAll(dirPath)
		api.RequestErrorHandler(w, fmt.Errorf("at least one file is required"))
		return
	}

	var extractedFiles []string

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			os.RemoveAll(dirPath)
			api.InternalErrorHandler(w)
			return
		}
		defer file.Close()

		filename := fileHeader.Filename

		if strings.HasSuffix(filename, ".zip") {
			buf := new(bytes.Buffer)
			size, _ := io.Copy(buf, file)
			extracted, err := extractZip(buf.Bytes(), size, dirPath)
			if err != nil {
				os.RemoveAll(dirPath)
				api.RequestErrorHandler(w, fmt.Errorf("failed to extract zip: %v", err))
				return
			}
			extractedFiles = append(extractedFiles, extracted...)

		} else if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz") {
			extracted, err := extractTarGz(file, dirPath)
			if err != nil {
				os.RemoveAll(dirPath)
				api.RequestErrorHandler(w, fmt.Errorf("failed to extract tar.gz: %v", err))
				return
			}
			extractedFiles = append(extractedFiles, extracted...)

		} else {
			destPath := filepath.Join(dirPath, filename)
			os.MkdirAll(filepath.Dir(destPath), 0755)
			dst, err := os.Create(destPath)
			if err != nil {
				os.RemoveAll(dirPath)
				api.InternalErrorHandler(w)
				return
			}
			io.Copy(dst, file)
			dst.Close()
			rel, _ := filepath.Rel(dirPath, destPath)
			extractedFiles = append(extractedFiles, rel)
		}
	}

	// Déduire l'entrypoint si non fourni et un seul fichier
	if entrypoint == "" && len(extractedFiles) == 1 {
		entrypoint = extractedFiles[0]
	}

	_, err := db.Exec(
		`INSERT INTO scripts (id, user_id, name, description, language, docker_image, file_path, entrypoint)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		scriptID, userID, name, description, language, dockerImage, dirPath, entrypoint,
	)
	if err != nil {
		os.RemoveAll(dirPath)
		api.InternalErrorHandler(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         scriptID,
		"entrypoint": entrypoint,
		"files":      extractedFiles,
		"message":    "Script uploaded successfully",
	})
}

func extractZip(data []byte, size int64, destDir string) ([]string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), size)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range r.File {
		if strings.Contains(f.Name, "..") {
			continue
		}
		destPath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(destPath), 0755)
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		dst, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return nil, err
		}
		io.Copy(dst, rc)
		dst.Close()
		rc.Close()
		rel, _ := filepath.Rel(destDir, destPath)
		files = append(files, filepath.ToSlash(rel))
	}
	return files, nil
}

func extractTarGz(r io.Reader, destDir string) ([]string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var files []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.Contains(header.Name, "..") {
			continue
		}
		destPath := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(destPath, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(destPath), 0755)
			dst, err := os.Create(destPath)
			if err != nil {
				return nil, err
			}
			io.Copy(dst, tr)
			dst.Close()
			rel, _ := filepath.Rel(destDir, destPath)
			files = append(files, filepath.ToSlash(rel))
		}
	}
	return files, nil
}

type TreeEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func buildTree(dirPath string) []TreeEntry {
	var entries []TreeEntry
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dirPath, path)
		entries = append(entries, TreeEntry{
			Path: filepath.ToSlash(rel),
			Size: info.Size(),
		})
		return nil
	})
	return entries
}

func dirSize(path string) int64 {
    var total int64
    filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
        if err == nil && !info.IsDir() {
            total += info.Size()
        }
        return nil
    })
    return total
}

// ListScriptsHandler — GET /scripts
func ListScriptsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)

	rows, err := db.Query(
		`SELECT id, name, description, language, docker_image, entrypoint, created_at, file_path FROM scripts WHERE user_id = ? ORDER BY created_at DESC`,
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
		Entrypoint  string `json:"entrypoint"`
		CreatedAt   string `json:"created_at"`
		SizeBytes   int64  `json:"size_bytes"`
	}

	scripts := []ScriptRow{}
	for rows.Next() {
		var s ScriptRow
		var dirPath string
		rows.Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.DockerImage, &s.Entrypoint, &s.CreatedAt, &dirPath)
		s.SizeBytes = dirSize(dirPath)
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
		ID          string      `json:"id"`
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Language    string      `json:"language"`
		DockerImage string      `json:"docker_image"`
		Entrypoint  string      `json:"entrypoint"`
		CreatedAt   string      `json:"created_at"`
		Tree        []TreeEntry `json:"tree"`
	}

	var s ScriptDetail
	var dirPath string
	err := db.QueryRow(
		`SELECT id, name, description, language, docker_image, file_path, entrypoint, created_at FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.DockerImage, &dirPath, &s.Entrypoint, &s.CreatedAt)

	fmt.Printf("DEBUG GetScript: userID=%d scriptID=%q\n", userID, scriptID)

	if err != nil {
        fmt.Println("❌ Scan script error:", err)  // ← et ici
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	s.Tree = buildTree(dirPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

// DeleteScriptHandler — DELETE /scripts/{id}
func DeleteScriptHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	var dirPath string
	err := db.QueryRow(
		`SELECT file_path FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&dirPath)

	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	os.RemoveAll(dirPath)
	db.Exec(`DELETE FROM scripts WHERE id = ?`, scriptID)

	w.WriteHeader(http.StatusNoContent)
}

func GetScriptExecutionsHandler(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value(middleware.UserIDKey).(int)
    scriptID := chi.URLParam(r, "id")

    type Execution struct {
        ID         string `json:"id"`
        Status     string `json:"status"`
        ExitCode   *int   `json:"exit_code"`
        StartedAt  string `json:"started_at"`
        FinishedAt *string `json:"finished_at"`
    }

    rows, err := db.Query(
        `SELECT id, status, exit_code, started_at, finished_at 
         FROM executions WHERE script_id = ? AND user_id = ?
         ORDER BY started_at DESC`,
        scriptID, userID,
    )
    if err != nil {
        http.Error(w, "Erreur serveur", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    executions := []Execution{}
    for rows.Next() {
        var e Execution
        err := rows.Scan(&e.ID, &e.Status, &e.ExitCode, &e.StartedAt, &e.FinishedAt)
        if err != nil {
            http.Error(w, "Erreur lecture", http.StatusInternalServerError)
            return
        }
        executions = append(executions, e)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(executions)
}