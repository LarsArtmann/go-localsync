package transform

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
)

func TestFromProviderItem_Valid(t *testing.T) {
	t.Parallel()

	p := &model.ProviderItem{
		ExternalID:     id.NewExternalID("event-123"),
		Source:         id.NewProviderID("github"),
		Type:           id.NewEventTypeID("PushEvent"),
		ActorLogin:     id.NewActorID("octocat"),
		ActorAvatarURL: "https://example.com/avatar.png",
		RepoName:       id.NewRepoID("org/repo"),
		RepoURL:        "https://github.com/org/repo",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		RawPayload:     []byte(`{"id": 123}`),
	}

	ctx := context.Background()
	item, err := NewFromProviderItem().Map(ctx, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.ExternalID != p.ExternalID {
		t.Error("ExternalID mismatch")
	}

	if item.Source != p.Source {
		t.Error("Source mismatch")
	}

	if item.Type != p.Type {
		t.Error("Type mismatch")
	}

	if item.ActorLogin != p.ActorLogin {
		t.Error("ActorLogin mismatch")
	}

	if item.RepoName != p.RepoName {
		t.Error("RepoName mismatch")
	}

	if item.ActorAvatarURL != p.ActorAvatarURL {
		t.Error("ActorAvatarURL mismatch")
	}

	if item.RepoURL != p.RepoURL {
		t.Error("RepoURL mismatch")
	}
}

func TestFromProviderItem_Nil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, err := NewFromProviderItem().Map(ctx, nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestFromProviderItem_Invalid(t *testing.T) {
	t.Parallel()

	p := &model.ProviderItem{
		Source:    id.NewProviderID("github"),
		Type:      id.NewEventTypeID("PushEvent"),
		CreatedAt: time.Now(),
	}

	ctx := context.Background()
	_, err := NewFromProviderItem().Map(ctx, p)
	if err == nil {
		t.Error("expected error for invalid input (missing ExternalID)")
	}
}

func TestToItemView_Valid(t *testing.T) {
	t.Parallel()

	item := &model.Item{
		ExternalID: id.NewExternalID("event-123"),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID("PushEvent"),
		CreatedAt:  time.Now(),
	}

	ctx := context.Background()
	view, err := NewToItemView().Map(ctx, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if view.ExternalID != item.ExternalID {
		t.Error("ExternalID mismatch")
	}

	if view.IsDeleted {
		t.Error("expected IsDeleted = false")
	}
}

func TestToItemView_Nil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, err := NewToItemView().Map(ctx, nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestCompose(t *testing.T) {
	t.Parallel()

	// Create a simple A→B mapper
	toString := NewMapper(func(_ context.Context, n int) (string, error) {
		return string(rune('a' + n)), nil //nolint:gosec // test: n is always 1-3, no overflow possible
	})

	// Create a simple B→C mapper
	toUpper := NewMapper(func(_ context.Context, s string) (string, error) {
		return s + s, nil
	})

	// Compose them: int → string → string
	composed := Compose(toString, toUpper)

	ctx := context.Background()
	result, err := composed.Map(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "aa" {
		t.Errorf("result = %q, want %q", result, "aa")
	}
}

func TestCompose_FirstError(t *testing.T) {
	t.Parallel()

	failFirst := NewMapper(func(_ context.Context, _ int) (string, error) {
		return "", errNilInput
	})

	passSecond := NewMapper(func(_ context.Context, s string) (string, error) {
		return s, nil
	})

	composed := Compose(failFirst, passSecond)

	ctx := context.Background()
	_, err := composed.Map(ctx, 0)
	if err == nil {
		t.Error("expected error from first mapper")
	}
}

func TestCompose_SecondError(t *testing.T) {
	t.Parallel()

	passFirst := NewMapper(func(_ context.Context, n int) (string, error) {
		return "ok", nil
	})

	failSecond := NewMapper(func(_ context.Context, _ string) (string, error) {
		return "", errNilInput
	})

	composed := Compose(passFirst, failSecond)

	ctx := context.Background()
	_, err := composed.Map(ctx, 0)
	if err == nil {
		t.Error("expected error from second mapper")
	}
}

func TestProviderToView(t *testing.T) {
	t.Parallel()

	p := &model.ProviderItem{
		ExternalID: id.NewExternalID("event-123"),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID("PushEvent"),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	view, err := NewProviderToView().Map(ctx, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if view.ExternalID != p.ExternalID {
		t.Error("ExternalID mismatch in composed pipeline")
	}

	if view.Source != p.Source {
		t.Error("Source mismatch in composed pipeline")
	}
}
