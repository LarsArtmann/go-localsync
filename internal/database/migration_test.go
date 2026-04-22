package database

import (
	"database/sql"
	"os"
	"testing"
)

func TestRunMigrations_FreshDB(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}

	if count != len(migrations) {
		t.Errorf("expected %d migrations, got %d", len(migrations), count)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := RunMigrations(db); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}

	if count != len(migrations) {
		t.Errorf("expected %d migrations after idempotent run, got %d", len(migrations), count)
	}
}

func TestRunMigrations_CreatesEventsTable(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='events'").
		Scan(&name)
	if err != nil {
		t.Fatal("events table not found after migration")
	}
}

func TestRunMigrations_CreatesIndexes(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := RunMigrations(db); err != nil {
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
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).
			Scan(&name)
		if err != nil {
			t.Errorf("index %s not found: %v", idx, err)
		}
	}
}

func TestOpen_CreatesAndMigrates(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { os.Remove(path) })

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}

	if count == 0 {
		t.Error("Open should have run migrations")
	}
}

func TestMigrations_Ordered(t *testing.T) {
	path := t.Name() + ".db"
	t.Cleanup(func() { os.Remove(path) })

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	t.Cleanup(func() { rows.Close() })

	var versions []int
	for rows.Next() {
		var v int
		err := rows.Scan(&v)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}

		versions = append(versions, v)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i] <= versions[i-1] {
			t.Errorf("versions not ordered: %v", versions)
		}
	}
}
