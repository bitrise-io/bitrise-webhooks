// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package libddwaf

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/DataDog/go-libddwaf/v5/internal/bindings"
	"github.com/DataDog/go-libddwaf/v5/internal/invariant"
	"github.com/DataDog/go-libddwaf/v5/timer"
	"github.com/DataDog/go-libddwaf/v5/waferrors"
)

// Handle represents an instance of the WAF for a given ruleset. It is obtained
// from [Builder.Build]; and must be disposed of by calling [Handle.Close]
// once no longer in use.
//
// The reference count equals the number of alive [Context] values plus the
// number of alive [Subcontext] values. Each [Handle.NewContext] call retains the
// handle, each [Context.Close] releases it, each [Context.NewSubcontext] call
// retains it, and each [Subcontext.Close] releases it.
//
// Library lifetime boundary: [Handle.retain] and [Handle.Close] only govern the
// lifetime of the underlying C ddwaf_handle. They do not keep the libddwaf
// shared library loaded. That shared library is a process-wide singleton
// loaded once via purego.Dlopen (see internal/bindings) and is never Dlclosed
// during normal operation. As a consequence, symbols such as
// bindings.Lib.ContextDestroy or bindings.Lib.SubcontextDestroy remain
// resolvable for the entire lifetime of the process, and Context/SubContext
// teardown paths that call into bindings.Lib after the Handle's refcount has
// reached zero are safe with respect to the library itself.
type Handle struct {
	// Lock-less reference counter avoiding blocking calls to the [Handle.Close]
	// method while WAF [Context]s are still using the WAF handle. Instead, we let
	// the release actually happen only when the reference counter reaches 0.
	// This can happen either from a request handler calling its WAF context's
	// [Context.Close] method, or either from the appsec instance calling the WAF
	// [Handle.Close] method when creating a new WAF handle with new rules.
	// Note that this means several instances of the WAF can exist at the same
	// time with their own set of rules. This choice was done to be able to
	// efficiently update the security rules concurrently, without having to
	// block the request handlers for the time of the security rules update.
	refCounter atomic.Int32

	cHandle bindings.WAFHandle

	// addrOnce guards the one-time population of addrSet from Handle.Addresses().
	// The C layer (KnownAddresses) is called at most once per handle lifetime.
	addrOnce sync.Once
	// addrSet is the lazily-built snapshot of known addresses for this handle.
	// Populated under addrOnce.Do; nil until first Supports call on a live handle.
	addrSet map[string]struct{}
}

// wrapHandle wraps the provided C handle into a [Handle]. The caller is
// responsible to ensure the cHandle value is not 0 (NULL). The returned
// [Handle] has a reference count of 1, so callers need not call [Handle.retain]
// on it.
func wrapHandle(cHandle bindings.WAFHandle) *Handle {
	handle := &Handle{cHandle: cHandle}
	handle.refCounter.Store(1)
	return handle
}

