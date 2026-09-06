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
		Source:   id.NewProviderID("github"),
		SourceID: id.NewSourceID("12345"),
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
		{"only sourceID", Key{SourceID: id.NewSourceID("x")}, false},
		{"both", Key{Source: id.NewProviderID("github"), SourceID: id.NewSourceID("x")}, false},
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

	a := Key{Source: id.NewProviderID("github"), SourceID: id.NewSourceID("123")}
	b := Key{Source: id.NewProviderID("github"), SourceID: id.NewSourceID("123")}
	c := Key{Source: id.NewProviderID("gitlab"), SourceID: id.NewSourceID("123")}
	d := Key{Source: id.NewProviderID("github"), SourceID: id.NewSourceID("456")}

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
		Source:   id.NewProviderID("github"),
		SourceID: id.NewSourceID("event-123"),
	}

	key := item.Key()

	if key.Source != item.Source {
		t.Errorf("Key.Source = %v, want %v", key.Source, item.Source)
	}

	if key.SourceID != item.SourceID {
		t.Errorf("Key.SourceID = %v, want %v", key.SourceID, item.SourceID)
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
		{"only sourceID", Item{SourceID: id.NewSourceID("x")}, false},
		{"both", Item{Source: id.NewProviderID("github"), SourceID: id.NewSourceID("x")}, false},
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
				SourceID:  id.NewSourceID("123"),
				Source:    id.NewProviderID("github"),
				Type:      id.NewEventTypeID("PushEvent"),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing sourceID",
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
				SourceID:  id.NewSourceID("123"),
				Type:      id.NewEventTypeID("PushEvent"),
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing type",
			item: Item{
				SourceID:  id.NewSourceID("123"),
				Source:    id.NewProviderID("github"),
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing createdAt",
			item: Item{
				SourceID:  id.NewSourceID("123"),
				Source:    id.NewProviderID("github"),
				Type:      id.NewEventTypeID("PushEvent"),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing updatedAt",
			item: Item{
				SourceID:  id.NewSourceID("123"),
				Source:    id.NewProviderID("github"),
				Type:      id.NewEventTypeID("PushEvent"),
				CreatedAt: time.Now(),
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
		"sourceID", "source", "type", "createdAt", "updatedAt",
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
	external := id.NewSourceID("event-42")

	key := ItemKey(source, external)

	if key.Source != source {
		t.Errorf("Source = %v, want %v", key.Source, source)
	}

	if key.SourceID != external {
		t.Errorf("SourceID = %v, want %v", key.SourceID, external)
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

// TestItem_TypedAttributeWriteHelpers pins the With* write-helper contract:
// values land under the canonical Attr* keys, the receiver is copied (the
// original item and any map sharing its backing array stay untouched), and a
// nil Attributes map is allocated on first write.
func TestItem_TypedAttributeWriteHelpers(t *testing.T) {
	t.Parallel()

	original := Item{}
	withActor := original.WithActorLogin("alice")

	if original.Attributes != nil {
		t.Error("WithActorLogin must not allocate on the receiver copy source")
	}

	if got := withActor.ActorLogin(); got != "alice" {
		t.Errorf("ActorLogin() = %q, want %q", got, "alice")
	}

	both := withActor.
		WithActorAvatarURL("https://example.com/a.png").
		WithRepoName("owner/repo").
		WithRepoURL("https://github.com/owner/repo")

	want := map[string]string{
		AttrActorLogin:     "alice",
		AttrActorAvatarURL: "https://example.com/a.png",
		AttrRepoName:       "owner/repo",
		AttrRepoURL:        "https://github.com/owner/repo",
	}

	if len(both.Attributes) != len(want) {
		t.Fatalf("attributes = %v, want %v", both.Attributes, want)
	}

	for k, v := range want {
		if both.Attributes[k] != v {
			t.Errorf("attributes[%s] = %q, want %q", k, both.Attributes[k], v)
		}
	}

	// Copy semantics: writing on a derived item must not leak into an item
	// that shared the original map.
	shared := Item{Attributes: map[string]string{AttrActorLogin: "bob"}}
	derived := shared.WithActorLogin("carol")

	if shared.Attributes[AttrActorLogin] != "bob" {
		t.Errorf(
			"shared map mutated: attributes[%s] = %q, want %q",
			AttrActorLogin,
			shared.Attributes[AttrActorLogin],
			"bob",
		)
	}

	if derived.Attributes[AttrActorLogin] != "carol" {
		t.Errorf("derived attributes[%s] = %q, want %q", AttrActorLogin, derived.Attributes[AttrActorLogin], "carol")
	}
}
