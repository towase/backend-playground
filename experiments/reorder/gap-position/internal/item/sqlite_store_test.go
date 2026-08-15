package item

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSQLiteStoreSeedAndList(t *testing.T) {
	store := newSQLiteTestStore(t)

	got, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []Item{
		{ID: "A", Name: "alpha", Position: 100},
		{ID: "B", Name: "bravo", Position: 200},
		{ID: "C", Name: "charlie", Position: 300},
		{ID: "D", Name: "delta", Position: 400},
		{ID: "E", Name: "echo", Position: 500},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

func TestSQLiteStoreReorder(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		previousID *string
		want       []Item
	}{
		{
			name:       "後ろの要素を前方へ移動する",
			id:         "D",
			previousID: stringPointer("A"),
			want: []Item{
				{ID: "A", Name: "alpha", Position: 100},
				{ID: "D", Name: "delta", Position: 150},
				{ID: "B", Name: "bravo", Position: 200},
				{ID: "C", Name: "charlie", Position: 300},
				{ID: "E", Name: "echo", Position: 500},
			},
		},
		{
			name:       "前の要素を後方へ移動する",
			id:         "B",
			previousID: stringPointer("D"),
			want: []Item{
				{ID: "A", Name: "alpha", Position: 100},
				{ID: "C", Name: "charlie", Position: 300},
				{ID: "D", Name: "delta", Position: 400},
				{ID: "B", Name: "bravo", Position: 450},
				{ID: "E", Name: "echo", Position: 500},
			},
		},
		{
			name:       "先頭へ移動する",
			id:         "C",
			previousID: nil,
			want: []Item{
				{ID: "C", Name: "charlie", Position: 50},
				{ID: "A", Name: "alpha", Position: 100},
				{ID: "B", Name: "bravo", Position: 200},
				{ID: "D", Name: "delta", Position: 400},
				{ID: "E", Name: "echo", Position: 500},
			},
		},
		{
			name:       "末尾へ移動する",
			id:         "A",
			previousID: stringPointer("E"),
			want: []Item{
				{ID: "B", Name: "bravo", Position: 200},
				{ID: "C", Name: "charlie", Position: 300},
				{ID: "D", Name: "delta", Position: 400},
				{ID: "E", Name: "echo", Position: 500},
				{ID: "A", Name: "alpha", Position: 600},
			},
		},
		{
			name:       "現在と同じ位置なら更新しない",
			id:         "B",
			previousID: stringPointer("A"),
			want:       testItems(),
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
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLiteStoreReorderValidation(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		previousID *string
		wantErr    error
	}{
		{name: "移動対象が存在しない", id: "unknown", wantErr: ErrItemNotFound},
		{name: "previousIdが存在しない", id: "A", previousID: stringPointer("unknown"), wantErr: ErrItemNotFound},
		{name: "idとpreviousIdが同じ", id: "A", previousID: stringPointer("A"), wantErr: ErrPreviousSame},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newSQLiteTestStore(t)
			before, err := store.List(context.Background())
			if err != nil {
				t.Fatalf("List() before reorder error = %v", err)
			}
			err = store.Reorder(context.Background(), tt.id, tt.previousID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Reorder() error = %v, want %v", err, tt.wantErr)
			}
			after, err := store.List(context.Background())
			if err != nil {
				t.Fatalf("List() after reorder error = %v", err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Errorf("エラー時にデータが変更された: got %v, want %v", after, before)
			}
		})
	}
}

func TestSQLiteStoreReorderUpdatesOnlyMovedItemWhenGapExists(t *testing.T) {
	store := newSQLiteTestStore(t)
	ctx := context.Background()
	_, err := store.db.ExecContext(ctx, `
        CREATE TABLE position_updates (item_id TEXT NOT NULL);
        CREATE TRIGGER count_position_updates
        AFTER UPDATE OF position ON items
        BEGIN
            INSERT INTO position_updates (item_id) VALUES (NEW.id);
        END;`)
	if err != nil {
		t.Fatalf("create update counter: %v", err)
	}

	if err := store.Reorder(ctx, "D", stringPointer("A")); err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}

	var count int
	if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM position_updates").Scan(&count); err != nil {
		t.Fatalf("count position updates: %v", err)
	}
	if count != 1 {
		t.Errorf("updated rows = %d, want 1", count)
	}

	if err := store.Reorder(ctx, "D", stringPointer("A")); err != nil {
		t.Fatalf("same Reorder() error = %v", err)
	}
	if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM position_updates").Scan(&count); err != nil {
		t.Fatalf("count position updates after no-op: %v", err)
	}
	if count != 1 {
		t.Errorf("updated rows after no-op = %d, want 1", count)
	}
}

