package cqrs

import (
	"testing"

	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func TestAggregateID_Deterministic(t *testing.T) {
	t.Parallel()

	source := "github"
	externalID := types.NewExternalID("12345")

	a := AggregateID(source, externalID)
	b := AggregateID(source, externalID)

	if !a.Equal(b) {
		t.Errorf("AggregateID not deterministic: %s != %s", a.Get(), b.Get())
	}
}

func TestAggregateID_DifferentInputs(t *testing.T) {
	t.Parallel()

	a := AggregateID("github", types.NewExternalID("123"))
	b := AggregateID("github", types.NewExternalID("456"))
	c := AggregateID("gitlab", types.NewExternalID("123"))

	if a.Equal(b) {
		t.Error("expected different externalIDs to produce different aggregate IDs")
	}

	if a.Equal(c) {
		t.Error("expected different sources to produce different aggregate IDs")
	}
}

func TestAggregateID_ValidFormat(t *testing.T) {
	t.Parallel()

	aggID := AggregateID("github", types.NewExternalID("123"))

	if aggID.IsZero() {
		t.Error("expected non-zero aggregate ID")
	}

	s := aggID.Get()
	if len(s) == 0 {
		t.Error("expected non-empty string representation")
	}
}

func TestAggregateID_ItemKey(t *testing.T) {
	t.Parallel()

	source := "github"
	externalID := types.NewExternalID("12345")

	key := itemKey(source, externalID)
	expected := "github:12345"

	if key != expected {
		t.Errorf("itemKey mismatch: expected %q, got %q", expected, key)
	}
}

func TestAggregateID_IntegrationWithMustParse(t *testing.T) {
	t.Parallel()

	aggID := AggregateID("github", types.NewExternalID("123"))

	parsed := id.MustParseAggregateID(aggID.Get())

	if !aggID.Equal(parsed) {
		t.Errorf("round-trip failed: %s != %s", aggID.Get(), parsed.Get())
	}
}
