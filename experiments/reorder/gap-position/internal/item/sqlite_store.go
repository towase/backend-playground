package item

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL UNIQUE
);`

const positionGap int64 = 100

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLiteStore(ctx context.Context, dataSourceName string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &SQLiteStore{db: db}
	if err := store.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) initialize(ctx context.Context) error {
	statements := []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		schema,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) SeedIfEmpty(ctx context.Context, items []Item) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		return fmt.Errorf("count items: %w", err)
	}
	if count > 0 {
		return tx.Commit()
	}

	statement, err := tx.PrepareContext(ctx, `
        INSERT INTO items (id, name, position)
        VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare seed: %w", err)
	}
	defer statement.Close()

	for _, item := range items {
		if _, err := statement.ExecContext(ctx, item.ID, item.Name, item.Position); err != nil {
			return fmt.Errorf("insert item %s: %w", item.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed: %w", err)
	}
	return nil
}

func (s *SQLiteStore) List(ctx context.Context) ([]Item, error) {
	return listItems(ctx, s.db)
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func listItems(ctx context.Context, queryer queryer) ([]Item, error) {
	rows, err := queryer.QueryContext(ctx, `
        SELECT id, name, position
        FROM items
        ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Position); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}
	return items, nil
}

func (s *SQLiteStore) Reorder(ctx context.Context, id string, previousID *string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin reorder transaction: %w", err)
	}
	defer tx.Rollback()

	current, err := listItems(ctx, tx)
	if err != nil {
		return fmt.Errorf("list items for reorder: %w", err)
	}
	if alreadyInPosition(current, id, previousID) {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit unchanged reorder: %w", err)
		}
		return nil
	}

	items, err := reorderItems(current, id, previousID)
	if err != nil {
		return err
	}
	movingIndex := indexByID(items, id)
	newPosition, err := estimatePosition(items, movingIndex)
	switch {
	case err == nil:
		if err := updatePosition(ctx, tx, id, newPosition); err != nil {
			return err
		}
	case errors.Is(err, ErrNoGap):
		if err := rebalance(ctx, tx, items); err != nil {
			return err
		}
	default:
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder: %w", err)
	}
	return nil
}

func alreadyInPosition(items []Item, id string, previousID *string) bool {
	index := indexByID(items, id)
	if index == -1 {
		return false
	}
	if previousID == nil {
		return index == 0
	}
	return index > 0 && items[index-1].ID == *previousID
}

func reorderItems(current []Item, id string, previousID *string) ([]Item, error) {
	if previousID != nil && id == *previousID {
		return nil, ErrPreviousSame
	}

	items := append([]Item(nil), current...)
	movingIndex := indexByID(items, id)
	if movingIndex == -1 {
		return nil, ErrItemNotFound
	}
	if previousID != nil && indexByID(items, *previousID) == -1 {
		return nil, ErrItemNotFound
	}

	movingItem := items[movingIndex]
	items = append(items[:movingIndex], items[movingIndex+1:]...)

	insertIndex := 0
	if previousID != nil {
		insertIndex = indexByID(items, *previousID) + 1
	}
	items = append(items, Item{})
	copy(items[insertIndex+1:], items[insertIndex:])
	items[insertIndex] = movingItem
	return items, nil
}

func estimatePosition(items []Item, movingIndex int) (int64, error) {
	if movingIndex < 0 || movingIndex >= len(items) {
		return 0, ErrItemNotFound
	}

	previousPosition := int64(0)
	if movingIndex > 0 {
		previousPosition = items[movingIndex-1].Position
	}
	if movingIndex == len(items)-1 {
		if previousPosition > math.MaxInt64-positionGap {
			return 0, ErrNoGap
		}
		return previousPosition + positionGap, nil
	}

	nextPosition := items[movingIndex+1].Position
	if nextPosition-previousPosition < 2 {
		return 0, ErrNoGap
	}
	return previousPosition + (nextPosition-previousPosition)/2, nil
}

func indexByID(items []Item, id string) int {
	for i := range items {
		if items[i].ID == id {
			return i
		}
	}
	return -1
}

func updatePosition(ctx context.Context, tx *sql.Tx, id string, position int64) error {
	result, err := tx.ExecContext(ctx, "UPDATE items SET position = ? WHERE id = ?", position, id)
	if err != nil {
		return fmt.Errorf("update item %s: %w", id, err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows for item %s: %w", id, err)
	}
	if updated != 1 {
		return fmt.Errorf("update item %s: expected 1 row, got %d", id, updated)
	}
	return nil
}

func rebalance(ctx context.Context, tx *sql.Tx, items []Item) error {
	if _, err := tx.ExecContext(ctx, "UPDATE items SET position = -position"); err != nil {
		return fmt.Errorf("reserve positions: %w", err)
	}

	statement, err := tx.PrepareContext(ctx, "UPDATE items SET position = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("prepare rebalance: %w", err)
	}
	defer statement.Close()

	for i, item := range items {
		if int64(i+1) > math.MaxInt64/positionGap {
			return fmt.Errorf("rebalance positions: too many items")
		}
		position := int64(i+1) * positionGap
		result, err := statement.ExecContext(ctx, position, item.ID)
		if err != nil {
			return fmt.Errorf("rebalance item %s: %w", item.ID, err)
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read affected rows for item %s: %w", item.ID, err)
		}
		if updated != 1 {
			return fmt.Errorf("rebalance item %s: expected 1 row, got %d", item.ID, updated)
		}
	}
	return nil
}
