package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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

    "bytes"
    "github.com/docker/docker/pkg/stdcopy"
)

// RunScriptHandler — POST /scripts/{id}/run
func RunScriptHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	fmt.Println("▶ RunScript userID:", userID, "scriptID:", scriptID)

	// Récupérer le script
	type Script struct {
		DockerImage string
		FilePath    string
		Language    string
		Entrypoint  string
	}
	var script Script
	err := db.QueryRow(
		`SELECT docker_image, file_path, language, entrypoint FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&script.DockerImage, &script.FilePath, &script.Language, &script.Entrypoint)
	if err != nil {
        fmt.Println("❌ Script not found:", err)  // ← ici
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	fmt.Println("✅ Script trouvé:", script)  // ← et ici

	// Créer l'exécution en base
	executionID := uuid.New().String()
	_, err = db.Exec(
		`INSERT INTO executions (id, script_id, user_id, status, started_at) VALUES (?, ?, ?, 'running', ?)`,
		executionID, scriptID, userID, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
        fmt.Println("❌ Insert execution error:", err)  // ← et ici
		api.InternalErrorHandler(w)
		return
	}

	// Lancer le conteneur en arrière-plan
	go runContainer(executionID, script.DockerImage, script.FilePath, script.Language, script.Entrypoint)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"execution_id": executionID,
		"status":       "running",
	})
}

// runContainer lance le script dans Docker et stocke les logs
func runContainer(executionID, dockerImage, dirPath, language, entrypoint string) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		fmt.Println("Docker client error:", err)
		storeLogs(executionID, "stderr", "Docker client error: "+err.Error())
		updateExecution(executionID, "failed", -1)
		return
	}

	ping, err := cli.Ping(context.Background())
	fmt.Println("✅ Docker ping:", ping, err)  // ← et ici
	defer cli.Close()

	// Commande via l'entrypoint
	containerEntry := "/app/" + entrypoint
	var cmd []string
	switch language {
		case "python":
			// 1. On crée le dossier libs s'il n'existe pas (--parents)
			// 2. On génère le hash du requirements.txt actuel
			// 3. On compare avec l'ancien hash (.req.hash)
			// 4. Si différent, on installe dans /app/libs et on met à jour le hash
			cmd = []string{"sh", "-c", `
				mkdir -p /app/libs &&
				if [ -f requirements.txt ]; then
					md5sum requirements.txt > .tmp_hash;
					if ! diff -q .tmp_hash /app/libs/.req.hash > /dev/null 2>&1; then
						echo "📦 Installation des dépendances Python..." &&
						pip install --target=/app/libs -r requirements.txt &&
						mv .tmp_hash /app/libs/.req.hash;
					else
						echo "✅ Dépendances à jour (cache utilisé)";
						rm .tmp_hash;
					fi
				fi &&
				export PYTHONPATH=$PYTHONPATH:/app/libs &&
				python -u ` + containerEntry}

		case "nodejs", "js":
			// Pour Node.js, on stocke le hash à la racine car node_modules est déjà au bon endroit
			cmd = []string{"sh", "-c", `
				if [ -f package.json ]; then
					md5sum package.json > .tmp_hash;
					if ! diff -q .tmp_hash .pkg.hash > /dev/null 2>&1; then
						echo "📦 Installation des modules Node.js..." &&
						npm install &&
						mv .tmp_hash .pkg.hash;
					else
						echo "✅ Modules à jour (cache utilisé)";
						rm .tmp_hash;
					fi
				fi &&
				node ` + containerEntry}

		case "bash":
			// Pour Bash, on force l'exécutabilité avant de lancer
			cmd = []string{"sh", "-c", "chmod +x " + containerEntry + " && ./" + containerEntry}

		default:
			// Par défaut, on tente de lancer avec sh
			cmd = []string{"sh", containerEntry}
		}

	// Monter tout le dossier dans /app/
	absPath, _ := filepath.Abs(dirPath)

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: dockerImage,
			Cmd:   cmd,
			WorkingDir: "/app",
		},
		&container.HostConfig{
			Binds:      []string{absPath + ":/app:z"},
			NetworkMode: "bridge",
		},
		nil, nil, executionID,
	)
	if err != nil {
		fmt.Println("ContainerCreate error:", err)
		storeLogs(executionID, "stderr", "ContainerCreate error: "+err.Error())
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
		var stdout, stderr bytes.Buffer

    	stdcopy.StdCopy(&stdout, &stderr, out)
		
		if stdout.Len() > 0 {
			for _, line := range strings.Split(stdout.String(), "\n") {
				line = strings.TrimRight(line, "\r")
				if line != "" {
					storeLogs(executionID, "stdout", line)
				}
			}
		}
		if stderr.Len() > 0 {
			for _, line := range strings.Split(stderr.String(), "\n") {
				line = strings.TrimRight(line, "\r")
				if line != "" {
					storeLogs(executionID, "stderr", line)
				}
			}
		}
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