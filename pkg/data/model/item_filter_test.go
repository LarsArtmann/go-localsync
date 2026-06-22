package model

import (
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

func TestItemFilter_Builder(t *testing.T) {
	t.Parallel()

	now := time.Now()
	typeID := id.NewEventTypeID("PushEvent")
	actorID := id.NewActorLogin("testuser")
	repoID := id.NewRepoID("test/repo")
	sourceID := id.NewProviderID("github")

	f := ItemFilter{}.
		WithType(typeID).
		WithActorLogin(actorID).
		WithRepoName(repoID).
		WithSource(sourceID).
		WithSince(now).
		WithLimit(10).
		WithOffset(5)

	if f.Type == nil || *f.Type != typeID {
		t.Error("Type not set correctly")
	}

	if f.ActorLogin == nil || *f.ActorLogin != actorID {
		t.Error("ActorLogin not set correctly")
	}

	if f.RepoName == nil || *f.RepoName != repoID {
		t.Error("RepoName not set correctly")
	}

	if f.Source == nil || *f.Source != sourceID {
		t.Error("Source not set correctly")
	}

	if f.Since == nil || !f.Since.Equal(now) {
		t.Error("Since not set correctly")
	}

	if f.Limit != 10 {
		t.Errorf("Limit=%d, want 10", f.Limit)
	}

	if f.Offset != 5 {
		t.Errorf("Offset=%d, want 5", f.Offset)
	}
}
