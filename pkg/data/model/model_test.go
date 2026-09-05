package model

import (
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/schema"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

func TestKeyString(t *testing.T) {
	t.Parallel()

	k := Key{
		Source:     id.NewProviderID("github"),
		ExternalID: id.NewExternalID("12345"),
	}

	if got, want := k.String(), "github/12345"; got != want {
		t.Errorf("Key.String() = %q, want %q", got, want)
	}
}

type isZeroer interface {
	IsZero() bool
}

func testIsZero[T isZeroer](t *testing.T, label string, subject T, want bool) {
	t.Helper()

	if got := subject.IsZero(); got != want {
		t.Errorf("%s.IsZero() = %v, want %v", label, got, want)
	}
}

func TestKeyIsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  Key
		want bool
	}{
		{"zero", Key{}, true},
		{"only source", Key{Source: id.NewProviderID("github")}, false},
		{"only externalID", Key{ExternalID: id.NewExternalID("x")}, false},
		{"both", Key{Source: id.NewProviderID("github"), ExternalID: id.NewExternalID("x")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testIsZero(t, "Key", tt.key, tt.want)
		})
	}
}

func TestKeyEquals(t *testing.T) {
	t.Parallel()

	a := Key{Source: id.NewProviderID("github"), ExternalID: id.NewExternalID("123")}
	b := Key{Source: id.NewProviderID("github"), ExternalID: id.NewExternalID("123")}
	c := Key{Source: id.NewProviderID("gitlab"), ExternalID: id.NewExternalID("123")}
	d := Key{Source: id.NewProviderID("github"), ExternalID: id.NewExternalID("456")}

	if !a.Equals(b) {
		t.Error("expected a.Equals(b) = true")
	}

	if a.Equals(c) {
		t.Error("expected a.Equals(c) = false")
	}

	if a.Equals(d) {
		t.Error("expected a.Equals(d) = false")
	}
}

func TestItemKey(t *testing.T) {
	t.Parallel()

	item := Item{
		Source:     id.NewProviderID("github"),
		ExternalID: id.NewExternalID("event-123"),
	}

	key := item.Key()

	if key.Source != item.Source {
		t.Errorf("Key.Source = %v, want %v", key.Source, item.Source)
	}

	if key.ExternalID != item.ExternalID {
		t.Errorf("Key.ExternalID = %v, want %v", key.ExternalID, item.ExternalID)
	}
}

func TestItemIsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item Item
		want bool
	}{
		{"zero", Item{}, true},
		{"only source", Item{Source: id.NewProviderID("github")}, false},
		{"only externalID", Item{ExternalID: id.NewExternalID("x")}, false},
		{"both", Item{Source: id.NewProviderID("github"), ExternalID: id.NewExternalID("x")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testIsZero(t, "Item", tt.item, tt.want)
		})
	}
}

func TestItemValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		item    Item
		wantErr bool
	}{
		{
			name: "valid",
			item: Item{
				ExternalID: id.NewExternalID("123"),
				Source:     id.NewProviderID("github"),
				Type:       id.NewEventTypeID("PushEvent"),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing externalID",
			item: Item{
				Source:    id.NewProviderID("github"),
				Type:      id.NewEventTypeID("PushEvent"),
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing source",
			item: Item{
				ExternalID: id.NewExternalID("123"),
				Type:       id.NewEventTypeID("PushEvent"),
				CreatedAt:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing type",
			item: Item{
				ExternalID: id.NewExternalID("123"),
				Source:     id.NewProviderID("github"),
				CreatedAt:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing createdAt",
			item: Item{
				ExternalID: id.NewExternalID("123"),
				Source:     id.NewProviderID("github"),
				Type:       id.NewEventTypeID("PushEvent"),
				UpdatedAt:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing updatedAt",
			item: Item{
				ExternalID: id.NewExternalID("123"),
				Source:     id.NewProviderID("github"),
				Type:       id.NewEventTypeID("PushEvent"),
				CreatedAt:  time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.item.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_CollectsAllFieldErrors verifies that Validate() returns all
// field errors in a single call via errors.Join, not just the first one.
// This is critical for UX — callers should fix all problems in one round
// instead of building, running, and rebuilding for each error.
func TestValidate_CollectsAllFieldErrors(t *testing.T) {
	t.Parallel()

	err := Item{}.Validate()
	if err == nil {
		t.Fatal("Validate() on zero item should fail")
	}

	// Model validation errors must classify as ErrInvalidInput so callers
	// can use errors.Is for retry/mapping decisions (session-28 fix).
	if !errors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("errors.Is(err, ErrInvalidInput) = false; err: %v", err)
	}

	msg := err.Error()
	for _, want := range []string{
		"externalID", "source", "type", "createdAt", "updatedAt",
	} {
		if !contains(msg, want) {
			t.Errorf("Validate() error should mention %q, got: %v", want, err)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestItemSchemaVersion(t *testing.T) {
	t.Parallel()

	item := Item{
		SchemaVersion: schema.V2,
	}

	if item.SchemaVersion != schema.V2 {
		t.Errorf("SchemaVersion = %v, want %v", item.SchemaVersion, schema.V2)
	}
}

func TestItemKeyConstructor(t *testing.T) {
	t.Parallel()

	source := id.NewProviderID("github")
	external := id.NewExternalID("event-42")

	key := ItemKey(source, external)

	if key.Source != source {
		t.Errorf("Source = %v, want %v", key.Source, source)
	}

	if key.ExternalID != external {
		t.Errorf("ExternalID = %v, want %v", key.ExternalID, external)
	}
}

func TestItem_AttributeAccessors(t *testing.T) {
	t.Parallel()

	full := Item{
		Attributes: map[string]string{
			AttrActorLogin:     "octocat",
			AttrActorAvatarURL: "https://avatars.example/u/1",
			AttrRepoName:       "octo/hello",
			AttrRepoURL:        "https://github.com/octo/hello",
		},
	}

	if full.ActorLogin() != "octocat" {
		t.Errorf("ActorLogin() = %q", full.ActorLogin())
	}

	if full.ActorAvatarURL() != "https://avatars.example/u/1" {
		t.Errorf("ActorAvatarURL() = %q", full.ActorAvatarURL())
	}

	if full.RepoName() != "octo/hello" {
		t.Errorf("RepoName() = %q", full.RepoName())
	}

	if full.RepoURL() != "https://github.com/octo/hello" {
		t.Errorf("RepoURL() = %q", full.RepoURL())
	}

	empty := Item{}
	for name, got := range map[string]string{
		"ActorLogin":     empty.ActorLogin(),
		"ActorAvatarURL": empty.ActorAvatarURL(),
		"RepoName":       empty.RepoName(),
		"RepoURL":        empty.RepoURL(),
	} {
		if got != "" {
			t.Errorf("%s() on nil Attributes must be empty, got %q", name, got)
		}
	}
}
