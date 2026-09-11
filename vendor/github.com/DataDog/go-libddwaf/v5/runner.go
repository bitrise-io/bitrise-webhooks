// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package libddwaf

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/DataDog/go-libddwaf/v5/internal/bindings"
	"github.com/DataDog/go-libddwaf/v5/timer"
	"github.com/DataDog/go-libddwaf/v5/waferrors"
)

// RunAddressData provides address data to the [Context.Run] method.
// Fields tagged `ddwaf:"ignore"` are omitted when encoding Go structs to the WAF-compatible format.
//
// Data passed to Run persists for the lifetime of the context or subcontext.
// Use NewSubcontext() for shorter-lived data.
type RunAddressData struct {
	// Data is passed to the WAF and persists across multiple calls to Run
	// until the context or subcontext closes.
	Data map[string]any

	// TimerKey tracks time spent in the WAF for this run.
	// Leave it empty to start a new timer with unlimited budget.
	TimerKey timer.Key
}

// Runner is the common run surface of [*Context] and [*Subcontext]. It is
// intentionally narrow: only Run belongs here, not Close, Truncations, or Supports.
type Runner interface {
	Run(ctx context.Context, addressData RunAddressData) (Result, error)
}

var _ Runner = (*Context)(nil)
var _ Runner = (*Subcontext)(nil)

func (d RunAddressData) isEmpty() bool {
	return len(d.Data) == 0
}

var runTimerComponents = timer.WithComponents(EncodeTimeKey, DurationTimeKey, DecodeTimeKey)

func newRunTimer(parent timer.NodeTimer, key timer.Key) (timer.NodeTimer, error) {
	if key == "" {
		return timer.NewTreeTimer(
			runTimerComponents,
			timer.WithBudget(parent.SumRemaining()),
		)
	}

	return parent.NewNode(key,
		runTimerComponents,
		timer.WithInheritedSumBudget(),
	)
}

// encodeAddressData encodes address data into WAF objects. Returns the encoded
// WAF object and any truncation info. The caller is responsible for merging
// truncations into its own truncation set.
func encodeAddressData(pinner *runtime.Pinner, addressData map[string]any, t timer.Timer) (*WAFObject, Truncations, error) {
	if addressData == nil {
		return nil, Truncations{}, nil
	}

	encoder, err := newEncoder(newEncoderConfig(pinner, WithTimer(t)))
	if err != nil {
		return nil, Truncations{}, fmt.Errorf("could not create encoder: %w", err)
	}

	data, err := encoder.Encode(addressData)
	if err != nil && !errors.Is(err, waferrors.ErrTimeout) {
		return nil, Truncations{}, fmt.Errorf("failed to encode address data: %w", err)
	}

	if t.Exhausted() {
		return nil, Truncations{}, waferrors.ErrTimeout
	}

	return data, encoder.enc.Truncations, nil
}

// effectiveTimeoutMicros computes the WAF call timeout in microseconds,
// using the minimum of the run timer's remaining budget and ctx's deadline.
func effectiveTimeoutMicros(ctx context.Context, runTimer timer.NodeTimer) uint64 {
	timeoutDuration := runTimer.SumRemaining()
	if deadline, ok := ctx.Deadline(); ok {
		ctxRemaining := time.Until(deadline)
		if ctxRemaining < timeoutDuration || timeoutDuration == timer.UnlimitedBudget {
			timeoutDuration = ctxRemaining
		}
	}
	if timeoutDuration < 0 {
		timeoutDuration = 0
	}
	return uint64(timeoutDuration.Microseconds()) & 0x007FFFFFFFFFFFFF
}

// decodeWafResult decodes the WAF C result into a Go Result, tracking decode
// time and checking for timeout/context-cancellation.
func decodeWafResult(ctx context.Context, ret bindings.WAFReturnCode, result *WAFObject, runTimer timer.NodeTimer) (Result, error) {
	decodeTimer := runTimer.MustLeaf(DecodeTimeKey)
	decodeTimer.Start()
	defer decodeTimer.Stop()

	res, duration, err := unwrapWafResult(ret, result)
	runTimer.AddTime(DurationTimeKey, duration)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return res, ctxErr
	}
	if runTimer.SumExhausted() {
		return res, waferrors.ErrTimeout
	}
	return res, err
}
