package httpresponse

import (
	"log"
	"net/http"
)

// BodyCloseWithErrorLog ...
func BodyCloseWithErrorLog(r *http.Response) {
	err := r.Body.Close()
	if err != nil {
		log.Printf(" [!] Exception: response.BodyCloseWithErrorLog: %+v", err)
	}
}
