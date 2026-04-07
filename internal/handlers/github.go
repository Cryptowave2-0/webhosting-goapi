package handlers

import (
	"net/http"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
	"strings"
	"context"
	
	"github.com/google/uuid"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types/container"
	"github.com/Cryptowave2-0/webhosting-goapi/internal/middleware"
	gogit "github.com/go-git/go-git/v5"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-chi/chi"
)

// ── Helpers internes ─────────────────────────────────────────────────────────

func gitAuth(token string) *githttp.BasicAuth {
	if token == "" {
		return nil
	}
	return &githttp.BasicAuth{
		Username: "x-token", // GitHub ignore le username avec un PAT
		Password: token,
	}
}

// getLatestRemoteSHA interroge l'API GitHub REST pour récupérer le SHA
// du dernier commit sur la branche par défaut, sans cloner.
// URL attendue : https://github.com/owner/repo  ou  https://github.com/owner/repo.git
func getLatestRemoteSHA(repoURL, token string) (string, error) {
	// Extraire owner/repo depuis l'URL
	clean := repoURL
	for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
		if len(clean) > len(prefix) {
			clean = clean[len(prefix):]
			break
		}
	}
	clean = strings.TrimSuffix(clean, ".git")
	// clean = "owner/repo"

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/commits?per_page=1", clean)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return "", err
	}
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found")
	}
	return commits[0].SHA, nil
}

