package main

import "net/http"

func routeNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	respondWithNotFoundError(w, "Not Found")
}
