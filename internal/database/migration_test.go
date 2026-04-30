package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestRunMigrations_FreshDB(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { _ = os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var count int
	err = db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}

	migs, migErr := getMigrations()
	if migErr != nil {
		t.Fatalf("getMigrations: %v", migErr)
	}

	if count != len(migs) {
		t.Errorf("expected %d migrations, got %d", len(migs), count)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { _ = os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	var count int
	err = db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}

	migs, migErr := getMigrations()
	if migErr != nil {
		t.Fatalf("getMigrations: %v", migErr)
	}

	if count != len(migs) {
		t.Errorf("expected %d migrations after idempotent run, got %d", len(migs), count)
	}
}

func TestRunMigrations_CreatesEventsTable(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { _ = os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var name string
	err = db.QueryRowContext(context.Background(),
		"SELECT name FROM sqlite_master WHERE type='table' AND name='events'",
	).Scan(&name)
	if err != nil {
		t.Fatal("events table not found after migration")
	}
}

func TestRunMigrations_CreatesIndexes(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { _ = os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	expectedIndexes := []string{
		"idx_events_created_at",
		"idx_events_type",
		"idx_events_source_id",
		"idx_events_actor_login",
		"idx_events_repo_name",
		"idx_events_source",
		"idx_events_source_source_id",
	}

	for _, idx := range expectedIndexes {
		var name string
		err = db.QueryRowContext(context.Background(),
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %s not found: %v", idx, err)
		}
	}
}

func TestOpen_CreatesAndMigrates(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { _ = os.Remove(path) })

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var count int
	err = db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}

	if count == 0 {
		t.Error("Open should have run migrations")
	}
}

func TestMigrations_Ordered(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { _ = os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	rows, err := db.QueryContext(context.Background(), "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	t.Cleanup(func() { _ = rows.Close() })

	var versions []int
	for rows.Next() {
		var version int

		err = rows.Scan(&version)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}

		versions = append(versions, version)
	}

	err = rows.Err()
	if err != nil {
		t.Fatalf("rows: %v", err)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i] <= versions[i-1] {
			t.Errorf("versions not ordered: %v", versions)
		}
	}
}