func validateConstructionContext(op string, ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%s: %w", op, waferrors.ErrNilContext)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// NewContext returns a new WAF context for the given WAF handle.
// The provided ctx only scopes the construction call itself and is not retained
// after NewContext returns; per-run deadlines and cancellation must be supplied
// to [Context.Run] via its own ctx argument.
// An error is returned when the WAF handle was released or when the WAF context
// couldn't be created.
func (handle *Handle) NewContext(ctx context.Context, timerOptions ...timer.Option) (*Context, error) {
	if err := validateConstructionContext("Handle.NewContext", ctx); err != nil {
		return nil, err
	}

	if !handle.retain() {
		return nil, waferrors.ErrHandleReleased
	}

	cContext := bindings.Lib.ContextInit(handle.cHandle, bindings.Lib.DefaultAllocator())
	if cContext == 0 {
		handle.Close() // We couldn't get a context, so we no longer have an implicit reference to the Handle in it...
		return nil, fmt.Errorf("%w: ddwaf_context_init returned null", waferrors.ErrContextInitFailed)
	}

	rootTimer, err := timer.NewTreeTimer(timerOptions...)
	if err != nil {
		bindings.Lib.ContextDestroy(cContext)
		handle.Close()
		return nil, fmt.Errorf("failed to create WAF context timer: %w", err)
	}

	return &Context{
		handle:   handle,
		Timer:    rootTimer,
		cContext: cContext,
	}, nil
}

// Addresses returns the list of addresses the WAF has been configured to monitor based on the input
// ruleset.
func (handle *Handle) Addresses() []string {
	return bindings.Lib.KnownAddresses(handle.cHandle)
}

// Supports reports whether addr is a member of this handle's known-address set
// (the authoritative monitored-address set returned by [Handle.Addresses] /
// KnownAddresses). This is NOT a diagnostics-derived set.
//
// The address set is built lazily on the first call and cached; the underlying
// C layer (KnownAddresses) is called at most once per handle. The set is a
// snapshot of the immutable handle, so subsequent calls are consistent.
//
// Supports is safe to call concurrently with [Handle.Close]. It retains the
// handle across the KnownAddresses call so a concurrent final Close cannot
// destroy the C handle mid-call (use-after-free). If the handle is released
// before the set is ever built, Supports returns false; a cached set remains
// valid and read-only after construction.
func (handle *Handle) Supports(addr string) bool {
	// Fast path: a released handle monitors nothing. refCounter is atomic, so
	// this read is race-free (unlike reading cHandle directly).
	if handle.refCounter.Load() <= 0 {
		return false
	}
	handle.addrOnce.Do(func() {
		// Retain across the KnownAddresses call so a concurrent final Close
		// cannot Destroy the C handle mid-call (use-after-free). If the handle
		// was released between the fast-path check and here, retain fails and
		// addrSet stays nil (-> Supports returns false).
		if !handle.retain() {
			return
		}
		defer handle.Close()
		addrs := handle.Addresses()
		set := make(map[string]struct{}, len(addrs))
		for _, a := range addrs {
			set[a] = struct{}{}
		}
		handle.addrSet = set
	})
	_, ok := handle.addrSet[addr]
	return ok
}

// Actions returns the list of actions the WAF has been configured to monitor based on the input
// ruleset.
func (handle *Handle) Actions() []string {
	return bindings.Lib.KnownActions(handle.cHandle)
}

// Close decrements the reference counter of this [Handle], possibly allowing it to be destroyed
// and all the resources associated with it to be released.
func (handle *Handle) Close() {
	if handle == nil {
		return
	}
	if handle.addRefCounter(-1) != 0 {
		return
	}

	bindings.Lib.Destroy(handle.cHandle)
	handle.cHandle = 0
}

// retain increments the reference counter of this [Handle]. Returns true if the
// [Handle] is still valid, false if it is no longer usable. Calls to
// [Handle.retain] must be balanced with calls to [Handle.Close] in order to
// avoid leaking [Handle]s.
func (handle *Handle) retain() bool {
	return handle.addRefCounter(1) > 0
}

// addRefCounter adds x to Handle.refCounter. The return valid indicates whether the refCounter
// reached 0 as part of this call or not, which can be used to perform "only-once" activities:
//
// * result > 0    => the Handle is still usable
// * result == 0   => the handle is no longer usable, ref counter reached 0 as part of this call
// * result == -1  => the handle is no longer usable, ref counter was already 0 previously
func (handle *Handle) addRefCounter(x int32) int32 {
	// We use a CAS loop to avoid setting the refCounter to a negative value.
	for {
		current := handle.refCounter.Load()
		if current <= 0 {
			// The object had already been released
			return -1
		}

		next := current + x
		if next < 0 {
			if swapped := handle.refCounter.CompareAndSwap(current, 0); swapped {
				// Refcount underflow is surfaced via internal/invariant under ci builds (see ADR-003).
				invariant.Assert(false, "refCounter went negative: current=%d, delta=%d", current, x)
				return 0
			}
			continue
		}

		if swapped := handle.refCounter.CompareAndSwap(current, next); swapped {
			return next
		}
	}
}
