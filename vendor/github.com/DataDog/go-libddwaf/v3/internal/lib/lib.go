// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm64))) && !go1.24 && !datadog.no_waf && (cgo || appsec)

package lib

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"

	_ "embed"
)

//go:embed .version
var EmbeddedWAFVersion string

func DumpEmbeddedWAF() (path string, err error) {
	file, err := os.CreateTemp("", embedNamePattern)
	if err != nil {
		return path, fmt.Errorf("error creating temp file: %w", err)
	}
	path = file.Name()

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("error closing file: %w", closeErr))
		}
		if path != "" && err != nil {
			if rmErr := os.Remove(path); rmErr != nil {
				err = errors.Join(err, fmt.Errorf("error removing file: %w", rmErr))
			}
		}
	}()

	gr, err := gzip.NewReader(bytes.NewReader(libddwaf))
	if err != nil {
		return path, fmt.Errorf("error creating gzip reader: %w", err)
	}

	uncompressedLibddwaf, err := io.ReadAll(gr)
	if err != nil {
		return path, fmt.Errorf("error reading gzip content: %w", err)
	}

	if err := gr.Close(); err != nil {
		return path, fmt.Errorf("error closing gzip reader: %w", err)
	}

	if err := os.WriteFile(file.Name(), uncompressedLibddwaf, 0400); err != nil {
		return path, fmt.Errorf("error writing file: %w", err)
	}

	return path, nil
}
