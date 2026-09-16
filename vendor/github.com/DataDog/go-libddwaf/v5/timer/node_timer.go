// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2022 Datadog, Inc.

package timer

import (
	"errors"
	"fmt"
	"time"
)

// nodeTimer is the type used for all the timers that can (and will) have children
type nodeTimer struct {
	baseTimer
	components
}

var _ NodeTimer = (*nodeTimer)(nil)

// NewTreeTimer creates a new Timer with the given options. You have to specify either the option WithBudget or WithUnlimitedBudget and at least one component using the WithComponents option.
func NewTreeTimer(options ...Option) (NodeTimer, error) {
	config := newConfig(options...)
	if config.budget == DynamicBudget {
		return nil, errors.New("root timer cannot inherit parent budget, please provide a budget using timer.WithBudget() or timer.WithUnlimitedBudget()")
	}

	return &nodeTimer{
		baseTimer: baseTimer{
			config: config,
		},
		components: newComponents(config.components),
	}, nil
}

func (timer *nodeTimer) NewNode(name Key, options ...Option) (NodeTimer, error) {
	config := newConfig(options...)
	if len(config.components) == 0 {
		return nil, errors.New("NewNode: node timer must have at least one component, otherwise use NewLeaf()")
	}

	_, ok := timer.lookup[name]
	if !ok {
		return nil, fmt.Errorf("NewNode: component %s not found", name)
	}

	return &nodeTimer{
		baseTimer: baseTimer{
			config:        config,
			parent:        timer,
			componentName: name,
		},
		components: newComponents(config.components),
	}, nil
}

func (timer *nodeTimer) NewLeaf(name Key, options ...Option) (Timer, error) {
	config := newConfig(options...)
	if len(config.components) != 0 {
		return nil, errors.New("NewLeaf: leaf timer cannot have components, otherwise use NewNode()")
	}

	_, ok := timer.lookup[name]
	if !ok {
		return nil, fmt.Errorf("NewLeaf: component %s not found", name)
	}

	return &baseTimer{
		config:        config,
		componentName: name,
		parent:        timer,
	}, nil
}

var defaultLeafConfig = newConfig()

func (timer *nodeTimer) MustLeaf(name Key, options ...Option) Timer {
	if len(options) == 0 {
		if _, ok := timer.lookup[name]; !ok {
			panic(fmt.Sprintf("MustLeaf: component %s not found", name))
		}
		return &baseTimer{
			config:        defaultLeafConfig,
			componentName: name,
			parent:        timer,
		}
	}
	leaf, err := timer.NewLeaf(name, options...)
	if err != nil {
		panic(err)
	}
	return leaf
}

func (timer *nodeTimer) childStarted() {}

func (timer *nodeTimer) childStopped(componentName Key, duration time.Duration) {
	timer.components.lookup[componentName].Add(int64(duration))
}

func (timer *nodeTimer) AddTime(name Key, duration time.Duration) {
	value, ok := timer.lookup[name]
	if !ok {
		return
	}

	value.Add(int64(duration))
}

func (timer *nodeTimer) Stats() map[Key]time.Duration {
	stats := make(map[Key]time.Duration, len(timer.lookup))
	timer.StatsInto(stats)
	return stats
}

func (timer *nodeTimer) StatsInto(dst map[Key]time.Duration) {
	for name, component := range timer.lookup {
		dst[name] = time.Duration(component.Load())
	}
}

func (timer *nodeTimer) ComponentKeys() []Key {
	keys := make([]Key, 0, len(timer.lookup))
	for k := range timer.lookup {
		keys = append(keys, k)
	}
	return keys
}

func (timer *nodeTimer) SumSpent() time.Duration {
	var sum time.Duration
	for _, component := range timer.lookup {
		sum += time.Duration(component.Load())
	}
	return sum
}

func (timer *nodeTimer) SumRemaining() time.Duration {
	budget := timer.sumBudget()
	if budget == UnlimitedBudget {
		return UnlimitedBudget
	}

	remaining := budget - timer.SumSpent()
	if remaining < 0 {
		return 0
	}

	return remaining
}

func (timer *nodeTimer) SumExhausted() bool {
	budget := timer.sumBudget()
	if budget == UnlimitedBudget {
		return false
	}

	return timer.SumSpent() > budget
}

// sumBudget returns the budget used by SumRemaining/SumExhausted.
// When Start() has resolved an inherited budget, that resolved value is used.
// Otherwise the budget is computed on the fly from the parent so callers can
// query a dynamically-budgeted node timer before it has been started.
func (timer *nodeTimer) sumBudget() time.Duration {
	budget := timer.budgetValue()
	if budget == DynamicBudget && timer.parent != nil {
		budget = timer.config.dynamicBudget(timer.parent)
	}
	if budget == DynamicBudget {
		return UnlimitedBudget
	}
	return budget
}
