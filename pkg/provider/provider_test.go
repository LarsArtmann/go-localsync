package provider

import (
	"encoding/json"
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

func TestItem_Validate(t *testing.T) {
	validItem := &Item{
		ID:        id.NewItemID(),
		SourceID:  id.NewSourceID("123"),
		Source:    id.NewProviderID("github"),
		Type:      id.NewEventTypeID("PushEvent"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	t.Run("valid item passes", func(t *testing.T) {
		err := validItem.Validate()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("rejects zero SourceID", func(t *testing.T) {
		item := *validItem
		item.SourceID = id.SourceID{}
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

	t.Run("rejects zero UpdatedAt", func(t *testing.T) {
		item := *validItem
		item.UpdatedAt = time.Time{}
		assertValidationError(t, &item)
	})

	t.Run("rejects multiple zero fields", func(t *testing.T) {
		item := *validItem
		item.SourceID = id.SourceID{}
		item.Source = id.ProviderID{}
		item.Type = id.EventTypeID{}
		item.CreatedAt = time.Time{}
		assertValidationError(t, &item)
	})

	t.Run("rejects empty string SourceID", func(t *testing.T) {
		item := *validItem
		item.SourceID = id.NewSourceID("")
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

func TestItem_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := &Item{
		ID:       id.NewItemID(),
		SourceID: id.NewSourceID("12345"),
		Source:   id.NewProviderID("github"),
		Type:     id.NewEventTypeID("PushEvent"),
		Attributes: map[string]string{
			"actor_login":      "octocat",
			"actor_avatar_url": "https://avatar.url",
			"repo_name":        "org/repo",
			"repo_url":         "https://github.com/org/repo",
		},
		CreatedAt: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		RawJSON:   json.RawMessage(`{"action":"push"}`),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Item
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !decoded.ID.Equal(original.ID) {
		t.Errorf("ID: got %s, want %s", decoded.ID.String(), original.ID.String())
	}

	if decoded.SourceID.Get() != original.SourceID.Get() {
		t.Errorf("SourceID: got %s, want %s", decoded.SourceID.Get(), original.SourceID.Get())
	}

	if decoded.Source.Get() != original.Source.Get() {
		t.Errorf("Source: got %s, want %s", decoded.Source.Get(), original.Source.Get())
	}

	if decoded.Type.Get() != original.Type.Get() {
		t.Errorf("Type: got %s, want %s", decoded.Type.Get(), original.Type.Get())
	}

	if decoded.Attributes["actor_login"] != original.Attributes["actor_login"] {
		t.Errorf(
			"Attributes[actor_login]: got %s, want %s",
			decoded.Attributes["actor_login"],
			original.Attributes["actor_login"],
		)
	}

	if decoded.Attributes["actor_avatar_url"] != original.Attributes["actor_avatar_url"] {
		t.Errorf(
			"Attributes[actor_avatar_url]: got %s, want %s",
			decoded.Attributes["actor_avatar_url"],
			original.Attributes["actor_avatar_url"],
		)
	}

	if decoded.Attributes["repo_name"] != original.Attributes["repo_name"] {
		t.Errorf(
			"Attributes[repo_name]: got %s, want %s",
			decoded.Attributes["repo_name"],
			original.Attributes["repo_name"],
		)
	}

	if decoded.Attributes["repo_url"] != original.Attributes["repo_url"] {
		t.Errorf(
			"Attributes[repo_url]: got %s, want %s",
			decoded.Attributes["repo_url"],
			original.Attributes["repo_url"],
		)
	}

	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", decoded.CreatedAt, original.CreatedAt)
	}

	if !decoded.UpdatedAt.Equal(original.UpdatedAt) {
		t.Errorf("UpdatedAt: got %v, want %v", decoded.UpdatedAt, original.UpdatedAt)
	}

	if string(decoded.RawJSON) != string(original.RawJSON) {
		t.Errorf("RawJSON: got %s, want %s", decoded.RawJSON, original.RawJSON)
	}
}
