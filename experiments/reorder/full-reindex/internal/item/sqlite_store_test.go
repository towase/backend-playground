package item

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSQLiteStoreReorder(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newSQLiteTestStore(t)
			ctx := context.Background()

			if err := store.Reorder(ctx, tt.id, tt.previousID); err != nil {
				t.Fatalf("Reorder() error = %v", err)
			}
			got, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if gotIDs := itemIDs(got); !reflect.DeepEqual(gotIDs, tt.wantIDs) {
				t.Errorf("IDs = %v, want %v", gotIDs, tt.wantIDs)
			}
			assertSequentialPositions(t, got)
		})
	}
}

func TestSQLiteStoreReorderValidationDoesNotChangeData(t *testing.T) {
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
			store := newSQLiteTestStore(t)
			ctx := context.Background()
			before, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List() before reorder error = %v", err)
			}

			err = store.Reorder(ctx, tt.id, tt.previousID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Reorder() error = %v, want %v", err, tt.wantErr)
			}
			after, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List() after reorder error = %v", err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Errorf("エラー時にデータが変更された: got %v, want %v", after, before)
			}
		})
	}
}

func TestSQLiteStoreReorderRollsBackWhenUpdateFails(t *testing.T) {
	store := newSQLiteTestStore(t)
	ctx := context.Background()
	before, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() before reorder error = %v", err)
	}

	_, err = store.db.ExecContext(ctx, `
        CREATE TRIGGER fail_c_position_update
        BEFORE UPDATE OF position ON items
        WHEN NEW.id = 'C' AND NEW.position > 0
        BEGIN
            SELECT RAISE(ABORT, 'forced update failure');
        END;`)
	if err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	err = store.Reorder(ctx, "D", stringPointer("A"))
	if err == nil {
		t.Fatal("Reorder() error = nil, want forced update failure")
	}

	after, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() after reorder error = %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("ロールバック後のデータ = %v, want %v", after, before)
	}
}

func TestSQLiteStorePersistsReorderedItems(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "items.db")
	store, err := OpenSQLiteStore(ctx, databasePath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	if err := store.SeedIfEmpty(ctx, testItems()); err != nil {
		t.Fatalf("SeedIfEmpty() error = %v", err)
	}
	if err := store.Reorder(ctx, "D", stringPointer("A")); err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := OpenSQLiteStore(ctx, databasePath)
	if err != nil {
		t.Fatalf("reopen SQLiteStore error = %v", err)
	}
	t.Cleanup(func() { reopened.Close() })

	got, err := reopened.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	wantIDs := []string{"A", "D", "B", "C", "E"}
	if gotIDs := itemIDs(got); !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("reopened IDs = %v, want %v", gotIDs, wantIDs)
	}
}

func newSQLiteTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	ctx := context.Background()
	store, err := OpenSQLiteStore(ctx, filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.SeedIfEmpty(ctx, testItems()); err != nil {
		t.Fatalf("SeedIfEmpty() error = %v", err)
	}
	return store
}

func testItems() []Item {
	return []Item{
		{ID: "A", Name: "alpha", Position: 1},
		{ID: "B", Name: "bravo", Position: 2},
		{ID: "C", Name: "charlie", Position: 3},
		{ID: "D", Name: "delta", Position: 4},
		{ID: "E", Name: "echo", Position: 5},
	}
}

func assertSequentialPositions(t *testing.T, items []Item) {
	t.Helper()
	for i, item := range items {
		if want := i + 1; item.Position != want {
			t.Errorf("item %s position = %d, want %d", item.ID, item.Position, want)
		}
	}
}
