package provider

import (
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		require.NoError(t, err)
	})

	t.Run("rejects zero ExternalID", func(t *testing.T) {
		item := *validItem
		item.ExternalID = types.ExternalID{}
		assert.True(t, errors.Is(item.Validate(), pkgerrors.ErrInvalidInput))
	})

	t.Run("rejects zero Source", func(t *testing.T) {
		item := *validItem
		item.Source = types.ProviderID{}
		assert.True(t, errors.Is(item.Validate(), pkgerrors.ErrInvalidInput))
	})

	t.Run("rejects zero Type", func(t *testing.T) {
		item := *validItem
		item.Type = types.EventTypeID{}
		assert.True(t, errors.Is(item.Validate(), pkgerrors.ErrInvalidInput))
	})

	t.Run("rejects zero CreatedAt", func(t *testing.T) {
		item := *validItem
		item.CreatedAt = time.Time{}
		assert.True(t, errors.Is(item.Validate(), pkgerrors.ErrInvalidInput))
	})
}
