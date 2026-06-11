package cqrs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/storage/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
	_ "modernc.org/sqlite"
)

func TestSQLiteReadModel_FilePersistence(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persistence-test.db")
	ctx := context.Background()

	db1, err := storage.OpenSQLite(dbPath)
	testutil.MustNoError(t, err)

	rm1, err := NewSQLiteReadModel(db1)
	testutil.MustNoError(t, err)

	item := &model.Item{
		ID:         id.NewItemID(),
		ExternalID: id.NewExternalID("persist-1"),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID("PushEvent"),
		ActorLogin: id.NewActorID("alice"),
		RepoName:   id.NewRepoID("org/repo"),
		CreatedAt:  testFutureNow(0),
		UpdatedAt:  testFutureNow(0),
	}

	testutil.MustNoError(t, rm1.Upsert(ctx, item))
	testutil.MustNoError(t, rm1.Close())

	db2, err := storage.OpenSQLite(dbPath)
	testutil.MustNoError(t, err)
	t.Cleanup(func() { _ = db2.Close() })

	rm2, err := NewSQLiteReadModel(db2)
	testutil.MustNoError(t, err)

	got, err := rm2.Get(ctx, "github", id.NewExternalID("persist-1"))
	testutil.MustNoError(t, err)

	if got == nil {
		t.Fatal("item should survive across read model restarts")
	}

	testutil.AssertEqual(t, got.ExternalID.Get(), "persist-1", "ExternalID")
	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type")

	count, err := rm2.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 1, "Count")
}
