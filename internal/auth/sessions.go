package auth

import (
	"database/sql"
)

var db *sql.DB

func Setup(database *sql.DB) {
	db = database
}

func SaveSession(token string, userID int) error {
	_, err := db.Exec("INSERT OR REPLACE INTO sessions(token, user_id) VALUES(?, ?)", token, userID)
	return err
}

func GetUserFromSession(token string) (int, error) {
	var userID int
	err := db.QueryRow("SELECT user_id FROM sessions WHERE token = ?", token).Scan(&userID)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func DeleteSession(token string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}