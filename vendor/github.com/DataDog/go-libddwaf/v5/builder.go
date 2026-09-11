// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package libddwaf

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"

	"github.com/DataDog/go-libddwaf/v5/internal/bindings"
	"github.com/DataDog/go-libddwaf/v5/internal/invariant"
	"github.com/DataDog/go-libddwaf/v5/internal/ruleset"
	"github.com/DataDog/go-libddwaf/v5/waferrors"
)

// Builder manages an evolving WAF configuration.
// Builder is not thread-safe. Concurrent use panics under `ci` builds.
type Builder struct {
	handle        bindings.WAFBuilder
	defaultLoaded bool
	inUse         atomic.Bool // detects concurrent use under ci builds
}

// acquire marks the builder as in use. Panics under ci builds if already in
// use, indicating a concurrent use violation. The atomic bookkeeping is gated
// on invariant.Active(), so it is dead-code-eliminated in production builds.
func (b *Builder) acquire() {
	if invariant.Active() && !b.inUse.CompareAndSwap(false, true) {
		invariant.Assert(false, "Builder used concurrently")
	}
}

func (b *Builder) release() {
	if invariant.Active() {
		b.inUse.Store(false)
	}
}

// NewBuilder creates a new [Builder] instance.
// The caller must call [Builder.Close] when it is no longer needed.
func NewBuilder() (*Builder, error) {
	if ok, err := Load(); !ok {
		if err != nil {
			return nil, fmt.Errorf("failed to load WAF library while creating builder: %w", err)
		}
		return nil, errors.New("failed to load WAF library while creating builder")
	}

	hdl := bindings.Lib.BuilderInit()
	if hdl == 0 {
		return nil, waferrors.ErrBuilderInitFailed
	}

	return &Builder{handle: hdl}, nil
}

// Close releases all resources associated with this builder.
func (b *Builder) Close() {
	if b == nil || b.handle == 0 {
		return
	}
	bindings.Lib.BuilderDestroy(b.handle)
	b.handle = 0
}

var (
	errUpdateFailed  = errors.New("failed to update WAF Builder instance")
	errBuilderClosed = errors.New("builder has already been closed")
)

const defaultRecommendedRulesetPath = "::/go-libddwaf/default/recommended.json"

// AddDefaultRecommendedRuleset adds the default recommended ruleset to the
// receiving [Builder], and returns the [Diagnostics] produced in the process.
func (b *Builder) AddDefaultRecommendedRuleset() (Diagnostics, error) {
	b.acquire()
	defer b.release()

	defaultRuleset, err := ruleset.DefaultRuleset()
	if err != nil {
		return Diagnostics{}, fmt.Errorf("failed to load default recommended ruleset: %w", err)
	}
	defer bindings.Lib.ObjectDestroy(&defaultRuleset, bindings.Lib.DefaultAllocator())

	diag, err := b.addOrUpdateConfig(defaultRecommendedRulesetPath, &defaultRuleset)
	if err == nil {
		b.defaultLoaded = true
	}
	return diag, err
}

// RemoveDefaultRecommendedRuleset removes the default recommended ruleset from
// the receiving [Builder]. Returns true if the removal occurred (meaning the
// default recommended ruleset was indeed present in the builder).
func (b *Builder) RemoveDefaultRecommendedRuleset() bool {
	if b.RemoveConfig(defaultRecommendedRulesetPath) {
		b.defaultLoaded = false
		return true
	}
	return false
}

// AddOrUpdateConfig adds or updates a configuration fragment to this [Builder].
// Returns the [Diagnostics] produced by adding or updating this configuration.
func (b *Builder) AddOrUpdateConfig(path string, fragment any) (Diagnostics, error) {
	if b == nil || b.handle == 0 {
		return Diagnostics{}, errBuilderClosed
	}

	b.acquire()
	defer b.release()

	if path == "" {
		return Diagnostics{}, errors.New("path cannot be blank")
	}

	var pinner runtime.Pinner
	defer pinner.Unpin()

	encoder, err := newEncoder(newEncoderConfig(&pinner, WithUnlimitedLimits()))
	if err != nil {
		return Diagnostics{}, fmt.Errorf("could not create encoder: %w", err)
	}

	frag, err := encoder.Encode(fragment)
	if err != nil {
		return Diagnostics{}, fmt.Errorf("could not encode the config fragment into a WAF object: %w", err)
	}

	return b.addOrUpdateConfig(path, frag)
}

// addOrUpdateConfig adds or updates a configuration fragment to this [Builder].
// Returns the [Diagnostics] produced by adding or updating this configuration.
func (b *Builder) addOrUpdateConfig(path string, cfg *WAFObject) (Diagnostics, error) {
	var diagnosticsWafObj WAFObject
	defer bindings.Lib.ObjectDestroy(&diagnosticsWafObj, bindings.Lib.DefaultAllocator())

	res := bindings.Lib.BuilderAddOrUpdateConfig(b.handle, path, cfg, &diagnosticsWafObj)

	var diags Diagnostics
	if !diagnosticsWafObj.IsInvalid() {
		// The Diagnostics object will be invalid if the config was completely
		// rejected.
		var err error
		diags, err = decodeDiagnostics(&diagnosticsWafObj)
		if err != nil {
			return diags, fmt.Errorf("failed to decode WAF diagnostics: %w", err)
		}
	}

	if !res {
		return diags, errUpdateFailed
	}
	return diags, nil
}

// RemoveConfig removes the configuration associated with the given path from
// this [Builder]. Returns true if the removal was successful.
func (b *Builder) RemoveConfig(path string) bool {
	if b == nil || b.handle == 0 {
		return false
	}

	b.acquire()
	defer b.release()

	return bindings.Lib.BuilderRemoveConfig(b.handle, path)
}

// ConfigPaths returns the list of currently loaded configuration paths.
func (b *Builder) ConfigPaths(filter string) ([]string, error) {
	if b == nil || b.handle == 0 {
		return nil, errBuilderClosed
	}

	b.acquire()
	defer b.release()

	return bindings.Lib.BuilderGetConfigPaths(b.handle, filter)
}

// Build creates a new [Handle] instance that uses the current configuration.
// Returns an error if the builder is not initialized or the C library fails to
// build the handle. The caller is responsible for calling [Handle.Close] when
// the handle is no longer needed.
func (b *Builder) Build() (*Handle, error) {
	if b == nil || b.handle == 0 {
		return nil, waferrors.ErrBuilderInitFailed
	}

	b.acquire()
	defer b.release()

	hdl := bindings.Lib.BuilderBuildInstance(b.handle)
	if hdl == 0 {
		return nil, errors.New("BuilderBuildInstance returned null")
	}

	return wrapHandle(hdl), nil
}
