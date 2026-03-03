package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Cryptowave2-0/webhosting-goapi/internal/middleware"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi"
)

type sseLogEvent struct {
	Stream  string `json:"stream"`
	Content string `json:"content"`
}

type sseDoneEvent struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
}

// StreamExecutionHandler — GET /executions/{id}/stream
// SSE : envoie les logs en temps réel si en cours, ou rejoue depuis la DB si terminé
func StreamExecutionHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	executionID := chi.URLParam(r, "id")

	// Vérifier que l'exécution appartient à l'user
	type ExecInfo struct {
		Status      string
		ExitCode    *int
		ContainerID string
		ScriptID    string
	}
	var exec ExecInfo
	err := db.QueryRow(
		`SELECT e.status, e.exit_code, e.script_id
		 FROM executions e
		 JOIN scripts s ON e.script_id = s.id
		 WHERE e.id = ? AND s.user_id = ?`,
		executionID, userID,
	).Scan(&exec.Status, &exec.ExitCode, &exec.ScriptID)

	if err == sql.ErrNoRows {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Headers SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // désactive le buffering nginx si présent

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Helper pour envoyer un événement SSE
	sendEvent := func(event string, data interface{}) {
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
		flusher.Flush()
	}

	// --- CAS 1 : Script déjà terminé → rejouer les logs depuis la DB ---
	if exec.Status == "success" || exec.Status == "failed" {
		rows, err := db.Query(
			`SELECT stream, content FROM logs WHERE execution_id = ? ORDER BY created_at ASC`,
			executionID,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var stream, content string
				rows.Scan(&stream, &content)
				sendEvent("log", sseLogEvent{Stream: stream, Content: content})
			}
		}
		sendEvent("done", sseDoneEvent{Status: exec.Status, ExitCode: exec.ExitCode})
		return
	}

	// --- CAS 2 : Script en cours → stream Docker en live ---
	cli, err := client.NewClientWithOpts(
		client.WithHost("npipe:////./pipe/docker_engine"),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		sendEvent("done", sseDoneEvent{Status: "failed", ExitCode: nil})
		return
	}
	defer cli.Close()

	// Contexte annulé si le client se déconnecte
	ctx := r.Context()

	// Attendre que le conteneur existe (il peut être créé juste après le /run)
	containerID := executionID // on nomme le conteneur avec l'execution ID
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, err := cli.ContainerInspect(ctx, containerID)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}

	// Stream les logs Docker
	out, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true, // suit le conteneur en temps réel
		Timestamps: false,
	})
	if err != nil {
		// Conteneur introuvable, lire les logs depuis la DB en fallback
		streamLogsFromDB(executionID, sendEvent)
		sendEvent("done", sseDoneEvent{Status: exec.Status, ExitCode: exec.ExitCode})
		return
	}
	defer out.Close()

	// Lire le stream Docker ligne par ligne
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			// Client déconnecté
			return
		default:
			n, err := out.Read(buf)
			if n > 0 {
				// Retirer le header Docker de 8 bytes
				raw := string(buf[:n])
				lines := strings.Split(raw, "\n")
				for _, line := range lines {
					if len(line) > 8 {
						content := strings.TrimRight(line[8:], "\r")
						if content != "" {
							sendEvent("log", sseLogEvent{Stream: "stdout", Content: content})
						}
					}
				}
			}
			if err == io.EOF {
				// Conteneur terminé → récupérer le statut final
				finalStatus, finalExit := getFinalStatus(executionID)
				sendEvent("done", sseDoneEvent{Status: finalStatus, ExitCode: finalExit})
				return
			}
			if err != nil {
				sendEvent("done", sseDoneEvent{Status: "failed", ExitCode: nil})
				return
			}
		}
	}
}

// streamLogsFromDB rejoue les logs stockés en DB via SSE
func streamLogsFromDB(executionID string, sendEvent func(string, interface{})) {
	rows, err := db.Query(
		`SELECT stream, content FROM logs WHERE execution_id = ? ORDER BY created_at ASC`,
		executionID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var stream, content string
		rows.Scan(&stream, &content)
		sendEvent("log", sseLogEvent{Stream: stream, Content: content})
	}
}

// getFinalStatus attend que l'exécution soit terminée et retourne son statut
func getFinalStatus(executionID string) (string, *int) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		var exitCode *int
		err := db.QueryRow(
			`SELECT status, exit_code FROM executions WHERE id = ?`,
			executionID,
		).Scan(&status, &exitCode)
		if err == nil && (status == "success" || status == "failed") {
			return status, exitCode
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "failed", nil
}
