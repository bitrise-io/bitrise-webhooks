// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build (linux || darwin) && (amd64 || arm64) && !go1.28 && !datadog.no_waf && (cgo || appsec)

package bindings

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/DataDog/go-libddwaf/v5/internal/lib"
	"github.com/DataDog/go-libddwaf/v5/internal/log"
	"github.com/DataDog/go-libddwaf/v5/internal/unsafeutil"
)

// WAFLib is the type wrapper for all C calls to the waf
// It uses `libwaf` to make C calls
// All calls must go through this one-liner to be type safe
// since purego calls are not type safe
type WAFLib struct {
	wafSymbols
	handle           uintptr
	defaultAllocator WAFAllocator
}

// newWAFLib loads the libddwaf shared library and resolves all the relevant symbols.
// The caller is responsible for calling wafDl.Close on the returned object once they
// are done with it so that associated resources can be released.
func newWAFLib() (dl *WAFLib, err error) {
	path, closer, err := lib.DumpEmbeddedWAF()
	if err != nil {
		return nil, fmt.Errorf("failed to dump embedded WAF library: %w", err)
	}
	defer func() {
		if rmErr := closer(); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("error removing %s: %w", path, rmErr))
		}
	}()

	var handle uintptr
	if handle, err = purego.Dlopen(path, purego.RTLD_LOCAL|purego.RTLD_NOW); err != nil {
		return nil, fmt.Errorf("failed to load dynamic library %q: %w", path, err)
	}

	closeOnError := func() {
		if closeErr := purego.Dlclose(handle); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("error closing the shared libddwaf library: %w", closeErr))
		}
	}

	var symbols wafSymbols
	if symbols, err = newWafSymbols(handle); err != nil {
		closeOnError()
		return
	}

	dl = &WAFLib{wafSymbols: symbols, handle: handle}

	// Try calling the waf to make sure everything is fine
	if _, err = tryCall(dl.Version); err != nil {
		err = fmt.Errorf("calling ddwaf_get_version after Dlopen: %w", err)
		closeOnError()
		return
	}

	dl.defaultAllocator = dl.loadDefaultAllocator()

	if val := os.Getenv(log.EnvVarLogLevel); val != "" {
		logLevel := log.LevelNamed(val)
		if logLevel != log.LevelOff {
			dl.SetLogCb(log.CallbackFunctionPointer(), logLevel)
		}
	}

	return
}

func (waf *WAFLib) Close() error {
	return purego.Dlclose(waf.handle)
}

// Version returned string is a static string so we do not need to free it
func (waf *WAFLib) Version() string {
	// The uintptr returned by syscall points to C-managed static memory the Go
	// GC does not track, so converting it back to unsafe.Pointer is safe despite
	// violating vet's unsafeptr rule (standard purego pattern).
	return unsafeutil.Gostring((*byte)(unsafe.Pointer(waf.syscall(waf.getVersion)))) //nolint:govet
}

// loadDefaultAllocator returns the default allocator used by the library.
// This is called once at load time; use DefaultAllocator() for the cached value.
func (waf *WAFLib) loadDefaultAllocator() WAFAllocator {
	return WAFAllocator(waf.syscall(waf.getDefaultAllocator))
}

// DefaultAllocator returns the cached default allocator.
func (waf *WAFLib) DefaultAllocator() WAFAllocator {
	return waf.defaultAllocator
}

// BuilderInit initializes a new WAF builder.
func (waf *WAFLib) BuilderInit() WAFBuilder {
	return WAFBuilder(waf.syscall(waf.builderInit))
}

// BuilderAddOrUpdateConfig adds or updates a configuration based on the
// given path, which must be a unique identifier for the provided configuration.
// Returns false in case of an error.
func (waf *WAFLib) BuilderAddOrUpdateConfig(builder WAFBuilder, path string, config *WAFObject, diags *WAFObject) bool {
	var pinner runtime.Pinner
	defer pinner.Unpin()
	pinner.Pin(config)
	pinner.Pin(diags)

	res := waf.syscall(waf.builderAddOrUpdateConfig,
		uintptr(builder),
		uintptr(unsafe.Pointer(unsafeutil.Cstring(&pinner, path))),
		uintptr(len(path)),
		uintptr(unsafe.Pointer(config)),
		uintptr(unsafe.Pointer(diags)),
	)
	return byte(res) != 0
}

