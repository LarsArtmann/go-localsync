package main

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// AppConfig holds all configuration for the github-sync application.
// Values are populated from environment variables with optional defaults.
// Command-line flags override environment variables.
type AppConfig struct {
	// Token is the GitHub personal access token.
	Token string `env:"GITHUB_TOKEN"`
	// Username is the GitHub username to sync events for.
	Username string `env:"GITHUB_USER"`
	// DBPath is the path to the database file.
	DBPath string `env:"DB_PATH"`
	// Backend is the storage backend: sqlite, memory.
	Backend string `env:"BACKEND" envDefault:"memory"`
	// MaxPages is the maximum number of pages to fetch.
	MaxPages int `env:"MAX_PAGES" envDefault:"10"`
	// Incremental enables incremental sync (only new events).
	Incremental bool `env:"INCREMENTAL" envDefault:"true"`
	// ConflictAware enables conflict-aware sync with CRDT resolution.
	ConflictAware bool `env:"CONFLICT_AWARE" envDefault:"false"`
	// ConflictStrategy selects the conflict resolver: remote-wins, lww.
	ConflictStrategy string `env:"CONFLICT_STRATEGY" envDefault:"remote-wins"`
	// ShowStats shows database statistics and exits.
	ShowStats bool `env:"SHOW_STATS" envDefault:"false"`
	// Verbose enables debug-level logging.
	Verbose bool `env:"VERBOSE" envDefault:"false"`
	// JSONOutput outputs results as JSON.
	JSONOutput bool `env:"JSON_OUTPUT" envDefault:"false"`
}

// LoadConfig parses environment variables into AppConfig.
func LoadConfig() (AppConfig, error) {
	var cfg AppConfig
	if err := env.Parse(&cfg); err != nil {
		return AppConfig{}, fmt.Errorf("parse env config: %w", err)
	}

	return cfg, nil
}
