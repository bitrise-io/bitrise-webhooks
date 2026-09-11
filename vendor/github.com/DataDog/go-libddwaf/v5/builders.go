// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package libddwaf

import (
	"math"

	"github.com/DataDog/go-libddwaf/v5/internal/invariant"
)

// ArrayBuilder accumulates WAFObject entries and commits them to a parent
// WAFObject on Close. It is not safe for concurrent use.
type ArrayBuilder struct {
	enc     *Encoder
	parent  *WAFObject
	entries []WAFObject
	skipped int
	closed  bool
}

// Array returns a new ArrayBuilder that will commit its entries into parent.
// capacityHint pre-allocates the backing slice (capped at MaxContainerSize);
// pass 0 when the eventual size is unknown.
func (e *Encoder) Array(parent *WAFObject, capacityHint int) *ArrayBuilder {
	initial := max(capacityHint, 0)
	if limit := e.Config.maxContainerSize(); initial > limit {
		initial = limit
	}
	return &ArrayBuilder{enc: e, parent: parent, entries: make([]WAFObject, 0, initial)}
}

// NextValue appends a new slot to the array and returns a pointer to the
// WAFObject for the caller to populate. Returns nil when the builder is at
// MaxContainerSize capacity. The caller must populate the slot or call DropLast
// before calling NextValue again.
func (b *ArrayBuilder) NextValue() *WAFObject {
	// Entries appended after Close would be silently lost: Close is
	// idempotent and never re-commits to the parent.
	invariant.Assert(!b.closed, "NextValue called on closed ArrayBuilder")
	if len(b.entries) >= b.enc.Config.maxContainerSize() {
		return nil
	}
	b.entries = append(b.entries, WAFObject{})
	return &b.entries[len(b.entries)-1]
}

// Skip records that an array element was intentionally omitted. The count is
// used by Close to record a ContainerTooLarge truncation if any were skipped.
func (b *ArrayBuilder) Skip() { b.skipped++ }

// DropLast removes the most recently appended entry. No-op if the builder has
// no entries. Use this to undo a NextValue call when encoding the value fails.
func (b *ArrayBuilder) DropLast() {
	if len(b.entries) == 0 {
		return
	}
	b.entries = b.entries[:len(b.entries)-1]
}

// Close commits the accumulated entries to the parent WAFObject. If any entries
// were skipped, a ContainerTooLarge truncation is recorded. Calling Close more
// than once is safe and has no additional effect.
func (b *ArrayBuilder) Close() {
	if b.closed {
		return
	}
	b.closed = true
	overflow := b.skipped
	if len(b.entries) > math.MaxUint16 {
		overflow += len(b.entries) - math.MaxUint16
		b.entries = b.entries[:math.MaxUint16]
	}
	if overflow > 0 {
		b.enc.Truncations.Record(ContainerTooLarge, len(b.entries)+overflow)
	}
	// Force cap == len: SetArrayData rejects cap > MaxUint16, and append growth
	// can leave the backing array's capacity above the element count.
	entries := b.entries[:len(b.entries):len(b.entries)]
	err := b.parent.SetArrayData(b.enc.Config.Pinner, entries)
	invariant.Assert(err == nil, "SetArrayData must not fail after capping to MaxUint16: %v", err)
}

// MapBuilder accumulates WAFObjectKV entries and commits them to a parent
// WAFObject on Close. It is not safe for concurrent use.
type MapBuilder struct {
	enc     *Encoder
	parent  *WAFObject
	entries []WAFObjectKV
	skipped int
	closed  bool
}

// Map returns a new MapBuilder that will commit its entries into parent.
// capacityHint pre-allocates the backing slice (capped at MaxContainerSize);
// pass 0 when the eventual size is unknown.
func (e *Encoder) Map(parent *WAFObject, capacityHint int) *MapBuilder {
	initial := max(capacityHint, 0)
	if limit := e.Config.maxContainerSize(); initial > limit {
		initial = limit
	}
	return &MapBuilder{enc: e, parent: parent, entries: make([]WAFObjectKV, 0, initial)}
}

// NextValue appends a new key-value slot to the map and returns a pointer to
// the value WAFObject for the caller to populate. Returns nil when the builder
// is at MaxContainerSize capacity. The caller must populate the value or call
// DropLast before calling NextValue again.
func (b *MapBuilder) NextValue(key string) *WAFObject {
	// Entries appended after Close would be silently lost: Close is
	// idempotent and never re-commits to the parent.
	invariant.Assert(!b.closed, "NextValue called on closed MapBuilder")
	if len(b.entries) >= b.enc.Config.maxContainerSize() {
		return nil
	}
	b.entries = append(b.entries, WAFObjectKV{})
	kv := &b.entries[len(b.entries)-1]
	b.enc.WriteString(&kv.Key, key)
	return &kv.Val
}

// Skip records that a key-value pair was intentionally omitted (e.g. due to
// an encoding error). The count is used by Close to record a ContainerTooLarge
// truncation if any entries were skipped.
func (b *MapBuilder) Skip() { b.skipped++ }

// DropLast removes the most recently appended entry. No-op if the builder has
// no entries. Use this to undo a NextValue call when encoding the value fails.
func (b *MapBuilder) DropLast() {
	if len(b.entries) == 0 {
		return
	}
	b.entries = b.entries[:len(b.entries)-1]
}

// Close commits the accumulated entries to the parent WAFObject. If any entries
// were skipped, a ContainerTooLarge truncation is recorded. Calling Close more
// than once is safe and has no additional effect.
func (b *MapBuilder) Close() {
	if b.closed {
		return
	}
	b.closed = true
	overflow := b.skipped
	if len(b.entries) > math.MaxUint16 {
		overflow += len(b.entries) - math.MaxUint16
		b.entries = b.entries[:math.MaxUint16]
	}
	if overflow > 0 {
		b.enc.Truncations.Record(ContainerTooLarge, len(b.entries)+overflow)
	}
	// Force cap == len: SetMapData rejects cap > MaxUint16, and append growth
	// can leave the backing array's capacity above the element count.
	entries := b.entries[:len(b.entries):len(b.entries)]
	err := b.parent.SetMapData(b.enc.Config.Pinner, entries)
	invariant.Assert(err == nil, "SetMapData must not fail after capping to MaxUint16: %v", err)
}
