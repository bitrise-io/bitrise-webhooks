// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2022 Datadog, Inc.

package timer

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// baseTimer is the type used for all the timers that won't have children
// It's implementation is more lightweight than nodeTimer and can be used as a standalone timer using NewTimer
type baseTimer struct {

	// config is the configuration of the timer
	config config

	// start is the time when the timer was started
	start atomic.Pointer[time.Time]
	// startValue stores the start time once it has been published via start.
	startValue time.Time
	// startOnce guards the non-atomic writes in Start (startValue, budget
	// resolution) against concurrent Start calls.
	startOnce sync.Once

	// parent is the parent timer. It is used to progate the stop of the timer to the parent timer and get the remaining time in case the budget has to be inherited.
	parent NodeTimer

	// componentName is the name of the component of the timer. It is used to store the time spent in the component and to propagate the stop of the timer to the parent timer.
	componentName Key

	// budget stores the resolved inherited budget once Start() has computed it.
	budget atomic.Int64
	// budgetResolved is true once budget has been resolved from the parent.
	budgetResolved atomic.Bool

	// spent is the time spent on the timer, set after calling stop
	spent atomic.Int64
	// stopped is true if the timer has been stopped
	stopped atomic.Bool
}

var _ Timer = (*baseTimer)(nil)

// NewTimer creates a new Timer with the given options. You have to specify either the option WithBudget or WithUnlimitedBudget.
func NewTimer(options ...Option) (Timer, error) {
	config := newConfig(options...)
	if config.budget == DynamicBudget {
		return nil, errors.New("root timer cannot inherit parent budget, please provide a budget using timer.WithBudget() or timer.WithUnlimitedBudget()")
	}

	if len(config.components) > 0 {
		return nil, errors.New("NewTimer: timer that have components must use NewTreeTimer()")
	}

	return &baseTimer{
		config: config,
	}, nil
}

func (timer *baseTimer) Start() time.Time {
	// already started before
	if start := timer.start.Load(); start != nil {
		return *start
	}

	timer.startOnce.Do(func() {
		if timer.budgetValue() == DynamicBudget && timer.parent != nil {
			timer.budget.Store(int64(timer.config.dynamicBudget(timer.parent)))
			timer.budgetResolved.Store(true)
		}

		timer.startValue = timer.now()
		timer.start.Store(&timer.startValue)
	})

	return *timer.start.Load()
}

func (timer *baseTimer) now() time.Time {
	return time.Now()
}

func (timer *baseTimer) Spent() time.Duration {
	// timer was already stopped
	if timer.stopped.Load() {
		return time.Duration(timer.spent.Load())
	}

	// timer was never started
	start := timer.start.Load()
	if start == nil {
		return 0
	}

	return time.Since(*start)
}

func (timer *baseTimer) Remaining() time.Duration {
	budget := timer.budgetValue()
	if budget == UnlimitedBudget || budget == DynamicBudget {
		return UnlimitedBudget
	}

	remaining := budget - timer.Spent()
	if remaining < 0 {
		return 0
	}

	return remaining
}

func (timer *baseTimer) Exhausted() bool {
	budget := timer.budgetValue()
	if budget == UnlimitedBudget || budget == DynamicBudget {
		return false
	}

	return timer.Spent() > budget
}

func (timer *baseTimer) budgetValue() time.Duration {
	if timer.config.budget != DynamicBudget {
		return timer.config.budget
	}

	if timer.budgetResolved.Load() {
		return time.Duration(timer.budget.Load())
	}

	return DynamicBudget
}

func (timer *baseTimer) Stop() time.Duration {
	// If the current timer has already stopped, return the current spent time
	if timer.stopped.Load() {
		return time.Duration(timer.spent.Load())
	}

	spent := timer.Spent()
	timer.spent.Store(int64(spent))
	// CAS ensures exactly one caller propagates the stop to the parent;
	// concurrent Stop calls would otherwise double-count the spent time
	// into the parent's component accumulator.
	if !timer.stopped.CompareAndSwap(false, true) {
		return time.Duration(timer.spent.Load())
	}
	if timer.parent != nil {
		timer.parent.childStopped(timer.componentName, spent)
	}

	return spent
}

func (timer *baseTimer) Timed(timedFunc func(timer Timer)) (spent time.Duration) {
	timer.Start()
	defer func() {
		spent = timer.Stop()
	}()

	timedFunc(timer)
	return
}
