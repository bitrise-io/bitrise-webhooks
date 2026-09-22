// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build !ci

package invariant

func assertImpl(cond bool, format string, args ...any) {
	// no-op in production builds
	_ = cond
	_ = format
	_ = args
}

const active = false
