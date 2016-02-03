package main

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/gorilla/mux"
)

func setupRoutes() {
	r := mux.NewRouter()
	r.HandleFunc("/hook/{repo-slug}/{api-token}", metrics.WrapHandlerFunc(hookHandler))
	r.HandleFunc("/", metrics.WrapHandlerFunc(rootHandler))
	http.Handle("/", r)
}
