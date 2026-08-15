package item

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"
)

func TestConcurrentReordersAreSerializedBySingleConnection(t *testing.T) {
	store := newSQLiteTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := make(chan struct{})
	errors := make(chan error, 2)
	for _, id := range []string{"D", "E"} {
		go func() {
			<-start
			errors <- store.Reorder(ctx, id, stringPointer("A"))
		}()
	}
	close(start)

	for range 2 {
		if err := <-errors; err != nil {
			t.Fatalf("Reorder() error = %v", err)
		}
	}

	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got[0].ID != "A" {
		t.Fatalf("first item = %s, want A", got[0].ID)
	}
	if !reflect.DeepEqual([]int64{got[0].Position, got[1].Position, got[2].Position}, []int64{100, 125, 150}) {
		t.Errorf("first three positions = %v, want [100 125 150]", []int64{got[0].Position, got[1].Position, got[2].Position})
	}
	moved := map[string]bool{got[1].ID: true, got[2].ID: true}
	if !moved["D"] || !moved["E"] {
		t.Errorf("items after A = [%s %s], want D and E", got[1].ID, got[2].ID)
	}
}

func TestTwoTransactionsCompeteForSamePosition(t *testing.T) {
	store := newSQLiteTestStore(t)
	store.db.SetMaxOpenConns(2)
	store.db.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	firstConnection, err := store.db.Conn(ctx)
	if err != nil {
		t.Fatalf("get first connection: %v", err)
	}
	defer firstConnection.Close()
	secondConnection, err := store.db.Conn(ctx)
	if err != nil {
		t.Fatalf("get second connection: %v", err)
	}
	defer secondConnection.Close()

	for _, connection := range []*sql.Conn{firstConnection, secondConnection} {
		if _, err := connection.ExecContext(ctx, "PRAGMA busy_timeout = 1000"); err != nil {
			t.Fatalf("set busy timeout: %v", err)
		}
	}

	firstTransaction, err := firstConnection.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		t.Fatalf("begin first transaction: %v", err)
	}
	defer firstTransaction.Rollback()
	secondTransaction, err := secondConnection.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		t.Fatalf("begin second transaction: %v", err)
	}
	defer secondTransaction.Rollback()

	firstPosition := positionAfterA(t, ctx, firstTransaction, "D")
	secondPosition := positionAfterA(t, ctx, secondTransaction, "E")
	if firstPosition != secondPosition {
		t.Fatalf("calculated positions = [%d %d], want the same position", firstPosition, secondPosition)
	}
	t.Logf("both transactions calculated position %d", firstPosition)

	if err := updatePosition(ctx, firstTransaction, "D", firstPosition); err != nil {
		t.Fatalf("first update: %v", err)
	}
	if err := firstTransaction.Commit(); err != nil {
		t.Fatalf("commit first transaction: %v", err)
	}

	err = updatePosition(ctx, secondTransaction, "E", secondPosition)
	if err == nil {
		t.Fatal("second update succeeded, want a concurrency conflict")
	}
	t.Logf("second update was rejected: %v", err)
	if err := secondTransaction.Rollback(); err != nil {
		t.Fatalf("rollback second transaction: %v", err)
	}

	got, err := listItems(ctx, firstConnection)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []Item{
		{ID: "A", Name: "alpha", Position: 100},
		{ID: "D", Name: "delta", Position: 150},
		{ID: "B", Name: "bravo", Position: 200},
		{ID: "C", Name: "charlie", Position: 300},
		{ID: "E", Name: "echo", Position: 500},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

func positionAfterA(t *testing.T, ctx context.Context, tx *sql.Tx, id string) int64 {
	t.Helper()

	current, err := listItems(ctx, tx)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	items, err := reorderItems(current, id, stringPointer("A"))
	if err != nil {
		t.Fatalf("reorder items: %v", err)
	}
	position, err := estimatePosition(items, indexByID(items, id))
	if err != nil {
		t.Fatalf("estimate position: %v", err)
	}
	return position
}
