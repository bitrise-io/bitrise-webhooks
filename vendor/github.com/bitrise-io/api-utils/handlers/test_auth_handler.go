package handlers

import (
	"net/http"

	"github.com/bitrise-io/api-utils/context"
	"github.com/bitrise-io/api-utils/httpresponse"
)

// TestAuthHandler ...
type TestAuthHandler struct {
	ContextElementList map[string]context.RequestContextKey
}

func (h *TestAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{}
	for respKey, ctxKey := range h.ContextElementList {
		response[respKey] = r.Context().Value(ctxKey)
	}

	httpresponse.RespondWithSuccessNoErr(w, response)
}
