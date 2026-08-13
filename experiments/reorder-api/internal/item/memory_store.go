package item

import (
	"sync"
)

type MemoryStore struct {
	mu    sync.Mutex
	items []Item
}

func NewMemoryStore(items []Item) *MemoryStore {
	copied := append([]Item(nil), items...)
	return &MemoryStore{items: copied}
}

func (s *MemoryStore) List() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]Item(nil), s.items...)
}

// Reorder moves id immediately after previousID.
// A nil previousID means moving id to the beginning.
func (s *MemoryStore) Reorder(id string, previousID *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := reorder(s.items, id, previousID)
	if err != nil {
		return err
	}
	s.items = items
	return nil
}
