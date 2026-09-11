// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package bindings

import (
	"errors"
	"sync"

	"github.com/DataDog/go-libddwaf/v5/internal/support"
)

// Globally dlopen() libddwaf only once because several dlopens (eg. in tests)
// aren't supported by macOS.
var (
	// Lib is libddwaf's dynamic library handle and entrypoints. This is only safe to
	// read after calling [Load] or having acquired [gMu].
	Lib *WAFLib
	// libddwaf's dlopen error if any. This is only safe to read after calling
	// [Load] or having acquired [gMu].
	gWafLoadErr error
	wafVersion  string
	// Protects the global variables above.
	gMu sync.Mutex

	openWafOnce sync.Once
)

// Load loads libddwaf's dynamic library once and stores it globally.
// It returns true when libddwaf was successfully loaded, along with an error.
func Load() (bool, error) {
	if ok, err := Usable(); !ok {
		return false, err
	}

	openWafOnce.Do(func() {
		gMu.Lock()
		defer gMu.Unlock()

		Lib, gWafLoadErr = newWAFLib()
		if gWafLoadErr != nil {
			return
		}
		wafVersion = Lib.Version()
	})

	return Lib != nil, gWafLoadErr
}

// Version returns the version returned by libddwaf.
// It relies on the dynamic loading of the library, which can fail and return
// an empty string or the previously loaded version, if any.
func Version() string {
	_, _ = Load()
	return wafVersion
}

// Usable returns true if the Lib is usable, false and an error otherwise.
//
// If the Lib is usable, an error value may still be returned and should be
// treated as a warning (it is non-blocking).
//
// The following conditions are checked:
//   - The Lib library has been loaded successfully (you need to call [Load] first for this case to be
//     taken into account)
//   - The Lib library has not been manually disabled with the `datadog.no_waf` go build tag
//   - The Lib library is not in an unsupported OS/Arch
//   - The Lib library is not in an unsupported Go version
func Usable() (bool, error) {
	wafSupportErrors := errors.Join(support.WafSupportErrors()...)
	wafManuallyDisabledErr := support.WafManuallyDisabledError()

	gMu.Lock()
	defer gMu.Unlock()

	usable := (Lib != nil || gWafLoadErr == nil) && wafSupportErrors == nil && wafManuallyDisabledErr == nil
	return usable, errors.Join(gWafLoadErr, wafSupportErrors, wafManuallyDisabledErr)
}
