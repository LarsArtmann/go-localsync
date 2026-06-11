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

func TestItem_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := &Item{
		ID:             id.NewItemID(),
		ExternalID:     id.NewExternalID("12345"),
		Source:         id.NewProviderID("github"),
		Type:           id.NewEventTypeID("PushEvent"),
		ActorLogin:     id.NewActorID("octocat"),
		ActorAvatarURL: "https://avatar.url",
		RepoName:       id.NewRepoID("org/repo"),
		RepoURL:        "https://github.com/org/repo",
		CreatedAt:      time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		RawJSON:        json.RawMessage(`{"action":"push"}`),
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

	if decoded.ExternalID.Get() != original.ExternalID.Get() {
		t.Errorf("ExternalID: got %s, want %s", decoded.ExternalID.Get(), original.ExternalID.Get())
	}

	if decoded.Source.Get() != original.Source.Get() {
		t.Errorf("Source: got %s, want %s", decoded.Source.Get(), original.Source.Get())
	}

	if decoded.Type.Get() != original.Type.Get() {
		t.Errorf("Type: got %s, want %s", decoded.Type.Get(), original.Type.Get())
	}

	if decoded.ActorLogin.Get() != original.ActorLogin.Get() {
		t.Errorf("ActorLogin: got %s, want %s", decoded.ActorLogin.Get(), original.ActorLogin.Get())
	}

	if decoded.ActorAvatarURL != original.ActorAvatarURL {
		t.Errorf("ActorAvatarURL: got %s, want %s", decoded.ActorAvatarURL, original.ActorAvatarURL)
	}

	if decoded.RepoName.Get() != original.RepoName.Get() {
		t.Errorf("RepoName: got %s, want %s", decoded.RepoName.Get(), original.RepoName.Get())
	}

	if decoded.RepoURL != original.RepoURL {
		t.Errorf("RepoURL: got %s, want %s", decoded.RepoURL, original.RepoURL)
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
