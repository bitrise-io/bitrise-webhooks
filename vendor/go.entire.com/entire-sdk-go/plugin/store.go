package plugin

import (
	"context"
	"sync"
	"time"
)

// ReplayStore remembers single-use identifiers, such as the jti of install
// receipts and delivery JWTs, so that each is accepted once.
//
// The default MemoryStore protects only a single process. A plugin that runs
// several replicas should supply a shared store, such as one backed by Redis
// or its database.
type ReplayStore interface {
	// Remember records key until the given time. It reports fresh=false when
	// key was already recorded and has not expired.
	Remember(ctx context.Context, key string, until time.Time) (fresh bool, err error)
}

// DeliveryStore records the webhook deliveries a plugin has handled, so that
// a delivery Entire sends again is handled once. Entire keeps the same
// delivery ID across retries and resends.
type DeliveryStore interface {
	// Handled reports whether the delivery was already handled.
	Handled(ctx context.Context, deliveryID string) (bool, error)
	// MarkHandled records that the delivery was handled, until the given
	// time.
	MarkHandled(ctx context.Context, deliveryID string, until time.Time) error
}

// MemoryStore is an in-memory ReplayStore and DeliveryStore. It is safe for
// concurrent use and forgets everything when the process exits.
type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]time.Time
	now     func() time.Time
	added   int
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: map[string]time.Time{}, now: time.Now}
}

// Remember implements ReplayStore.
func (m *MemoryStore) Remember(_ context.Context, key string, until time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if expires, ok := m.entries["r:"+key]; ok && now.Before(expires) {
		return false, nil
	}
	m.put("r:"+key, until, now)
	return true, nil
}

// Handled implements DeliveryStore.
func (m *MemoryStore) Handled(_ context.Context, deliveryID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	expires, ok := m.entries["d:"+deliveryID]
	return ok && m.now().Before(expires), nil
}

// MarkHandled implements DeliveryStore.
func (m *MemoryStore) MarkHandled(_ context.Context, deliveryID string, until time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.put("d:"+deliveryID, until, m.now())
	return nil
}

// put stores an entry, and sweeps expired entries every 1,024 additions.
func (m *MemoryStore) put(key string, until, now time.Time) {
	m.entries[key] = until
	if m.added++; m.added%1024 == 0 {
		for k, expires := range m.entries {
			if !now.Before(expires) {
				delete(m.entries, k)
			}
		}
	}
}
