// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016 Datadog, Inc.

<<<<<<<< HEAD:vendor/github.com/DataDog/dd-trace-go/v2/ddtrace/ext/system.go
package ext

// Standard system metadata names
const (
	// The pid of the traced process
	Pid = "process_id"
)
========
package config

import (
	"net/http"
)

// WithResourceNamer populates the name of a resource based on a custom function.
func WithResourceNamer(namer func(req *http.Request) string) OptionFn {
	return func(cfg *CommonConfig) {
		cfg.ResourceNamer = namer
	}
}
>>>>>>>> origin/master:vendor/github.com/DataDog/dd-trace-go/contrib/net/http/v2/internal/config/options.go
