package main

import (
	"fmt"
	"net/http"
	"time"
)

// RootRespModel ...
type RootRespModel struct {
	Message string `json:"message"`
	Version string `json:"version"`
	Time    string `json:"time"`
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	resp := RootRespModel{
		Message: "Welcome to bitrise-webhooks!",
		Version: VERSION,
		Time:    fmt.Sprintf("%s", time.Now()),
	}

	respondWithSuccess(w, resp)
}
