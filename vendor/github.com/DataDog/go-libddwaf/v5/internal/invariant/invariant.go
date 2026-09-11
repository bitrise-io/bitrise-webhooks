// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package invariant provides a runtime-invariant assertion helper that panics
// when the condition is violated and the build is compiled with the "ci"
// build tag. In production builds (no "ci" tag) the function is a no-op.
//
// This package intentionally has a minimal API. DO NOT add new functions
// without a corresponding ADR.
//
// The "ci" build tag is set by .github/workflows/ci.sh and is project-internal.
// Downstream consumers MUST NOT set this build tag.
package invariant

// Assert panics with a formatted message when cond is false AND the build
// has the "ci" tag. In production builds (no tag), it is a no-op.
//
// Callers MUST NOT rely on Assert for control flow in production code paths.
func Assert(cond bool, format string, args ...any) {
	assertImpl(cond, format, args...)
}

// Active reports whether invariant checks are compiled in (the "ci" build tag).
// It returns a build-specific constant, so guarding invariant-only bookkeeping
// with Active() lets the compiler eliminate it in production builds.
func Active() bool { return active }