// BuilderRemoveConfig removes a configuration based on the provided path.
// Returns false in case of an error.
func (waf *WAFLib) BuilderRemoveConfig(builder WAFBuilder, path string) bool {
	var pinner runtime.Pinner
	defer pinner.Unpin()

	return byte(waf.syscall(waf.builderRemoveConfig,
		uintptr(builder),
		uintptr(unsafe.Pointer(unsafeutil.Cstring(&pinner, path))),
		uintptr(len(path)),
	)) != 0
}

// BuilderBuildInstance builds a WAF instance based on the current set of configurations.
// Returns nil in case of an error.
func (waf *WAFLib) BuilderBuildInstance(builder WAFBuilder) WAFHandle {
	return WAFHandle(waf.syscall(waf.builderBuildInstance, uintptr(builder)))
}

// BuilderGetConfigPaths returns the list of currently loaded paths.
func (waf *WAFLib) BuilderGetConfigPaths(builder WAFBuilder, filter string) ([]string, error) {
	var paths WAFObject
	var pinner runtime.Pinner
	defer pinner.Unpin()
	pinner.Pin(&paths)

	// Pin the string backing data, not the header
	if filterData := unsafeutil.StringData(filter); filterData != nil {
		pinner.Pin(filterData)
	}

	count := waf.syscall(waf.builderGetConfigPaths,
		uintptr(builder),
		uintptr(unsafe.Pointer(&paths)),
		uintptr(unsafe.Pointer(unsafeutil.StringData(filter))),
		uintptr(len(filter)),
	)
	defer waf.ObjectDestroy(&paths, waf.defaultAllocator)

	list := make([]string, 0, count)
	items, err := paths.ArrayValues()
	if err != nil {
		return nil, fmt.Errorf("failed to decode config paths array for filter %q: %w", filter, err)
	}
	for i, item := range items {
		path, err := item.StringValue()
		if err != nil {
			return nil, fmt.Errorf("failed to decode config path %d for filter %q: %w", i, filter, err)
		}
		list = append(list, path)
	}
	return list, nil
}

// BuilderDestroy destroys a WAF builder instance.
func (waf *WAFLib) BuilderDestroy(builder WAFBuilder) {
	waf.syscall(waf.builderDestroy, uintptr(builder))
}

// SetLogCb sets the log callback function for the WAF.
func (waf *WAFLib) SetLogCb(cb uintptr, level log.Level) {
	waf.syscall(waf.setLogCb, cb, uintptr(level))
}

// Destroy destroys a WAF instance.
func (waf *WAFLib) Destroy(handle WAFHandle) {
	waf.syscall(waf.destroy, uintptr(handle))
}

func (waf *WAFLib) KnownAddresses(handle WAFHandle) []string {
	return waf.knownX(handle, waf.knownAddresses)
}

func (waf *WAFLib) KnownActions(handle WAFHandle) []string {
	return waf.knownX(handle, waf.knownActions)
}

func (waf *WAFLib) knownX(handle WAFHandle, symbol uintptr) []string {
	var nbAddresses uint32

	var pinner runtime.Pinner
	defer pinner.Unpin()
	pinner.Pin(&nbAddresses)

	arrayVoidC := waf.syscall(symbol, uintptr(handle), uintptr(unsafe.Pointer(&nbAddresses)))
	if arrayVoidC == 0 || nbAddresses == 0 {
		return nil
	}

	// These C strings are static strings so we do not need to free them.
	// arrayVoidC points to C-managed memory the Go GC does not track, so the
	// uintptr round-trip is safe despite vet's unsafeptr rule (purego pattern).
	arrayPtr := unsafe.Pointer(arrayVoidC) //nolint:govet
	addresses := make([]string, int(nbAddresses))
	for i := range int(nbAddresses) {
		charPtr := *(**byte)(unsafe.Add(arrayPtr, uintptr(i)*unsafe.Sizeof(uintptr(0))))
		addresses[i] = unsafeutil.Gostring(charPtr)
	}

	return addresses
}

// ContextInit creates a new WAF context.
// Takes an allocator for output objects created during evaluation.
func (waf *WAFLib) ContextInit(handle WAFHandle, outputAlloc WAFAllocator) WAFContext {
	return WAFContext(waf.syscall(waf.contextInit, uintptr(handle), uintptr(outputAlloc)))
}

