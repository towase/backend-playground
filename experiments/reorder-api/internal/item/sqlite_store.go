package item

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL UNIQUE
);`

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLiteStore(ctx context.Context, dataSourceName string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Keep this experiment on one connection. This also makes connection-local
	// PRAGMA settings deterministic; concurrent-writer behavior can be explored
	// separately by increasing the pool size.
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

// Reorder reads and rewrites the ordering in one transaction. If any update
// fails, the deferred rollback restores every position to its original value.
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
	items, err := reorder(current, id, previousID)
	if err != nil {
		return err
	}

	// position has a UNIQUE constraint. Move every current value into the
	// negative range first so the positive positions can be assigned safely.
	if _, err := tx.ExecContext(ctx, "UPDATE items SET position = -position"); err != nil {
		return fmt.Errorf("reserve positions: %w", err)
	}

	statement, err := tx.PrepareContext(ctx, "UPDATE items SET position = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("prepare position update: %w", err)
	}
	defer statement.Close()

	for _, item := range items {
		result, err := statement.ExecContext(ctx, item.Position, item.ID)
		if err != nil {
			return fmt.Errorf("update item %s: %w", item.ID, err)
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read affected rows for item %s: %w", item.ID, err)
		}
		if updated != 1 {
			return fmt.Errorf("update item %s: expected 1 row, got %d", item.ID, updated)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder: %w", err)
	}
	return nil
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
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Position); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
