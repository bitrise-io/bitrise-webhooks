package metrics

import (
	"log"
	"net/http"
	"time"
)

// WrapHandlerFunc ...
func WrapHandlerFunc(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	requestWrap := func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		h(w, req)
		log.Printf(" => %s: %s - %s (%s)", req.Method, req.RequestURI, time.Since(startTime), req.Header.Get("Content-Type"))
	}
	return requestWrap
	// if newRelicAgent == nil {
	// 	return requestWrap
	// }
	// return newRelicAgent.WrapHTTPHandlerFunc(requestWrap)
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
	// if newRelicAgent == nil {
	// 	wrapFn()
	// 	return
	// }
	// newRelicAgent.Tracer.Trace(name, wrapFn)
}
