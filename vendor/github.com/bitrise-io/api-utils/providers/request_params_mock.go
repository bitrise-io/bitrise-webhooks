package providers

import "net/http"

// RequestParamsMock ...
type RequestParamsMock struct {
	Params map[string]string
}

// Get ...
func (r *RequestParamsMock) Get(req *http.Request) map[string]string {
	return r.Params
}
