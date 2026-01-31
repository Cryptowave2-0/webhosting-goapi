package api

import (
	"encoding/json"
	"net/http"
)

type Error struct {
	Code int
	Message string
}

type loginParams struct{
	Username string
	Password string
}

type loginResponse struct {
	Code int
	Hash64 string
}

