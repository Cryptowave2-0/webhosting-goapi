package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"os"

	"github.com/Cryptowave2-0/webhosting-goapi/internal/middleware"
	"github.com/go-chi/chi"
)

// ScriptSettings représente tous les paramètres modifiables d'un script
type ScriptSettings struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Language      string  `json:"language"`
	DockerImage   string  `json:"docker_image"`
	Entrypoint    string  `json:"entrypoint"`
	GithubURL     string  `json:"github_url"`
	GithubToken   string  `json:"github_token,omitempty"` // jamais renvoyé en clair, juste "set" ou ""
	GithubTokenSet bool   `json:"github_token_set"`
	AutoUpdate    bool    `json:"auto_update"`
	AutoRestart   bool    `json:"auto_restart"`
	LastCommitSHA string  `json:"last_commit_sha"`
	LastPulledAt  *string `json:"last_pulled_at"`
}

// GetSettingsHandler — GET /scripts/{id}/settings
func GetSettingsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	var s ScriptSettings
	var githubToken string
	var autoUpdate, autoRestart int

	err := db.QueryRow(`
		SELECT id, name, description, language, docker_image, entrypoint,
		       COALESCE(github_url, ''), COALESCE(github_token, ''),
		       COALESCE(auto_update, 0), COALESCE(auto_restart, 0),
		       COALESCE(last_commit_sha, ''), last_pulled_at
		FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(
		&s.ID, &s.Name, &s.Description, &s.Language, &s.DockerImage, &s.Entrypoint,
		&s.GithubURL, &githubToken,
		&autoUpdate, &autoRestart,
		&s.LastCommitSHA, &s.LastPulledAt,
	)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	s.AutoUpdate = autoUpdate == 1
	s.AutoRestart = autoRestart == 1
	s.GithubTokenSet = githubToken != ""
	s.GithubToken = "" // ne jamais renvoyer le token en clair

	// Récupérer la liste des fichiers pour le sélecteur d'entrypoint
	var dirPath string
	db.QueryRow(`SELECT file_path FROM scripts WHERE id = ?`, scriptID).Scan(&dirPath)
	tree := buildTree(dirPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"settings": s,
		"files":    tree,
	})
}

// UpdateSettingsHandler — PATCH /scripts/{id}/settings
func UpdateSettingsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	// Vérifier ownership
	var currentLang, dirPath string
	err := db.QueryRow(
		`SELECT language, file_path FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&currentLang, &dirPath)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Language    *string `json:"language"`
		Entrypoint  *string `json:"entrypoint"`
		GithubURL   *string `json:"github_url"`
		GithubToken *string `json:"github_token"` // null = ne pas changer, "" = supprimer
		AutoUpdate  *bool   `json:"auto_update"`
		AutoRestart *bool   `json:"auto_restart"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	// Construire la requête dynamiquement
	setClauses := []string{}
	args := []interface{}{}

	if body.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *body.Name)
	}
	if body.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *body.Description)
	}
	if body.Language != nil {
		if img, ok := languageImages[*body.Language]; ok {
			setClauses = append(setClauses, "language = ?", )
			args = append(args, *body.Language)
			setClauses = append(setClauses, "docker_image = ?")
			args = append(args, img)
		} else {
			http.Error(w, fmt.Sprintf("Unsupported language: %s", *body.Language), http.StatusBadRequest)
			return
		}
	}
	if body.Entrypoint != nil {
		// Vérifier que le fichier existe dans le projet
		abs, err := resolveFilePath(dirPath, *body.Entrypoint)
		if err != nil {
			http.Error(w, "Invalid entrypoint path", http.StatusBadRequest)
			return
		}
		if !fileExists(abs) {
			http.Error(w, "Entrypoint file does not exist in project", http.StatusBadRequest)
			return
		}
		setClauses = append(setClauses, "entrypoint = ?")
		args = append(args, *body.Entrypoint)
	}
	if body.GithubURL != nil {
		setClauses = append(setClauses, "github_url = ?")
		args = append(args, *body.GithubURL)
	}
	if body.GithubToken != nil {
		setClauses = append(setClauses, "github_token = ?")
		args = append(args, *body.GithubToken)
	}
	if body.AutoUpdate != nil {
		v := 0
		if *body.AutoUpdate { v = 1 }
		setClauses = append(setClauses, "auto_update = ?")
		args = append(args, v)
	}
	if body.AutoRestart != nil {
		v := 0
		if *body.AutoRestart { v = 1 }
		setClauses = append(setClauses, "auto_restart = ?")
		args = append(args, v)
	}

	if len(setClauses) == 0 {
		http.Error(w, "Nothing to update", http.StatusBadRequest)
		return
	}

	query := "UPDATE scripts SET " + strings.Join(setClauses, ", ") + " WHERE id = ? AND user_id = ?"
	args = append(args, scriptID, userID)

	if _, err := db.Exec(query, args...); err != nil {
		fmt.Println("UpdateSettings error:", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Settings updated"})
}

// fileExists vérifie si un chemin pointe vers un fichier existant
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
