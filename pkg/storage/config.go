package storage

import (
	"fmt"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

// Backend identifies a storage backend.
type Backend string

const (
	// BackendSQLite uses the embedded SQLite database (default).
	BackendSQLite Backend = "sqlite"
	// BackendMemory uses an in-memory store (testing, development).
	BackendMemory Backend = "memory"
	// BackendLibSQL uses LibSQL (local file or remote Turso database).
	BackendLibSQL Backend = "libsql"
)

// Config holds configuration for constructing storage backends.
type Config struct {
	Backend   Backend
	DBPath    string
	AuthToken string
}

// Option configures a Config.
type Option func(*Config)

// WithBackend sets the storage backend.
func WithBackend(b Backend) Option {
	return func(c *Config) { c.Backend = b }
}

// WithDBPath sets the database file path for persistent backends.
func WithDBPath(path string) Option {
	return func(c *Config) { c.DBPath = path }
}

// WithAuthToken sets the auth token for remote backends (e.g. Turso).
func WithAuthToken(token string) Option {
	return func(c *Config) { c.AuthToken = token }
}

// NewConfig builds a Config from options, applying defaults.
func NewConfig(opts ...Option) Config {
	cfg := Config{
		Backend: BackendSQLite,
		DBPath:  "",
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// NewStorage creates a Storage based on the configuration.
// External backends should implement the Storage interface directly.
func NewStorage(cfg Config) (Storage, error) {
	switch cfg.Backend {
	case BackendSQLite:
		path := cfg.DBPath
		if path == "" {
			return nil, fmt.Errorf(
				"%w: db path is required for sqlite backend",
				pkgerrors.ErrDatabase,
			)
		}

		return Open(path)
	case BackendMemory:
		return NewMemoryStorage(), nil
	case BackendLibSQL:
		url := cfg.DBPath
		if url == "" {
			return nil, fmt.Errorf(
				"%w: url is required for libsql backend",
				pkgerrors.ErrDatabase,
			)
		}

		return OpenLibSQL(url, cfg.AuthToken)
	default:
		return nil, fmt.Errorf(
			"%w: unknown storage backend: %q",
			pkgerrors.ErrDatabase,
			cfg.Backend,
		)
	}
}
