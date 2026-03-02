package main

import (
	"fmt"
	"net/http"
	"database/sql"

	"github.com/go-chi/chi"
	"github.com/Cryptowave2-0/webhosting-goapi/internal/handlers"
	"github.com/Cryptowave2-0/webhosting-goapi/internal/auth"
	log "github.com/sirupsen/logrus"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.SetReportCaller(true)
	var r *chi.Mux = chi.NewRouter()

	db, err := sql.Open("sqlite3", "app.db")
	if err != nil {
		fmt.Println(err.Error())
		log.Fatal(err)
	}
	
	defer db.Close()

	initDB(db)

	auth.Setup(db)
	handlers.Setup(db)

	fmt.Println(`
	 ______    ______   __    __
	/\  __ \  /\  == \ /\ \  /\_\
	\ \  __ \ \ \  _-/ \ \ \ \/_/_
	 \ \_\ \ \ \ \_\    \ \_\  /\_\
	  \/_/\/_/  \/_/     \/_/  \/_/
	`)

	fmt.Println("Starting GO API service...")

	handlers.RegisterAPIRoutes(r)

	err1 := http.ListenAndServe(":8000", r)
	if err1 != nil {
		log.Error(err1)
	}
	
}

func initDB(db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER NOT NULL UNIQUE PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL
		);`)
	if err != nil {
    	log.Fatalf("failed creating users table: %v", err)
	} else {
		fmt.Println("Table 'users' created succesfully")
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		`)
	if err != nil {
    	log.Fatalf("failed creating users table: %v", err)
	} else {
		fmt.Println("Table 'sessions' created succesfully")
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS scripts (
			id          TEXT PRIMARY KEY,
			user_id     INTEGER NOT NULL,
			name        TEXT NOT NULL,
			description TEXT DEFAULT '',
			language    TEXT NOT NULL,
			docker_image TEXT NOT NULL,
			file_path   TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`)
	if err != nil {
		log.Fatalf("failed creating scripts table: %v", err)
	} else {
		fmt.Println("Table 'scripts' created succesfully")
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS executions (
			id          TEXT PRIMARY KEY,
			script_id   TEXT NOT NULL,
			user_id     INTEGER NOT NULL,
			status      TEXT NOT NULL DEFAULT 'pending',
			exit_code   INTEGER,
			started_at  DATETIME,
			finished_at DATETIME,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(script_id) REFERENCES scripts(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`)
	if err != nil {
		log.Fatalf("failed creating executions table: %v", err)
	} else {
		fmt.Println("Table 'executions' created succesfully")
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS logs (
			id           TEXT PRIMARY KEY,
			execution_id TEXT NOT NULL,
			stream       TEXT NOT NULL,
			content      TEXT NOT NULL,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(execution_id) REFERENCES executions(id) ON DELETE CASCADE
		);`)
	if err != nil {
		log.Fatalf("failed creating logs table: %v", err)
	} else {
		fmt.Println("Table 'logs' created succesfully")
	}

}