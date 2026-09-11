// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package libddwaf

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/DataDog/go-libddwaf/v5/internal/bindings"
	"github.com/DataDog/go-libddwaf/v5/timer"
	"github.com/DataDog/go-libddwaf/v5/waferrors"
)

// Lock order: Subcontext.mu MUST be acquired before parent.Context.mu. Never reverse.

// Subcontext is a derived ephemeral evaluation scope. Spawned via [Context.NewSubcontext].
// Data passed to a Subcontext's Run persists for the Subcontext's lifetime and is
// released when the Subcontext is closed.
//
// # Concurrency
//
// Subcontext.Run may be called concurrently across different Subcontexts of
// the same parent; same-Subcontext Run calls serialize internally.
//
// Subcontext.Close should be called before its parent Context.Close for clean
// teardown; if the parent is already closed, Subcontext.Close is still safe
// (it skips the underlying destroy).
type Subcontext struct {
	Timer         timer.NodeTimer
	parent        *Context
	closedHint    atomic.Bool
	mu            sync.Mutex
	cSub          bindings.WAFSubcontext
	truncationsMu sync.RWMutex
	truncations   Truncations
	pinners       []*runtime.Pinner
	pinnersMu     sync.Mutex
}

// Run encodes the given [RunAddressData] values and runs them against the WAF rules.
func (s *Subcontext) Run(ctx context.Context, addressData RunAddressData) (res Result, err error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("Subcontext.Run: %w", waferrors.ErrNilContext)
	}

	if addressData.isEmpty() {
		if s.closedHint.Load() || s.parent.closedHint.Load() {
			return Result{}, waferrors.ErrContextClosed
		}
		return Result{}, nil
	}

	if s.closedHint.Load() {
		return Result{}, waferrors.ErrContextClosed
	}

	if s.Timer.SumExhausted() {
		return Result{}, waferrors.ErrTimeout
	}

	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	runTimer, err := newRunTimer(s.Timer, addressData.TimerKey)
	if err != nil {
		return Result{}, err
	}
	defer func() { res.TimerStats = runTimer.Stats() }()

	runTimer.Start()
	defer runTimer.Stop()

	pinner := new(runtime.Pinner)
	// wafOwnsData becomes true once the encoded data is handed to the WAF,
	// which retains references to it until the subcontext is destroyed. Until
	// then (encode error, timeout, closed context) the pinned data was never
	// seen by the WAF and can be released immediately.
	wafOwnsData := false

	defer func() {
		if !wafOwnsData {
			pinner.Unpin()
			return
		}
		s.pinnersMu.Lock()
		defer s.pinnersMu.Unlock()
		// Unpin immediately if either this subcontext or its parent context is
		// closing: in both cases the underlying ddwaf resource is (being)
		// destroyed and nothing will reference this data, and the closing path
		// will not observe a pinner appended after it. This mirrors how
		// Context.Run handles its own pinner against context.closedHint.
		if s.closedHint.Load() || s.parent.closedHint.Load() {
			pinner.Unpin()
			return
		}
		s.pinners = append(s.pinners, pinner)
	}()

	wafEncodeTimer := runTimer.MustLeaf(EncodeTimeKey)
	wafEncodeTimer.Start()
	data, truncations, err := encodeAddressData(pinner, addressData.Data, wafEncodeTimer)
	wafEncodeTimer.Stop()
	if err != nil {
		return Result{}, err
	}
	if !truncations.IsEmpty() {
		s.truncationsMu.Lock()
		s.truncations.Merge(truncations)
		s.truncationsMu.Unlock()
	}

	if runTimer.SumExhausted() {
		return Result{}, waferrors.ErrTimeout
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closedHint.Load() {
		return Result{}, waferrors.ErrContextClosed
	}

	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	s.parent.mu.Lock()
	if s.parent.closedHint.Load() {
		s.parent.mu.Unlock()
		return Result{}, waferrors.ErrContextClosed
	}
	s.parent.evalsInFlight.Add(1)
	s.parent.mu.Unlock()
	defer s.parent.evalsInFlight.Done()

	var resultPinner runtime.Pinner
	defer resultPinner.Unpin()
	var result WAFObject
	resultPinner.Pin(&result)
	defer bindings.Lib.ObjectDestroy(&result, bindings.Lib.DefaultAllocator())

	wafOwnsData = true
	ret := bindings.Lib.SubcontextEval(s.cSub, data, 0, &result, effectiveTimeoutMicros(ctx, runTimer))

	return decodeWafResult(ctx, ret, &result, runTimer)
}

