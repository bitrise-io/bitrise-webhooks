package main

import (
	"net/http"

	"github.com/bitrise-io/bitrise-webhooks/internal/pubsub"
	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/bitrise-io/bitrise-webhooks/service"
	"github.com/bitrise-io/bitrise-webhooks/service/hook"
	"github.com/bitrise-io/bitrise-webhooks/service/root"
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

func setupRoutes(pubsubClient *pubsub.Client) {
	r := mux.NewRouter(mux.WithServiceName("webhooks"))
	//
	hookClient := hook.Client{PubsubClient: pubsubClient}
	r.HandleFunc("/h/{service-id}/{app-slug}/{api-token}", metrics.WrapHandlerFunc(hookClient.HTTPHandler)).
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
