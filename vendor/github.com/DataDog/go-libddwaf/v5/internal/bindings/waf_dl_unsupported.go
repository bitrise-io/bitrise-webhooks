// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build (!linux && !darwin) || (!amd64 && !arm64) || go1.28 || datadog.no_waf || (!cgo && !appsec)

package bindings

import (
	"errors"

	"github.com/DataDog/go-libddwaf/v5/internal/log"
)

type WAFLib struct{}

func newWAFLib() (*WAFLib, error) {
	return nil, errors.New("go-libddwaf is not supported on this platform")
}

func (*WAFLib) Close() error { return nil }

func (*WAFLib) Version() string { return "" }

func (*WAFLib) DefaultAllocator() WAFAllocator { return 0 }

func (*WAFLib) BuilderInit() WAFBuilder { return 0 }

func (*WAFLib) BuilderAddOrUpdateConfig(WAFBuilder, string, *WAFObject, *WAFObject) bool {
	return false
}

func (*WAFLib) BuilderRemoveConfig(WAFBuilder, string) bool { return false }

func (*WAFLib) BuilderBuildInstance(WAFBuilder) WAFHandle { return 0 }

func (*WAFLib) BuilderGetConfigPaths(WAFBuilder, string) ([]string, error) { return nil, nil }

func (*WAFLib) BuilderDestroy(WAFBuilder) {}

func (*WAFLib) SetLogCb(uintptr, log.Level) {}

func (*WAFLib) Destroy(WAFHandle) {}

func (*WAFLib) KnownAddresses(WAFHandle) []string { return nil }

func (*WAFLib) KnownActions(WAFHandle) []string { return nil }

func (*WAFLib) ContextInit(WAFHandle, WAFAllocator) WAFContext { return 0 }

func (*WAFLib) ContextEval(WAFContext, *WAFObject, WAFAllocator, *WAFObject, uint64) WAFReturnCode {
	return WAFErrInternal
}

func (*WAFLib) ContextDestroy(WAFContext) {}

func (*WAFLib) SubcontextInit(WAFContext) WAFSubcontext { return 0 }

func (*WAFLib) SubcontextEval(WAFSubcontext, *WAFObject, WAFAllocator, *WAFObject, uint64) WAFReturnCode {
	return WAFErrInternal
}

func (*WAFLib) SubcontextDestroy(WAFSubcontext) {}

func (*WAFLib) ObjectDestroy(*WAFObject, WAFAllocator) {}

func (*WAFLib) ObjectFromJSON([]byte, WAFAllocator) (WAFObject, bool) { return WAFObject{}, false }

func (*WAFLib) Handle() uintptr { return 0 }
