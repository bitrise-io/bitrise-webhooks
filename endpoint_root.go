package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bitrise-io/bitrise-webhooks/config"
)

// RootRespModel ...
type RootRespModel struct {
	Message         string `json:"message"`
	Version         string `json:"version"`
	Time            string `json:"time"`
	EnvironmentMode string `json:"environment_mode"`
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	resp := RootRespModel{
		Message:         "Welcome to bitrise-webhooks!",
		Version:         VERSION,
		Time:            fmt.Sprintf("%s", time.Now()),
		EnvironmentMode: config.GetServerEnvMode(),
	}

	respondWithSuccess(w, resp)
}
