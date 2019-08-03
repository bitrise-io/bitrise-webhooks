package httprequest

import (
	"log"
	"net/http"
)

// BodyCloseWithErrorLog ...
func BodyCloseWithErrorLog(r *http.Request) {
	err := r.Body.Close()
	if err != nil {
		log.Printf(" [!] Exception: request.BodyCloseWithErrorLog: %+v", err)
	}
}
