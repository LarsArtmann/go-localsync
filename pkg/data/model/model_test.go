package model

import (
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/schema"
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

			if got := tt.key.IsZero(); got != tt.want {
				t.Errorf("Key.IsZero() = %v, want %v", got, tt.want)
			}
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

			if got := tt.item.IsZero(); got != tt.want {
				t.Errorf("Item.IsZero() = %v, want %v", got, tt.want)
			}
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
