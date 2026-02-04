package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Vérifie que la DB répond
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func Init(db *sql.DB) error {
	queries := []string{
		`PRAGMA foreign_keys = ON;`,
		
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER NOT NULL UNIQUE PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id)
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}

	log.Println("Database initialized")
	return nil
}
