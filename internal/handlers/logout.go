package handlers

import (
	"net/http"

	"github.com/Cryptowave2-0/webhosting-goapi/internal/auth"
)

func LogoutHandler(w http.ResponseWriter, r *http.Request) {

	cookie, err := r.Cookie("session_token")
	if err == nil {
		_ = auth.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
	})

	w.Write([]byte("Logged out"))
}
