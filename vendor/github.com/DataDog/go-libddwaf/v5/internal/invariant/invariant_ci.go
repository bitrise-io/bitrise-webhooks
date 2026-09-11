// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build ci

package invariant

import "fmt"

func assertImpl(cond bool, format string, args ...any) {
	if !cond {
		panic(fmt.Sprintf("invariant violated: "+format, args...))
	}
}

const active = true
