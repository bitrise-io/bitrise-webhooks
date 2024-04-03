package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bitrise-io/api-utils/logging"
)

// WrapHandlerFunc ...
func WrapHandlerFunc(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	logger := logging.WithContext(nil)
	defer logger.Sync()

	requestWrap := func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		h(w, req)
		logger.Info(fmt.Sprintf(" => %s: %s - %s (%s)", req.Method, req.RequestURI, time.Since(startTime), req.Header.Get("Content-Type")))
	}
	return requestWrap
	// if newRelicAgent == nil {
	// 	return requestWrap
	// }
	// return newRelicAgent.WrapHTTPHandlerFunc(requestWrap)
}

// Trace ...
func Trace(name string, fn func()) {
	logger := logging.WithContext(nil)
	defer logger.Sync()

	wrapFn := func() {
		startTime := time.Now()
		fn()
		logger.Info(fmt.Sprintf(" ==> TRACE (%s) - %s", name, time.Since(startTime)))
	}
	wrapFn()
	return
	// if newRelicAgent == nil {
	// 	wrapFn()
	// 	return
	// }
	// newRelicAgent.Tracer.Trace(name, wrapFn)
}
