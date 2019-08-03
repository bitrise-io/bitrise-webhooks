package providers

import (
	"net/http"

	"github.com/gorilla/mux"
)

// RequestParamsInterface ...
type RequestParamsInterface interface {
	Get(req *http.Request) map[string]string
}

// RequestParams ...
type RequestParams struct{}

// Get ...
func (r *RequestParams) Get(req *http.Request) map[string]string {
	return mux.Vars(req)
}
