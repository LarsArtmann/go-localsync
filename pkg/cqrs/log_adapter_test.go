package cqrs

import (
	"testing"

	"charm.land/log/v2"
)

func TestCharmLogAdapter_Info(t *testing.T) {
	t.Parallel()

	logger := log.Default()
	adapter := &charmLogAdapter{logger: logger}

	// Should not panic
	adapter.Info("test info message", "key", "value")
}

func TestCharmLogAdapter_Error(t *testing.T) {
	t.Parallel()

	logger := log.Default()
	adapter := &charmLogAdapter{logger: logger}

	// Should not panic — this path was never triggered before
	adapter.Error("test error message", "key", "value")
}