func TestSQLiteStoreReorderRebalancesWhenGapIsExhausted(t *testing.T) {
	store := newSQLiteTestStoreWithItems(t, []Item{
		{ID: "A", Name: "alpha", Position: 100},
		{ID: "B", Name: "bravo", Position: 101},
		{ID: "C", Name: "charlie", Position: 200},
	})
	ctx := context.Background()

	if err := store.Reorder(ctx, "C", stringPointer("A")); err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []Item{
		{ID: "A", Name: "alpha", Position: 100},
		{ID: "C", Name: "charlie", Position: 200},
		{ID: "B", Name: "bravo", Position: 300},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

func TestSQLiteStoreReorderRebalancesAtPositionBoundary(t *testing.T) {
	tests := []struct {
		name       string
		items      []Item
		id         string
		previousID *string
		want       []Item
	}{
		{
			name: "先頭に整数の隙間がない",
			items: []Item{
				{ID: "A", Name: "alpha", Position: 1},
				{ID: "B", Name: "bravo", Position: 100},
				{ID: "C", Name: "charlie", Position: 200},
			},
			id:         "C",
			previousID: nil,
			want: []Item{
				{ID: "C", Name: "charlie", Position: 100},
				{ID: "A", Name: "alpha", Position: 200},
				{ID: "B", Name: "bravo", Position: 300},
			},
		},
		{
			name: "末尾でint64を加算できない",
			items: []Item{
				{ID: "A", Name: "alpha", Position: math.MaxInt64 - 100},
				{ID: "B", Name: "bravo", Position: math.MaxInt64},
			},
			id:         "A",
			previousID: stringPointer("B"),
			want: []Item{
				{ID: "B", Name: "bravo", Position: 100},
				{ID: "A", Name: "alpha", Position: 200},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newSQLiteTestStoreWithItems(t, tt.items)
			ctx := context.Background()
			if err := store.Reorder(ctx, tt.id, tt.previousID); err != nil {
				t.Fatalf("Reorder() error = %v", err)
			}
			got, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSQLiteStoreReorderRollsBackWhenRebalanceFails(t *testing.T) {
	store := newSQLiteTestStoreWithItems(t, []Item{
		{ID: "A", Name: "alpha", Position: 100},
		{ID: "B", Name: "bravo", Position: 101},
		{ID: "C", Name: "charlie", Position: 200},
	})
	ctx := context.Background()
	before, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() before reorder error = %v", err)
	}

	_, err = store.db.ExecContext(ctx, `
        CREATE TRIGGER fail_b_position_update
        BEFORE UPDATE OF position ON items
        WHEN NEW.id = 'B' AND NEW.position > 0
        BEGIN
            SELECT RAISE(ABORT, 'forced update failure');
        END;`)
	if err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if err := store.Reorder(ctx, "C", stringPointer("A")); err == nil {
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

func newSQLiteTestStore(t *testing.T) *SQLiteStore {
	return newSQLiteTestStoreWithItems(t, testItems())
}

func newSQLiteTestStoreWithItems(t *testing.T, items []Item) *SQLiteStore {
	t.Helper()
	ctx := context.Background()
	store, err := OpenSQLiteStore(ctx, filepath.Join(t.TempDir(), "items.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.SeedIfEmpty(ctx, items); err != nil {
		t.Fatalf("SeedIfEmpty() error = %v", err)
	}
	return store
}

func testItems() []Item {
	return []Item{
		{ID: "A", Name: "alpha", Position: 100},
		{ID: "B", Name: "bravo", Position: 200},
		{ID: "C", Name: "charlie", Position: 300},
		{ID: "D", Name: "delta", Position: 400},
		{ID: "E", Name: "echo", Position: 500},
	}
}

func stringPointer(value string) *string {
	return &value
}
