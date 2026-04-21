package storage

import (
	"testing"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStorage_MemoryBackend(t *testing.T) {
	cfg := NewConfig(WithBackend(BackendMemory))
	store, err := NewStorage(cfg)
	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestNewStorage_DefaultIsSQLite(t *testing.T) {
	cfg := NewConfig()
	assert.Equal(t, BackendSQLite, cfg.Backend)
}

func TestNewStorage_SQLiteWithoutPath(t *testing.T) {
	cfg := NewConfig(WithBackend(BackendSQLite))
	_, err := NewStorage(cfg)
	assert.ErrorIs(t, err, pkgerrors.ErrDatabase)
}

func TestNewStorage_UnknownBackend(t *testing.T) {
	cfg := NewConfig(WithBackend("nonexistent"))
	_, err := NewStorage(cfg)
	assert.ErrorIs(t, err, pkgerrors.ErrDatabase)
}

func TestNewStorage_MemoryImplementsInterface(t *testing.T) {
	cfg := NewConfig(WithBackend(BackendMemory))
	store, err := NewStorage(cfg)
	require.NoError(t, err)
	defer store.Close()

	assert.NotNil(t, store)
}
