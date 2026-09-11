// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build (linux || darwin) && (amd64 || arm64) && !go1.28 && !datadog.no_waf && !cgo && appsec

package log

import (
	"sync"

	"github.com/DataDog/go-libddwaf/v5/internal/unsafeutil"

	"github.com/ebitengine/purego"
)

var (
	once = sync.OnceValue(func() uintptr {
		return purego.NewCallback(ddwafLogCallbackFn)
	})
)

// CallbackFunctionPointer returns a pointer to the log callback function which
// can be used with libddwaf.
func CallbackFunctionPointer() uintptr {
	return once()
}

func ddwafLogCallbackFn(level Level, fnPtr, filePtr *byte, line uint, msgPtr *byte, _ uint64) {
	function := unsafeutil.Gostring(fnPtr)
	file := unsafeutil.Gostring(filePtr)
	message := unsafeutil.Gostring(msgPtr)

	logMessage(level, function, file, line, message)
}
