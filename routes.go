package main

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/bitrise-io/bitrise-webhooks/service"
	"github.com/bitrise-io/bitrise-webhooks/service/hook"
	"github.com/bitrise-io/bitrise-webhooks/service/root"
	"github.com/gorilla/mux"
)

func setupRoutes() {
	r := mux.NewRouter()
	//
	r.HandleFunc("/h/{service-id}/{app-slug}/{api-token}", metrics.WrapHandlerFunc(hook.HTTPHandler)).
		Methods("POST")
	//
	r.HandleFunc("/", metrics.WrapHandlerFunc(root.HTTPHandler)).
		Methods("GET")
	//
	r.NotFoundHandler = http.HandlerFunc(metrics.WrapHandlerFunc(routeNotFoundHandler))
	//
	http.Handle("/", r)
}

func routeNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	service.RespondWithNotFoundError(w, "Not Found")
}
