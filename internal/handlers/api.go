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

	// routes protégées
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.AuthMiddleware(db)) // middleware prend db
		
		protected.Post("/logout", LogoutHandler)

		// Scripts
		protected.Post("/scripts/upload", UploadScriptHandler)
		protected.Get("/scripts", ListScriptsHandler)
		protected.Get("/scripts/{id}", GetScriptHandler)
		protected.Delete("/scripts/{id}", DeleteScriptHandler)

		// Exécutions
		protected.Post("/scripts/{id}/run", RunScriptHandler)
		protected.Get("/executions/{id}", GetExecutionHandler)
		protected.Get("/executions/{id}/logs", GetExecutionLogsHandler)

	})
}