// pullRepo effectue un git pull sur un repo déjà cloné
func pullRepo(dirPath, token string) (bool, error) {
	repo, err := gogit.PlainOpen(dirPath)
	if err != nil {
		return false, fmt.Errorf("not a git repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return false, err
	}

	err = wt.Pull(&gogit.PullOptions{
		Auth:  gitAuth(token),
		Force: false,
	})
	if err == gogit.NoErrAlreadyUpToDate {
		return false, nil // pas de changement
	}
	if err != nil {
		return false, err
	}
	return true, nil // des changements ont été tirés
}

// ── GithubCloneHandler — POST /scripts/{id}/github/clone ────────────────────
// Clone un repo GitHub dans le dossier du script (remplace les fichiers existants)
func GithubCloneHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	var dirPath, storedToken, storedURL string
	err := db.QueryRow(
		`SELECT file_path, COALESCE(github_url,''), COALESCE(github_token,'')
		 FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&dirPath, &storedURL, &storedToken)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}

	var body struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	// Priorité : body > stored
	cloneURL := storedURL
	if body.URL != "" { cloneURL = body.URL }
	token := storedToken
	if body.Token != "" { token = body.Token }

	if cloneURL == "" {
		http.Error(w, "No GitHub URL configured", http.StatusBadRequest)
		return
	}

	// Vider le dossier et recloner
	os.RemoveAll(dirPath)
	os.MkdirAll(dirPath, 0755)

	_, err = gogit.PlainClone(dirPath, false, &gogit.CloneOptions{
		URL:      cloneURL,
		Auth:     gitAuth(token),
		Progress: os.Stdout,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Clone failed: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Récupérer le SHA du commit cloné
	repo, _ := gogit.PlainOpen(dirPath)
	var sha string
	if ref, err := repo.Head(); err == nil {
		sha = ref.Hash().String()
	}

	// Sauvegarder l'URL, le token et le SHA
	db.Exec(`
		UPDATE scripts SET github_url = ?, github_token = ?, last_commit_sha = ?, last_pulled_at = ?
		WHERE id = ?`,
		cloneURL, token, sha, time.Now().UTC().Format(time.RFC3339), scriptID,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":          "Clone successful",
		"last_commit_sha":  sha,
	})
}

// ── GithubPullHandler — POST /scripts/{id}/github/pull ──────────────────────
// Pull manuel depuis GitHub
func GithubPullHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)
	scriptID := chi.URLParam(r, "id")

	var dirPath, token, githubURL, lastSHA string
	err := db.QueryRow(
		`SELECT file_path, COALESCE(github_token,''), COALESCE(github_url,''), COALESCE(last_commit_sha,'')
		 FROM scripts WHERE id = ? AND user_id = ?`,
		scriptID, userID,
	).Scan(&dirPath, &token, &githubURL, &lastSHA)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}
	if githubURL == "" {
		http.Error(w, "No GitHub URL configured", http.StatusBadRequest)
		return
	}

	changed, err := pullRepo(dirPath, token)
	if err != nil {
		http.Error(w, fmt.Sprintf("Pull failed: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Récupérer le nouveau SHA
	var newSHA string
	if repo, err := gogit.PlainOpen(dirPath); err == nil {
		if ref, err := repo.Head(); err == nil {
			newSHA = ref.Hash().String()
		}
	}

	db.Exec(
		`UPDATE scripts SET last_commit_sha = ?, last_pulled_at = ? WHERE id = ?`,
		newSHA, time.Now().UTC().Format(time.RFC3339), scriptID,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"changed":         changed,
		"last_commit_sha": newSHA,
		"message":         map[bool]string{true: "Updated", false: "Already up to date"}[changed],
	})
}

// ── Polling goroutine ────────────────────────────────────────────────────────

// StartGithubPoller démarre le poller dans une goroutine.
// À appeler une fois depuis main.go après Setup(db).
func StartGithubPoller() {
	go githubPollerLoop()
}

func githubPollerLoop() {
	for {
		now := time.Now()

		// Prochain minuit + jitter [-15min, +15min] propre à ce démarrage
		jitter := time.Duration(rand.Intn(30)-15) * time.Minute
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		next := midnight.Add(jitter)

		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}

		fmt.Printf("[github-poller] next run at %s\n", next.Format("2006-01-02 15:04:05"))
		time.Sleep(time.Until(next))

		runGithubPoll()
	}
}

func runGithubPoll() {
	fmt.Println("[github-poller] running poll...")

	rows, err := db.Query(`
		SELECT id, file_path, COALESCE(github_url,''), COALESCE(github_token,''),
		       COALESCE(last_commit_sha,''), COALESCE(auto_update,0), COALESCE(auto_restart,0)
		FROM scripts
		WHERE github_url != '' AND auto_update = 1
	`)
	if err != nil {
		fmt.Println("[github-poller] DB error:", err)
		return
	}
	defer rows.Close()

	type scriptInfo struct {
		id          string
		dirPath     string
		githubURL   string
		token       string
		lastSHA     string
		autoUpdate  int
		autoRestart int
	}

	var scripts []scriptInfo
	for rows.Next() {
		var s scriptInfo
		rows.Scan(&s.id, &s.dirPath, &s.githubURL, &s.token, &s.lastSHA, &s.autoUpdate, &s.autoRestart)
		scripts = append(scripts, s)
	}
	rows.Close()

	for _, s := range scripts {
		pollScript(s.id, s.dirPath, s.githubURL, s.token, s.lastSHA, s.autoRestart == 1)
		// Petit délai entre chaque script pour ne pas spammer GitHub
		time.Sleep(2 * time.Second)
	}

	fmt.Printf("[github-poller] done, polled %d scripts\n", len(scripts))
}

func pollScript(scriptID, dirPath, githubURL, token, lastSHA string, autoRestart bool) {
	// 1. Vérifier si il y a un nouveau commit via l'API GitHub
	remoteSHA, err := getLatestRemoteSHA(githubURL, token)
	if err != nil {
		fmt.Printf("[github-poller] %s: SHA check failed: %s\n", scriptID[:8], err)
		return
	}

	if remoteSHA == lastSHA {
		fmt.Printf("[github-poller] %s: already up to date (%s)\n", scriptID[:8], remoteSHA[:8])
		return
	}

	fmt.Printf("[github-poller] %s: new commit %s → pulling\n", scriptID[:8], remoteSHA[:8])

	// 2. Trouver une éventuelle exécution en cours
	var runningExecID string
	db.QueryRow(
		`SELECT id FROM executions WHERE script_id = ? AND status = 'running' ORDER BY started_at DESC LIMIT 1`,
		scriptID,
	).Scan(&runningExecID)

	// 3. Si autoRestart et un exec tourne → le marquer failed (le container sera nettoyé par runContainer)
	if autoRestart && runningExecID != "" {
		fmt.Printf("[github-poller] %s: stopping running execution %s\n", scriptID[:8], runningExecID[:8])
		stopRunningContainer(runningExecID)
	}

	// 4. Pull
	changed, err := pullRepo(dirPath, token)
	if err != nil {
		fmt.Printf("[github-poller] %s: pull failed: %s\n", scriptID[:8], err)
		return
	}

	// 5. Mettre à jour le SHA en DB
	db.Exec(
		`UPDATE scripts SET last_commit_sha = ?, last_pulled_at = ? WHERE id = ?`,
		remoteSHA, time.Now().UTC().Format(time.RFC3339), scriptID,
	)

	if !changed {
		return
	}

	// 6. Si autoRestart → relancer le script
	if autoRestart {
		fmt.Printf("[github-poller] %s: restarting after update\n", scriptID[:8])
		launchScriptAfterUpdate(scriptID)
	}
}

// stopRunningContainer arrête proprement un container Docker via l'exec ID
func stopRunningContainer(execID string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(
		client.WithHost("npipe:////./pipe/docker_engine"),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		fmt.Println("[github-poller] Docker client error:", err)
		return
	}
	defer cli.Close()

	timeout := 5 // secondes
	cli.ContainerStop(ctx, execID, container.StopOptions{Timeout: &timeout})
	db.Exec(
		`UPDATE executions SET status = 'failed', exit_code = -1, finished_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), execID,
	)
}

// launchScriptAfterUpdate recrée une exécution et lance le container
func launchScriptAfterUpdate(scriptID string) {
	type Script struct {
		DockerImage string
		FilePath    string
		Language    string
		Entrypoint  string
	}
	var s Script
	err := db.QueryRow(
		`SELECT docker_image, file_path, language, entrypoint FROM scripts WHERE id = ?`,
		scriptID,
	).Scan(&s.DockerImage, &s.FilePath, &s.Language, &s.Entrypoint)
	if err != nil {
		fmt.Println("[github-poller] script not found for restart:", err)
		return
	}

	// Récupérer le user_id
	var userID int
	db.QueryRow(`SELECT user_id FROM scripts WHERE id = ?`, scriptID).Scan(&userID)

	executionID := uuid.New().String()
	_, err = db.Exec(
		`INSERT INTO executions (id, script_id, user_id, status, started_at) VALUES (?, ?, ?, 'running', ?)`,
		executionID, scriptID, userID, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		fmt.Println("[github-poller] insert execution error:", err)
		return
	}

	go runContainer(executionID, s.DockerImage, s.FilePath, s.Language, s.Entrypoint)
}
