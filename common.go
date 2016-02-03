package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// StandardErrorRespModel ...
type StandardErrorRespModel struct {
	ErrorMessage string `json:"error_message"`
}

func respondWithSuccess(w http.ResponseWriter, respModel interface{}) {
	w.Header().Set("Content Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(&respModel); err != nil {
		log.Println("Error: ", err)
	}
}

func respondWithBadRequestError(w http.ResponseWriter, errMsg string) {
	respondWithError(w, errMsg, http.StatusBadRequest)
}

func respondWithNotFoundError(w http.ResponseWriter, errMsg string) {
	respondWithError(w, errMsg, http.StatusNotFound)
}

func respondWithError(w http.ResponseWriter, errMsg string, httpErrCode int) {
	resp := StandardErrorRespModel{
		ErrorMessage: errMsg,
	}

	w.Header().Set("Content Type", "application/json")
	w.WriteHeader(httpErrCode)
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		log.Println("Error: ", err)
	}
}
