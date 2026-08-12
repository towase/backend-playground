package item

import (
	"errors"
	"sync"
)

var (
	ErrItemNotFound = errors.New("item not found")
	ErrPreviousSame = errors.New("id and previousId must be different")
)

type Item struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type Store struct {
	mu    sync.Mutex
	items []Item
}

func NewStore(items []Item) *Store {
	copied := append([]Item(nil), items...)
	return &Store{items: copied}
}

func (s *Store) List() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]Item(nil), s.items...)
}

// Reorder moves id immediately after previousID.
// A nil previousID means moving id to the beginning.
func (s *Store) Reorder(id string, previousID *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.find(id)
	if item == nil {
		return ErrItemNotFound
	}
	var previousItem *Item
	if previousID != nil {
		if id == *previousID {
			return ErrPreviousSame
		}
		previousItem = s.find(*previousID)
		if previousItem == nil {
			return ErrItemNotFound
		}
	}
	movingItem := *item
	s.remove(id)
	if previousID == nil {
		s.items = append([]Item{movingItem}, s.items...)
	} else {
		for i := range len(s.items) {
			if s.items[i].ID == *previousID {
				insertIndex := i + 1
				items := append([]Item(nil), s.items[:insertIndex]...)
				items = append(items, movingItem)
				items = append(items, s.items[insertIndex:]...)
				s.items = items
				break
			}
		}
	}
	for i := range len(s.items) {
		s.items[i].Position = i + 1
	}
	return nil
}

func (s *Store) find(id string) *Item {
	for i := range len(s.items) {
		if s.items[i].ID == id {
			return &s.items[i]
		}
	}
	return nil
}

func (s *Store) remove(id string) error {
	for i := range len(s.items) {
		if s.items[i].ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return nil
		}
	}
	return nil
}
