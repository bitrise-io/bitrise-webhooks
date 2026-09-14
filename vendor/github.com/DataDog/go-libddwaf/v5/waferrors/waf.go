// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package waferrors

import (
	"errors"
	"fmt"
)

var (
	// ErrContextClosed is returned when an operation is attempted on a
	// [github.com/DataDog/go-libddwaf/v5.Context] that has already been closed.
	ErrContextClosed = errors.New("closed WAF context")

	// ErrMaxDepthExceeded is returned when the WAF encounters a value that
	// exceeds the maximum depth.
	ErrMaxDepthExceeded = errors.New("max depth exceeded")
	// ErrUnsupportedValue is returned when the WAF encounters a value that
	// is not supported by the encoder or decoder.
	ErrUnsupportedValue = errors.New("unsupported Go value")
	// ErrInvalidMapKey is returned when the WAF encounters an invalid map key.
	ErrInvalidMapKey = errors.New("invalid WAF object map key")
	// ErrNilObjectPtr is returned when the WAF encounters a nil object pointer at
	// an unexpected location.
	ErrNilObjectPtr = errors.New("nil WAF object pointer")
	// ErrInvalidObjectType is returned when the WAF encounters an invalid type
	// when decoding a value.
	ErrInvalidObjectType = errors.New("invalid type encountered when decoding")
	// ErrTooManyIndirections is returned when the WAF encounters a value that
	// exceeds the maximum number of indirections (pointer to pointer to...).
	ErrTooManyIndirections = errors.New("too many indirections")
	// ErrHandleReleased is returned when an operation is attempted on a
	// [github.com/DataDog/go-libddwaf/v5.Handle] whose reference count has
	// reached zero.
	ErrHandleReleased = errors.New("WAF handle has been released")

	// ErrBuilderInitFailed is returned when the C library fails to allocate
	// a WAF builder.
	ErrBuilderInitFailed = errors.New("failed to initialize the WAF builder")

	// ErrContextInitFailed is returned when the C library fails to allocate
	// a WAF context.
	ErrContextInitFailed = errors.New("failed to initialize WAF context")

	// ErrUnknownReturnCode wraps an unexpected return code from the C library.
	ErrUnknownReturnCode = errors.New("unknown WAF return code")

	// ErrResultInvalidType is returned when unwrapping a WAF result encounters
	// an object whose type is not expected for that field.
	ErrResultInvalidType = errors.New("invalid WAF result object type")

	// ErrNilContext is returned when a nil context.Context is passed where a
	// non-nil one is required.
	ErrNilContext = errors.New("nil context.Context")

	// ErrBinaryNotString is returned by the decoder when a value expected to be
	// a string is not actually a string/[]byte.
	ErrBinaryNotString = errors.New("WAF object value is not a string")
)

// RunError represents an error returned by the WAF during a run.
type RunError int

// RunError values returned by the WAF.
const (
	// ErrInternal denotes a WAF internal error.
	ErrInternal RunError = iota + 1
	// ErrInvalidObject is returned when the WAF received an invalid object.
	ErrInvalidObject
	// ErrInvalidArgument is returned when the WAF received an invalid argument.
	ErrInvalidArgument
	// ErrTimeout is returned when the WAF ran out of time budget to spend.
	ErrTimeout
	// ErrOutOfMemory is returned when the WAF ran out of memory when trying to
	// allocate a result object.
	ErrOutOfMemory
	// ErrEmptyRuleAddresses is returned when the WAF received an empty list of
	// rule addresses.
	ErrEmptyRuleAddresses
)

// Error returns the string representation of the [RunError].
func (e RunError) Error() string {
	switch e {
	case ErrInternal:
		return "internal waf error"
	case ErrInvalidObject:
		return "invalid waf object"
	case ErrInvalidArgument:
		return "invalid waf argument"
	case ErrTimeout:
		return "waf timeout"
	case ErrOutOfMemory:
		return "out of memory"
	case ErrEmptyRuleAddresses:
		return "empty rule addresses"
	default:
		return fmt.Sprintf("unknown waf error %d", int(e))
	}
}

// ToWafErrorCode converts an error to a WAF error code, returns zero if the
// error is not a [RunError].
func ToWafErrorCode(in error) int {
	var runError RunError
	if !errors.As(in, &runError) {
		return 0
	}
	return int(runError)
}

// PanicError is an error type wrapping a recovered panic value that happened
// during a function call. Such error must be considered unrecoverable and be
// used to try to gracefully abort. Keeping using this package after such an
// error is unreliable and the caller must rather stop using the library.
// Examples include safety checks errors.
type PanicError struct {
	// The recovered panic error while executing the function `in`.
	Err error
	// The function symbol name that was given to `tryCall()`.
	In string
}

// Unwrap the error and return it.
// Required by errors.Is and errors.As functions.
func (e *PanicError) Unwrap() error {
	return e.Err
}

// Error returns the error string representation.
func (e *PanicError) Error() string {
	return fmt.Sprintf("panic while executing %s: %v", e.In, e.Err)
}
