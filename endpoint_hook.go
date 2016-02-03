package main

import (
	"log"
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/metrics"
)

// HookRespModel ...
type HookRespModel struct {
	Msg string `json:"msg"`
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	metrics.Trace("Hook", func() {
		log.Println("Handling hook...")
	})

	resp := HookRespModel{
		Msg: "This is the Hook Handler endpoint",
	}

	respondWithSuccess(w, resp)
}
