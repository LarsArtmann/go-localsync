package cqrs

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	_ "modernc.org/sqlite"
)

func benchmarkReadModelList(b *testing.B, rm ReadModel) {
	b.Helper()
	ctx := context.Background()
	// Seed with 1000 items.
	for i := range 1000 {
		item := &model.Item{
			ID:         id.NewItemID(),
			ExternalID: id.NewExternalID(fmt.Sprintf("item-%d", i)),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			ActorLogin: id.NewActorID("testuser"),
			RepoName:   id.NewRepoID("test/repo"),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		_ = rm.Upsert(ctx, item)
	}
	b.ResetTimer()
	for range b.N {
		_, err := rm.List(ctx, model.ItemFilter{Limit: 100, Offset: 0})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryReadModel_List(b *testing.B) {
	rm := NewMemoryReadModel()
	benchmarkReadModelList(b, rm)
}

func BenchmarkSQLiteReadModel_List(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	rm, err := NewSQLiteReadModel(db)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = rm.Close() }()
	benchmarkReadModelList(b, rm)
}
