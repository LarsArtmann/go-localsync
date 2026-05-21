package provider

import (
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func assertValidationError(t *testing.T, item *Item) {
	t.Helper()

	if !errors.Is(item.Validate(), pkgerrors.ErrInvalidInput) {
		t.Error("expected ErrInvalidInput")
	}
}

func TestItem_Validate(t *testing.T) {
	validItem := &Item{
		ID:         types.NewItemID(),
		ExternalID: types.NewExternalID("123"),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID("PushEvent"),
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
		item.ExternalID = types.ExternalID{}
		assertValidationError(t, &item)
	})

	t.Run("rejects zero Source", func(t *testing.T) {
		item := *validItem
		item.Source = types.ProviderID{}
		assertValidationError(t, &item)
	})

	t.Run("rejects zero Type", func(t *testing.T) {
		item := *validItem
		item.Type = types.EventTypeID{}
		assertValidationError(t, &item)
	})

	t.Run("rejects zero CreatedAt", func(t *testing.T) {
		item := *validItem
		item.CreatedAt = time.Time{}
		assertValidationError(t, &item)
	})
}
