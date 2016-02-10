package service

import (
	"encoding/json"
	"log"
	"net/http"
)

// StandardErrorRespModel ...
type StandardErrorRespModel struct {
	ErrorMessage string `json:"error_message"`
}

// RespondWithSuccess ...
func RespondWithSuccess(w http.ResponseWriter, respModel interface{}) {
	w.Header().Set("Content Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(&respModel); err != nil {
		log.Println("respondWithSuccess: Error: ", err)
	}
}

// RespondWithSuccessJSONBytes ...
func RespondWithSuccessJSONBytes(w http.ResponseWriter, respBytes []byte) {
	w.Header().Set("Content Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(respBytes); err != nil {
		log.Println("respondWithSuccessJSONBytes: Error: ", err)
	}
}

// RespondWithBadRequestError ...
func RespondWithBadRequestError(w http.ResponseWriter, errMsg string) {
	RespondWithError(w, errMsg, http.StatusBadRequest)
}

// RespondWithNotFoundError ...
func RespondWithNotFoundError(w http.ResponseWriter, errMsg string) {
	RespondWithError(w, errMsg, http.StatusNotFound)
}

// RespondWithError ...
func RespondWithError(w http.ResponseWriter, errMsg string, httpErrCode int) {
	resp := StandardErrorRespModel{
		ErrorMessage: errMsg,
	}

	w.Header().Set("Content Type", "application/json")
	w.WriteHeader(httpErrCode)
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		log.Println("Error: ", err)
	}
}
