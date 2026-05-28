package provider

import (
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

func assertValidationError(t *testing.T, item *Item) {
	t.Helper()

	if !errors.Is(item.Validate(), pkgerrors.ErrInvalidInput) {
		t.Error("expected ErrInvalidInput")
	}
}

func TestItemFilter_Builder(t *testing.T) {
	now := time.Now()
	typeID := id.NewEventTypeID("PushEvent")
	actorID := id.NewActorID("testuser")
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

func TestItem_Validate(t *testing.T) {
	validItem := &Item{
		ID:         id.NewItemID(),
		ExternalID: id.NewExternalID("123"),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID("PushEvent"),
		CreatedAt:  time.Now(),
	}

	t.Run("valid item passes", func(t *testing.T) {
		err := validItem.Validate()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("rejects zero ExternalID", func(t *testing.T) {
		item := *validItem
		item.ExternalID = id.ExternalID{}
		assertValidationError(t, &item)
	})

	t.Run("rejects zero Source", func(t *testing.T) {
		item := *validItem
		item.Source = id.ProviderID{}
		assertValidationError(t, &item)
	})

	t.Run("rejects zero Type", func(t *testing.T) {
		item := *validItem
		item.Type = id.EventTypeID{}
		assertValidationError(t, &item)
	})

	t.Run("rejects zero CreatedAt", func(t *testing.T) {
		item := *validItem
		item.CreatedAt = time.Time{}
		assertValidationError(t, &item)
	})

	t.Run("rejects multiple zero fields", func(t *testing.T) {
		item := *validItem
		item.ExternalID = id.ExternalID{}
		item.Source = id.ProviderID{}
		item.Type = id.EventTypeID{}
		item.CreatedAt = time.Time{}
		assertValidationError(t, &item)
	})

	t.Run("rejects empty string ExternalID", func(t *testing.T) {
		item := *validItem
		item.ExternalID = id.NewExternalID("")
		assertValidationError(t, &item)
	})

	t.Run("rejects empty string Source", func(t *testing.T) {
		item := *validItem
		item.Source = id.NewProviderID("")
		assertValidationError(t, &item)
	})

	t.Run("rejects empty string Type", func(t *testing.T) {
		item := *validItem
		item.Type = id.NewEventTypeID("")
		assertValidationError(t, &item)
	})
}
