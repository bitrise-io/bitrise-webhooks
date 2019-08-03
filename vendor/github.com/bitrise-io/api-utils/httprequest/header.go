package httprequest

import (
	"errors"
	"net/http"
	"strings"
)

// AuthTokenFromHeader ...
func AuthTokenFromHeader(h http.Header) (string, error) {
	headerValue := h.Get("Authorization")
	token := strings.TrimPrefix(headerValue, "token ")
	if token == "" {
		return "", errors.New("No Authorization header specified")
	}
	return token, nil
}