// Close disposes of the underlying subcontext and folds its per-scope timer
// durations and truncations into the parent [Context]. After Close returns,
// [Context.Timer] stats and [Context.Truncations] reflect the subcontext's
// accumulated data.
//
// Only runs made with a non-empty [RunAddressData.TimerKey] are captured; runs
// with an empty TimerKey use a standalone timer not attached to the subcontext's
// scope tree, so their time is not rolled up.
//
// For this rollup, only runs that have fully returned before Close are
// guaranteed to be reflected; a Run executing concurrently with Close may miss
// having its timer durations or truncations captured.
//
// This rollup affects only the aggregate [Context.Timer] / [Context.Truncations]
// view, not any [Result.TimerStats] already returned by a prior Run call.
func (s *Subcontext) Close() {
	s.close(false)
}

// Supports reports whether the WAF ruleset behind this subcontext monitors
// addr, via the handle's cached known-address set.
func (s *Subcontext) Supports(addr string) bool {
	return s.parent.handle.Supports(addr)
}

// close tears down the subcontext. parentLocked must be true only when the
// caller already holds s.parent.mu (i.e. Context.Close cascading): the C
// subcontext destroy and the subcontexts-map mutation require parent.mu, and
// sync.Mutex is not reentrant, so we must not re-acquire it on that path.
// Holding parent.mu across SubcontextDestroy keeps it ordered before the
// parent's ContextDestroy and serializes access to the subcontexts map.
func (s *Subcontext) close(parentLocked bool) {
	if !s.closedHint.CompareAndSwap(false, true) {
		return
	}

	if !parentLocked {
		// Barrier: wait for an in-flight Run on this subcontext (which holds
		// s.mu and may still reach SubcontextEval with s.cSub) to finish before
		// we destroy cSub. We then take parent.mu so the SubcontextDestroy is
		// ordered before the parent's ContextDestroy and the subcontexts-map
		// mutation is serialized.
		//
		// Both are skipped on the parentLocked path (Context.Close cascading):
		// acquiring s.mu while the caller holds parent.mu would invert Run's
		// s.mu -> parent.mu lock order and deadlock, and it is unnecessary
		// because Context.Close has already set the parent's closedHint and
		// drained evalsInFlight, so any in-flight Run bails at the
		// parent.closedHint check before using s.cSub.
		s.mu.Lock()
		s.mu.Unlock() //nolint:staticcheck // SA2001: intentional barrier — synchronize with Run before pinner close
		s.parent.mu.Lock()
	}
	if s.cSub != 0 {
		bindings.Lib.SubcontextDestroy(s.cSub)
		s.cSub = 0
	}
	if !parentLocked {
		delete(s.parent.subcontexts, s)
		s.parent.mu.Unlock()
	}

	// Pinner cleanup and handle release do not need parent.mu; keeping it
	// held here would block Run/NewSubcontext on the parent while unpinning.
	s.pinnersMu.Lock()
	for _, p := range s.pinners {
		p.Unpin()
	}
	s.pinners = nil
	s.pinnersMu.Unlock()

	s.rollupInto(s.parent)
	s.parent.handle.Close()
}

// rollupInto merges this subcontext's per-scope timer durations and truncations
// into parent. Called from close(), it runs on both close paths (explicit
// Subcontext.Close and Context.Close cascade).
//
// Lock discipline: MUST NOT acquire s.mu or parent.mu. On the cascade path
// (parentLocked=true), parent.mu is already held by Context.Close; re-acquiring
// it self-deadlocks. Only atomic parent.Timer.AddTime and the independent
// parent.truncationsMu are used.
func (s *Subcontext) rollupInto(parent *Context) {
	// Per-scope timer rollup: AddTime is atomic and a silent no-op for keys
	// the parent timer does not have (e.g. "encode"/"duration"/"decode" when
	// the parent was created with no named components).
	for key, dur := range s.Timer.Stats() {
		parent.Timer.AddTime(key, dur)
	}

	// Truncation rollup: read under s.truncationsMu because on the cascade
	// path (parentLocked=true, s.mu not held) a concurrent Subcontext.Run may
	// have written s.truncations after the parent-closed check and before
	// close(true) was called.
	s.truncationsMu.RLock()
	local := Truncations{
		StringTooLong:     append([]int(nil), s.truncations.StringTooLong...),
		ContainerTooLarge: append([]int(nil), s.truncations.ContainerTooLarge...),
		ObjectTooDeep:     append([]int(nil), s.truncations.ObjectTooDeep...),
	}
	s.truncationsMu.RUnlock()

	if local.IsEmpty() {
		return
	}

	parent.truncationsMu.Lock()
	parent.truncations.Merge(local)
	parent.truncationsMu.Unlock()
}

// Truncations returns the truncations that occurred while encoding address data for WAF execution.
func (s *Subcontext) Truncations() Truncations {
	s.truncationsMu.RLock()
	defer s.truncationsMu.RUnlock()
	return Truncations{
		StringTooLong:     append([]int(nil), s.truncations.StringTooLong...),
		ContainerTooLarge: append([]int(nil), s.truncations.ContainerTooLarge...),
		ObjectTooDeep:     append([]int(nil), s.truncations.ObjectTooDeep...),
	}
}
