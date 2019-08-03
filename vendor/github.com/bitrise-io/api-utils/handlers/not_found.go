package handlers

import (
	"net/http"

	"github.com/bitrise-io/api-utils/httpresponse"
)

// NotFoundHandler ...
type NotFoundHandler struct {
}

func (h *NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpresponse.RespondWithJSONNoErr(w, http.StatusNotFound, httpresponse.StandardErrorRespModel{
		Message: "Not Found",
	})
}
