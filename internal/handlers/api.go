package handlers

import (
	"database/sql"

	"github.com/Cryptowave2-0/webhosting-goapi/internal/middleware"
	"github.com/go-chi/chi"
)


var db *sql.DB

func Setup(database *sql.DB) {
	db = database
}

func RegisterAPIRoutes(r chi.Router) {

	r.Post("/login", LoginHandler)

	r.Group(func(protected chi.Router) {
		protected.Use(middleware.AuthMiddleware(db)) // middleware prend db
		
		protected.Post("/logout", LogoutHandler)

		// Scripts
		protected.Post("/scripts/upload", UploadScriptHandler)
		protected.Get("/scripts", ListScriptsHandler)
		protected.Get("/scripts/{id}", GetScriptHandler)
		protected.Delete("/scripts/{id}", DeleteScriptHandler)
		protected.Get("/scripts/{id}/executions", GetScriptExecutionsHandler)

		// Settings
		protected.Get("/scripts/{id}/settings", GetSettingsHandler)
		protected.Patch("/scripts/{id}/settings", UpdateSettingsHandler)

		// GitHub
		protected.Post("/scripts/{id}/github/clone", GithubCloneHandler)
		protected.Post("/scripts/{id}/github/pull", GithubPullHandler)

		// Files — ordre important : routes fixes avant wildcard
		protected.Get("/scripts/{id}/archive", DownloadArchiveHandler)
		protected.Get("/scripts/{id}/files/*", ReadFileHandler)
		protected.Put("/scripts/{id}/files/*", WriteFileHandler)
		protected.Post("/scripts/{id}/files/*", CreateFileHandler)
		protected.Patch("/scripts/{id}/files/*", MoveFileHandler)
		protected.Delete("/scripts/{id}/files/*", DeleteFileHandler)
		protected.Get("/scripts/{id}/download/*", DownloadFileHandler)

		// Executions
		protected.Post("/scripts/{id}/run", RunScriptHandler)
		protected.Get("/executions/{id}", GetExecutionHandler)
		protected.Get("/executions/{id}/logs", GetExecutionLogsHandler)
		protected.Get("/executions/{id}/stream", StreamExecutionHandler)

	})
}