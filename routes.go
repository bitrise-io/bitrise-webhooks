package main

import (
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"net/http"
	"os"

	"github.com/bitrise-io/bitrise-webhooks/internal/pubsub"
	"github.com/bitrise-io/bitrise-webhooks/metrics"
	"github.com/bitrise-io/bitrise-webhooks/service"
	"github.com/bitrise-io/bitrise-webhooks/service/hook"
	"github.com/bitrise-io/bitrise-webhooks/service/root"
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

func setupRoutes(pubsubClient *pubsub.Client) {
	r := mux.NewRouter(mux.WithServiceName("webhooks"))
	r.Use(dropTraceMiddleware())

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

func dropTraceMiddleware() func(http.Handler) http.Handler {
	header := os.Getenv("DROP_TRACE_HEADER")
	if header == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(header) != "" {
				span, _ := tracer.StartSpanFromContext(r.Context(), "Drop trace")
				defer span.Finish()

				span.SetTag(ext.ManualDrop, true)
			}

			next.ServeHTTP(w, r)
		})
	}
}