// ContextEval performs a matching operation on the provided data.
//
// Parameters:
//   - context: WAF context for this evaluation
//   - data: Map of addresses to values for evaluation (will be stored by context)
//   - alloc: Allocator used to free the data after context destruction (can be 0 to not free)
//   - result: Output object for events, actions, duration, timeout, attributes
//   - timeout: Maximum time budget in microseconds
func (waf *WAFLib) ContextEval(context WAFContext, data *WAFObject, alloc WAFAllocator, result *WAFObject, timeout uint64) WAFReturnCode {
	var pinner runtime.Pinner
	defer pinner.Unpin()

	pinner.Pin(data)
	pinner.Pin(result)

	return WAFReturnCode(waf.syscall(waf.contextEval,
		uintptr(context),
		uintptr(unsafe.Pointer(data)),
		uintptr(alloc),
		uintptr(unsafe.Pointer(result)),
		uintptr(timeout),
	))
}

func (waf *WAFLib) ContextDestroy(context WAFContext) {
	waf.syscall(waf.contextDestroy, uintptr(context))
}

// SubcontextInit creates a subcontext from a parent context for ephemeral data evaluation.
func (waf *WAFLib) SubcontextInit(context WAFContext) WAFSubcontext {
	return WAFSubcontext(waf.syscall(waf.subcontextInit, uintptr(context)))
}

// SubcontextEval performs a matching operation on the provided data within a subcontext.
//
// Parameters:
//   - subcontext: WAF subcontext for this evaluation
//   - data: Map of addresses to values for evaluation
//   - alloc: Allocator used to free the data after subcontext destruction (can be 0 to not free)
//   - result: Output object for events, actions, duration, timeout, attributes
//   - timeout: Maximum time budget in microseconds
func (waf *WAFLib) SubcontextEval(subcontext WAFSubcontext, data *WAFObject, alloc WAFAllocator, result *WAFObject, timeout uint64) WAFReturnCode {
	var pinner runtime.Pinner
	defer pinner.Unpin()

	pinner.Pin(data)
	pinner.Pin(result)

	return WAFReturnCode(waf.syscall(waf.subcontextEval,
		uintptr(subcontext),
		uintptr(unsafe.Pointer(data)),
		uintptr(alloc),
		uintptr(unsafe.Pointer(result)),
		uintptr(timeout),
	))
}

// SubcontextDestroy destroys a subcontext and frees data passed to it.
func (waf *WAFLib) SubcontextDestroy(subcontext WAFSubcontext) {
	waf.syscall(waf.subcontextDestroy, uintptr(subcontext))
}

// ObjectDestroy frees the memory contained within the object using the provided allocator.
// The allocator should be the same one that was used to allocate the object's contents,
// or the default allocator if the object was created by the WAF library.
func (waf *WAFLib) ObjectDestroy(obj *WAFObject, alloc WAFAllocator) {
	var pinner runtime.Pinner
	defer pinner.Unpin()
	pinner.Pin(obj)

	waf.syscall(waf.objectDestroy, uintptr(unsafe.Pointer(obj)), uintptr(alloc))
}

func (waf *WAFLib) Handle() uintptr {
	return waf.handle
}

// ObjectFromJSON parses JSON into a WAFObject.
func (waf *WAFLib) ObjectFromJSON(json []byte, alloc WAFAllocator) (WAFObject, bool) {
	var (
		obj    WAFObject
		pinner runtime.Pinner
	)

	defer pinner.Unpin()
	pinner.Pin(&obj)

	// Pin the slice backing data, not the header
	if jsonData := unsafeutil.SliceData(json); jsonData != nil {
		pinner.Pin(jsonData)
	}

	success := waf.syscall(waf.objectFromJSON,
		uintptr(unsafe.Pointer(&obj)),
		uintptr(unsafe.Pointer(unsafe.SliceData(json))),
		uintptr(len(json)),
		uintptr(alloc),
	) != 0
	return obj, success
}

// syscall is the only way to make C calls with this interface.
// purego implementation limits the number of arguments to 9, it will panic if more are provided
// Note: `purego.SyscallN` has 3 return values: these are the following:
//
//	1st - The return value is a pointer or a int of any type
//	2nd - The return value is a float
//	3rd - The value of `errno` at the end of the call
//
//go:uintptrescapes
func (waf *WAFLib) syscall(fn uintptr, args ...uintptr) uintptr {
	ret, _, _ := purego.SyscallN(fn, args...)
	return ret
}
