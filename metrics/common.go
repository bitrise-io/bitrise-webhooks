package metrics

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// WrapHandlerFunc ...
func WrapHandlerFunc(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	requestWrap := func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				//panic happened
				w.Header().Set("Connection", "close")

				formattedError := fmt.Errorf("%s", err)
				log.Printf("PANIC happened: %s  --  %s", formattedError.Error(), debug.Stack())
			}
		}()

		startTime := time.Now()
		h(w, req)
		log.Printf(" => %s: %s - %s (%s)", req.Method, req.RequestURI, time.Since(startTime), req.Header.Get("Content-Type"))
	}
	return requestWrap
}

// Trace ...
func Trace(name string, fn func()) {
	wrapFn := func() {
		startTime := time.Now()
		fn()
		log.Printf(" ==> TRACE (%s) - %s", name, time.Since(startTime))
	}
	wrapFn()
	return
}
