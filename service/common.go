package service

import (
	"encoding/json"
	"net/http"

	"github.com/bitrise-io/api-utils/logging"
	"go.uber.org/zap"
)

// StandardErrorRespModel ...
type StandardErrorRespModel struct {
	ErrorMessage string `json:"error"`
}

// -----------------
// --- Generic ---

// RespondWith ...
func RespondWith(w http.ResponseWriter, httpStatusCode int, respModel interface{}) {
	logger := logging.WithContext(nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(&respModel); err != nil {
		logger.Error(" [!] Exception: RespondWith", zap.Error(err))
	}
}

// -----------------
// --- Successes ---

// RespondWithSuccessOK ...
func RespondWithSuccessOK(w http.ResponseWriter, respModel interface{}) {
	RespondWith(w, http.StatusOK, respModel)
}

// --------------
// --- Errors ---

// RespondWithBadRequestError ...
func RespondWithBadRequestError(w http.ResponseWriter, errMsg string) {
	RespondWithError(w, http.StatusBadRequest, errMsg)
}

// RespondWithNotFoundError ...
func RespondWithNotFoundError(w http.ResponseWriter, errMsg string) {
	RespondWithError(w, http.StatusNotFound, errMsg)
}

// RespondWithError ...
func RespondWithError(w http.ResponseWriter, httpErrCode int, errMsg string) {
	resp := StandardErrorRespModel{
		ErrorMessage: errMsg,
	}
	RespondWithErrorJSON(w, httpErrCode, resp)
}

// RespondWithErrorJSON ...
func RespondWithErrorJSON(w http.ResponseWriter, httpErrCode int, respModel interface{}) {
	logger := logging.WithContext(nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpErrCode)
	if err := json.NewEncoder(w).Encode(&respModel); err != nil {
		logger.Error(" [!] Exception: RespondWithErrorJSON", zap.Error(err))
	}
}
