// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build (linux || darwin) && (amd64 || arm64) && !go1.28 && !datadog.no_waf && (cgo || appsec)

package bindings

import (
	"fmt"

	"github.com/ebitengine/purego"
)

type wafSymbols struct {
	builderInit              uintptr
	builderAddOrUpdateConfig uintptr
	builderRemoveConfig      uintptr
	builderBuildInstance     uintptr
	builderGetConfigPaths    uintptr
	builderDestroy           uintptr
	destroy                  uintptr
	knownAddresses           uintptr
	knownActions             uintptr
	getVersion               uintptr
	contextInit              uintptr
	contextEval              uintptr
	contextDestroy           uintptr
	subcontextInit           uintptr
	subcontextEval           uintptr
	subcontextDestroy        uintptr
	objectDestroy            uintptr
	objectFromJSON           uintptr
	getDefaultAllocator      uintptr
	setLogCb                 uintptr
}

// newWafSymbols resolves the symbols of [wafSymbols] from the provided
// [purego.Dlopen] handle.
func newWafSymbols(handle uintptr) (syms wafSymbols, err error) {
	resolve := func(name string) (uintptr, error) {
		sym, err := purego.Dlsym(handle, name)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve libddwaf symbol %q: %w", name, err)
		}
		return sym, nil
	}

	if syms.builderAddOrUpdateConfig, err = resolve("ddwaf_builder_add_or_update_config"); err != nil {
		return syms, err
	}
	if syms.builderBuildInstance, err = resolve("ddwaf_builder_build_instance"); err != nil {
		return syms, err
	}
	if syms.builderDestroy, err = resolve("ddwaf_builder_destroy"); err != nil {
		return syms, err
	}
	if syms.builderGetConfigPaths, err = resolve("ddwaf_builder_get_config_paths"); err != nil {
		return syms, err
	}
	if syms.builderInit, err = resolve("ddwaf_builder_init"); err != nil {
		return syms, err
	}
	if syms.builderRemoveConfig, err = resolve("ddwaf_builder_remove_config"); err != nil {
		return syms, err
	}
	if syms.contextDestroy, err = resolve("ddwaf_context_destroy"); err != nil {
		return syms, err
	}
	if syms.contextInit, err = resolve("ddwaf_context_init"); err != nil {
		return syms, err
	}
	if syms.contextEval, err = resolve("ddwaf_context_eval"); err != nil {
		return syms, err
	}
	if syms.subcontextInit, err = resolve("ddwaf_subcontext_init"); err != nil {
		return syms, err
	}
	if syms.subcontextEval, err = resolve("ddwaf_subcontext_eval"); err != nil {
		return syms, err
	}
	if syms.subcontextDestroy, err = resolve("ddwaf_subcontext_destroy"); err != nil {
		return syms, err
	}
	if syms.destroy, err = resolve("ddwaf_destroy"); err != nil {
		return syms, err
	}
	if syms.getVersion, err = resolve("ddwaf_get_version"); err != nil {
		return syms, err
	}
	if syms.knownActions, err = resolve("ddwaf_known_actions"); err != nil {
		return syms, err
	}
	if syms.knownAddresses, err = resolve("ddwaf_known_addresses"); err != nil {
		return syms, err
	}
	if syms.objectDestroy, err = resolve("ddwaf_object_destroy"); err != nil {
		return syms, err
	}
	if syms.objectFromJSON, err = resolve("ddwaf_object_from_json"); err != nil {
		return syms, err
	}
	if syms.getDefaultAllocator, err = resolve("ddwaf_get_default_allocator"); err != nil {
		return syms, err
	}
	if syms.setLogCb, err = resolve("ddwaf_set_log_cb"); err != nil {
		return syms, err
	}
	return syms, nil
}
