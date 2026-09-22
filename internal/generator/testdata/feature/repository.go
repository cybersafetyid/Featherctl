package userprofile

import (
	"context"
	"strconv"
	"sync"
)

// Repository stores and retrieves UserProfile aggregates.
//
// The service depends on this interface, never on a concrete store, so
// swapping in a database later touches only the container.
type Repository interface {
	// Create stores item and returns it with the ID assigned by the store.
	Create(ctx context.Context, item UserProfile) (UserProfile, error)
	// List returns every stored UserProfile, oldest first.
	List(ctx context.Context) ([]UserProfile, error)
}

// InMemoryRepository is a goroutine-safe Repository backed by a map.
//
// It exists so a freshly generated feature compiles, runs and can be tested
// before a real data store exists. Replace it — and the wiring in
// internal/platform/container — when you have one.
type InMemoryRepository struct {
	mu    sync.RWMutex
	next  int
	items map[string]UserProfile
	ids   []string
}

// NewInMemoryRepository returns an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{items: make(map[string]UserProfile)}
}

// Create implements [Repository].
func (r *InMemoryRepository) Create(ctx context.Context, item UserProfile) (UserProfile, error) {
	if err := ctx.Err(); err != nil {
		return UserProfile{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.next++
	item.ID = strconv.Itoa(r.next)
	r.items[item.ID] = item
	r.ids = append(r.ids, item.ID)

	return item, nil
}

// List implements [Repository].
func (r *InMemoryRepository) List(ctx context.Context) ([]UserProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]UserProfile, 0, len(r.ids))
	for _, id := range r.ids {
		out = append(out, r.items[id])
	}
	return out, nil
}
