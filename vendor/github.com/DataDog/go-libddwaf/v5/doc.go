// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package libddwaf provides Go bindings for libddwaf — Datadog's in-app
// Web Application Firewall evaluation engine.
//
// The package offers a high-level API built around three core types:
//
//   - [Builder]: constructs a [Handle] from one or more ruleset fragments.
//   - [Handle]: holds a compiled ruleset; creates per-request [Context]s.
//   - [Context]: evaluates input data against the ruleset.
//   - [Subcontext]: scopes ephemeral data to a shorter lifetime within a [Context].
//
// A typical usage pattern:
//
//	builder, err := libddwaf.NewBuilder()
//	if err != nil { /* handle */ }
//	defer builder.Close()
//	if _, err := builder.AddOrUpdateConfig("/rules", ruleset); err != nil { /* handle */ }
//	handle, err := builder.Build()
//	if err != nil { /* handle */ }
//	defer handle.Close()
//	ctx, err := handle.NewContext(context.Background())
//	if err != nil { /* handle */ }
//	defer ctx.Close()
//	result, err := ctx.Run(context.Background(), libddwaf.RunAddressData{Data: input})
//	subCtx, err := ctx.NewSubcontext(context.Background())
//	// ...
//	result, err = subCtx.Run(context.Background(), libddwaf.RunAddressData{Data: ephemeral})
//
// For supported platforms, loading mechanics, and CGO/purego trade-offs, see
// the repository README.
package libddwaf
