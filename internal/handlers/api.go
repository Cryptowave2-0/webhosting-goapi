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
	r.Post("/logout", LogoutHandler)

	// routes protégées
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.AuthMiddleware(db)) // middleware prend db
		protected.Post("/logout", LogoutHandler)
	})
}