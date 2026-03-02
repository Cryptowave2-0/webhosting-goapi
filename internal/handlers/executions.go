package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cryptowave2-0/webhosting-goapi/api"
	"github.com/Cryptowave2-0/webhosting-goapi/internal/middleware"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

// RunScriptHandler — POST /scripts/{id}/run
func RunScriptHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	// Récupérer le script
	type Script struct {
		DockerImage string
		FilePath    string
		Language    string
	}
	var script Script
	err := db.QueryRow(
		`SELECT docker_image, file_path, language FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&script.DockerImage, &script.FilePath, &script.Language)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	// Créer l'exécution en base
	executionID := uuid.New().String()
	_, err = db.Exec(
		`INSERT INTO executions (id, script_id, user_id, status, started_at) VALUES (?, ?, ?, 'running', ?)`,
		executionID, scriptID, userID, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		api.InternalErrorHandler(w)
		return
	}

	// Lancer le conteneur en arrière-plan
	go runContainer(executionID, script.DockerImage, script.FilePath, script.Language)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"execution_id": executionID,
		"status":       "running",
	})
}

// runContainer lance le script dans Docker et stocke les logs
func runContainer(executionID, dockerImage, filePath, language string) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		updateExecution(executionID, "failed", -1)
		return
	}
	defer cli.Close()

	// Commande selon le langage
	ext := filepath.Ext(filePath)
	var cmd []string
	switch language {
	case "python":
		cmd = []string{"python", "/app/script" + ext}
	case "bash":
		cmd = []string{"bash", "/app/script" + ext}
	case "nodejs", "js":
		cmd = []string{"node", "/app/script" + ext}
	default:
		cmd = []string{"sh", "/app/script" + ext}
	}

	// Chemin absolu pour le bind mount
	absPath, _ := filepath.Abs(filePath)

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: dockerImage,
			Cmd:   cmd,
		},
		&container.HostConfig{
			Binds:      []string{absPath + ":/app/script" + ext + ":ro"},
			AutoRemove: false, // on veut lire les logs après
		},
		nil, nil, executionID,
	)
	if err != nil {
		updateExecution(executionID, "failed", -1)
		return
	}

	cli.ContainerStart(ctx, resp.ID, container.StartOptions{})

	// Attendre la fin
	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	var exitCode int64
	select {
	case status := <-statusCh:
		exitCode = status.StatusCode
	case <-errCh:
		exitCode = -1
	}

	// Récupérer les logs
	out, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err == nil {
		defer out.Close()
		content, _ := io.ReadAll(out)
		// Docker préfixe chaque ligne avec 8 bytes de header stream
		// On sépare stdout/stderr simplement en stockant tout
		storeLogs(executionID, "stdout", cleanDockerLogs(string(content)))
	}

	// Nettoyer le conteneur
	cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})

	status := "success"
	if exitCode != 0 {
		status = "failed"
	}
	updateExecution(executionID, status, int(exitCode))
}

// cleanDockerLogs retire les 8 bytes de header de chaque ligne Docker
func cleanDockerLogs(raw string) string {
	lines := strings.Split(raw, "\n")
	var cleaned []string
	for _, line := range lines {
		if len(line) > 8 {
			cleaned = append(cleaned, line[8:])
		} else if len(line) > 0 {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

func updateExecution(executionID, status string, exitCode int) {
	db.Exec(
		`UPDATE executions SET status = ?, exit_code = ?, finished_at = ? WHERE id = ?`,
		status, exitCode, time.Now().UTC().Format(time.RFC3339), executionID,
	)
}

func storeLogs(executionID, stream, content string) {
	logID := uuid.New().String()
	db.Exec(
		`INSERT INTO logs (id, execution_id, stream, content) VALUES (?, ?, ?, ?)`,
		logID, executionID, stream, content,
	)
}

// GetExecutionHandler — GET /executions/{id}
func GetExecutionHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	executionID := chi.URLParam(r, "id")

	type Execution struct {
		ID         string  `json:"id"`
		ScriptID   string  `json:"script_id"`
		Status     string  `json:"status"`
		ExitCode   *int    `json:"exit_code"`
		StartedAt  string  `json:"started_at"`
		FinishedAt *string `json:"finished_at"`
	}

	var e Execution
	err := db.QueryRow(
		`SELECT e.id, e.script_id, e.status, e.exit_code, e.started_at, e.finished_at
		 FROM executions e
		 JOIN scripts s ON e.script_id = s.id
		 WHERE e.id = ? AND s.user_id = ?`,
		executionID, userID,
	).Scan(&e.ID, &e.ScriptID, &e.Status, &e.ExitCode, &e.StartedAt, &e.FinishedAt)

	if err != nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(e)
}

// GetExecutionLogsHandler — GET /executions/{id}/logs
func GetExecutionLogsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	executionID := chi.URLParam(r, "id")

	// Vérifier que l'exécution appartient à l'user
	var count int
	db.QueryRow(
		`SELECT COUNT(*) FROM executions e JOIN scripts s ON e.script_id = s.id WHERE e.id = ? AND s.user_id = ?`,
		executionID, userID,
	).Scan(&count)

	if count == 0 {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	rows, err := db.Query(
		`SELECT stream, content, created_at FROM logs WHERE execution_id = ? ORDER BY created_at ASC`,
		executionID,
	)
	if err != nil {
		api.InternalErrorHandler(w)
		return
	}
	defer rows.Close()

	type LogEntry struct {
		Stream    string `json:"stream"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}

	logs := []LogEntry{}
	for rows.Next() {
		var l LogEntry
		rows.Scan(&l.Stream, &l.Content, &l.CreatedAt)
		logs = append(logs, l)
	}

	// Affichage lisible si ?format=text
	if r.URL.Query().Get("format") == "text" {
		w.Header().Set("Content-Type", "text/plain")
		for _, l := range logs {
			fmt.Fprintf(w, "[%s] %s\n", l.Stream, l.Content)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
