package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"fmt"

	"github.com/Cryptowave2-0/webhosting-goapi/api"
	"github.com/Cryptowave2-0/webhosting-goapi/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var userID int
	var hashedPassword string

	err := db.QueryRow(
		"SELECT id, password FROM users WHERE username = ?",
		req.Username,
	).Scan(&userID, &hashedPassword)

	if err == sql.ErrNoRows {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	} else if err != nil {
		api.InternalErrorHandler(w)
		fmt.Println(err.Error())
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateSessionToken()
	if err != nil {
		api.InternalErrorHandler(w)
		fmt.Println(err.Error())
		return
	}

	err = auth.SaveSession(token, userID)
	// if err != nil {
	// 	api.InternalErrorHandler(w)
	// 	fmt.Println(err.Error())
	// 	return
	// } 
	if err != nil {
    	http.Error(w, err.Error(), 500) // TEMPORAIRE DEBUG
    	return
	}

	// 4. Envoyer au client via cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
	})

	w.Write([]byte("Logged in : "+ token))
}
