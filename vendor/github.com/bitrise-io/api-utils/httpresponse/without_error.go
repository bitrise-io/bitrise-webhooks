package httpresponse

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pkg/errors"
)

// StandardErrorRespModel ...
type StandardErrorRespModel struct {
	Message string `json:"message"`
}

// ValidationErrorRespModel ...
type ValidationErrorRespModel struct {
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

// HanderFuncWithInternalError ...
type HanderFuncWithInternalError func(http.ResponseWriter, *http.Request) error

// InternalErrHandlerFuncAdapter ...
func InternalErrHandlerFuncAdapter(h HanderFuncWithInternalError) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		intServErr := h(w, r)
		if intServErr != nil {
			RespondWithInternalServerError(w, errors.WithStack(intServErr))
		}
	})
}

// RespondWithJSONNoErr ...
func RespondWithJSONNoErr(w http.ResponseWriter, httpCode int, respModel interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpCode)
	if err := json.NewEncoder(w).Encode(&respModel); err != nil {
		log.Printf(" [!] Exception: failed to respond with JSON, error: %+v", errors.WithStack(err))
	}
}

// RespondWithErrorNoErr ...
func RespondWithErrorNoErr(w http.ResponseWriter, errMsg string, httpErrCode int) {
	RespondWithJSONNoErr(w, httpErrCode, StandardErrorRespModel{
		Message: errMsg,
	})
}

// RespondWithSuccessNoErr ...
func RespondWithSuccessNoErr(w http.ResponseWriter, respModel interface{}) {
	RespondWithJSONNoErr(w, http.StatusOK, respModel)
}

// RespondWithBadRequestErrorNoErr ...
func RespondWithBadRequestErrorNoErr(w http.ResponseWriter, errMsg string) {
	RespondWithErrorNoErr(w, errMsg, http.StatusBadRequest)
}

// RespondWithNotFoundErrorWithMessageNoErr ...
func RespondWithNotFoundErrorWithMessageNoErr(w http.ResponseWriter, errMsg string) {
	RespondWithErrorNoErr(w, errMsg, http.StatusNotFound)
}

// RespondWithNotFoundErrorNoErr ...
func RespondWithNotFoundErrorNoErr(w http.ResponseWriter) {
	RespondWithNotFoundErrorWithMessageNoErr(w, "Not Found")
}

// RespondWithUnauthorizedNoErr ...
func RespondWithUnauthorizedNoErr(w http.ResponseWriter) {
	RespondWithErrorNoErr(w, "Unauthorized", http.StatusUnauthorized)
}

// RespondWithForbiddenNoErr ...
func RespondWithForbiddenNoErr(w http.ResponseWriter) {
	RespondWithErrorNoErr(w, "Forbidden", http.StatusForbidden)
}

// RespondWithInternalServerError ...
func RespondWithInternalServerError(w http.ResponseWriter, errorToLog error) {
	log.Printf(" [!] Exception: Internal Server Error: %+v", errors.WithStack(errorToLog))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, err := fmt.Fprintln(w, `{"message":"Internal Server Error"}`)
	if err != nil {
		log.Printf(" [!] Exception: failed to write Internal Server Error response, error: %+v", errors.WithStack(err))
	}
}
