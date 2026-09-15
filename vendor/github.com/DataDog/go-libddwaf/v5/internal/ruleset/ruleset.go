// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build (linux || darwin) && !datadog.no_waf

package ruleset

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"errors"
	"fmt"
	"io"

	"github.com/DataDog/go-libddwaf/v5/internal/bindings"
)

//go:embed recommended.json.gz
var defaultRuleset []byte

// DefaultRuleset returns the default ruleset as a WAFObject
// It is the caller's responsibility to free the returned WAFObject using [bindings.Lib.ObjectDestroy]
// when it is no longer needed.
// The returned error is non-nil if the ruleset could not be decompressed or parsed.
func DefaultRuleset() (obj bindings.WAFObject, err error) {
	if ok, err := bindings.Load(); !ok {
		return bindings.WAFObject{}, fmt.Errorf("loading default ruleset: %w", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(defaultRuleset))
	if err != nil {
		return bindings.WAFObject{}, fmt.Errorf("failed to create gzip reader for default ruleset: %w", err)
	}

	defer func() {
		if closeErr := gz.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close gzip reader: %w", closeErr)
		}
	}()

	decompressedRuleset, err := io.ReadAll(gz)
	if err != nil {
		return bindings.WAFObject{}, fmt.Errorf("failed to decompress default ruleset: %w", err)
	}

	parsedRuleset, ok := bindings.Lib.ObjectFromJSON(decompressedRuleset, bindings.Lib.DefaultAllocator())
	if !ok {
		return bindings.WAFObject{}, errors.New("failed to parse default ruleset: ddwaf_object_from_json returned false")
	}
	return parsedRuleset, nil
}
