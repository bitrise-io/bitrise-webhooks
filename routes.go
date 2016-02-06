package main

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/gorilla/mux"
)

func setupRoutes() {
	r := mux.NewRouter()
	r.HandleFunc("/hook/{app-slug}/{api-token}", metrics.WrapHandlerFunc(hookHandler)).
		Methods("POST")
	r.HandleFunc("/", metrics.WrapHandlerFunc(rootHandler)).
		Methods("GET")
	r.NotFoundHandler = http.HandlerFunc(metrics.WrapHandlerFunc(routeNotFoundHandler))
	http.Handle("/", r)
}
