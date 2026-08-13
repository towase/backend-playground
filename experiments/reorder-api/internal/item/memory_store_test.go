package item

import (
	"errors"
	"reflect"
	"testing"
)

func TestMemoryStoreReorder(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		previousID *string
		wantIDs    []string
	}{
		{
			name:       "後ろの要素を前方へ移動する",
			id:         "D",
			previousID: stringPointer("A"),
			wantIDs:    []string{"A", "D", "B", "C", "E"},
		},
		{
			name:       "前の要素を後方へ移動する",
			id:         "B",
			previousID: stringPointer("D"),
			wantIDs:    []string{"A", "C", "D", "B", "E"},
		},
		{
			name:       "先頭へ移動する",
			id:         "C",
			previousID: nil,
			wantIDs:    []string{"C", "A", "B", "D", "E"},
		},
		{
			name:       "現在と同じ位置への移動は順序を変えない",
			id:         "B",
			previousID: stringPointer("A"),
			wantIDs:    []string{"A", "B", "C", "D", "E"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore()

			if err := store.Reorder(tt.id, tt.previousID); err != nil {
				t.Fatalf("Reorder() error = %v", err)
			}

			got := store.List()
			if gotIDs := itemIDs(got); !reflect.DeepEqual(gotIDs, tt.wantIDs) {
				t.Errorf("IDs = %v, want %v", gotIDs, tt.wantIDs)
			}

			for i, item := range got {
				if want := i + 1; item.Position != want {
					t.Errorf("item %s position = %d, want %d", item.ID, item.Position, want)
				}
			}
		})
	}
}

func TestMemoryStoreReorderValidation(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		previousID *string
		wantErr    error
	}{
		{
			name:    "移動対象が存在しない",
			id:      "unknown",
			wantErr: ErrItemNotFound,
		},
		{
			name:       "previousIdが存在しない",
			id:         "A",
			previousID: stringPointer("unknown"),
			wantErr:    ErrItemNotFound,
		},
		{
			name:       "idとpreviousIdが同じ",
			id:         "A",
			previousID: stringPointer("A"),
			wantErr:    ErrPreviousSame,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore()
			before := store.List()

			err := store.Reorder(tt.id, tt.previousID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Reorder() error = %v, want %v", err, tt.wantErr)
			}
			if got := store.List(); !reflect.DeepEqual(got, before) {
				t.Errorf("エラー時にデータが変更された: got %v, want %v", got, before)
			}
		})
	}
}

func newTestStore() *MemoryStore {
	return NewMemoryStore([]Item{
		{ID: "A", Name: "alpha", Position: 1},
		{ID: "B", Name: "bravo", Position: 2},
		{ID: "C", Name: "charlie", Position: 3},
		{ID: "D", Name: "delta", Position: 4},
		{ID: "E", Name: "echo", Position: 5},
	})
}

func stringPointer(value string) *string {
	return &value
}

func itemIDs(items []Item) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}
