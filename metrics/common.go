package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"syscall"
	"time"

	"github.com/bitrise-io/api-utils/logging"
)

// WrapHandlerFunc ...
func WrapHandlerFunc(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	logger := logging.WithContext(nil)
	defer func() {
		err := logger.Sync()
		if err != nil && !errors.Is(err, syscall.ENOTTY) {
			fmt.Println("Failed to Sync logger", err)
		}
	}()

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
	defer func() {
		err := logger.Sync()
		if err != nil && !errors.Is(err, syscall.ENOTTY) {
			fmt.Println("Failed to Sync logger", err)
		}
	}()

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
