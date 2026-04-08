# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- Migration system with version tracking (`internal/database/migration.go`)
- `schema_migrations` table for tracking applied database migrations
- Source provider indexes (`idx_events_source`, `idx_events_source_github_id`)
- Conflict-aware sync engine (`pkg/sync/conflict_aware.go`) using go-localfirst CRDT primitives
- `UpdatedAt` field on `provider.Item` for proper LWW conflict resolution
- `source` column tracking in events table for multi-provider support
- `GetByID` method on `Storage` interface for ID-based lookups
- Branded IDs from go-composable-business-types (`ItemID`, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`)
- Shared test helpers in `pkg/testhelpers/` for mocks and factories
- Comprehensive execution plan document (`docs/planning/`)
- golangci-lint v2 configuration with extensive linter enablement

### Changed

- Database schema now uses migration system instead of inline schema creation
- Upsert SQL uses `DO UPDATE SET` with `excluded.updated_at` instead of `CURRENT_TIMESTAMP`
- `toDBParams()` now passes `UpdatedAt` from provider data
- Replaced all `interface{}` with `any` in sqlc-generated code
- Removed `go.mod` replace directives in favor of GitHub pseudo-versions
- Updated justfile lint target to use `golangci-lint run` with 5m timeout
- Added `lint-fix` and `fmt` recipes to justfile
- Added `.DS_Store`, `go.work`, `go.work.sum` to `.gitignore`

### Fixed

- `findExistingItem` now uses `GetByID` instead of `GetItems(1, 0)` for correct ID lookup
- LWW resolver compares `UpdatedAt` instead of `CreatedAt`
- All `provider.Item` construction sites now set `UpdatedAt`
- Storage layer error wrapping with `ErrNotFound` sentinel

## [0.2.0] - 2026-03-29

### Added

- GitHub client tests (13 comprehensive test functions with mock HTTP server)
- Sync package tests (NewSyncer, Sync, SyncIncremental, GetStats, Close)
- Typed/sentinel errors (ErrNotFound, ErrInvalidInput, ErrDatabase, ErrGitHubAPI, ErrRateLimited, ErrUnauthorized, ErrConfiguration)
- CI/CD Pipeline (GitHub Actions with test, lint, build, release jobs)
- Domain Event types (`pkg/event/event.go`) - decoupled from storage layer
- Build version injection via ldflags (version, commit, date)

### Changed

- Fixed golangci-lint warnings (containedctx, duplicate code, funlen)
- Refactored SyncIncremental for better maintainability (extracted processIncrementalItems helper)

### Fixed

- go.mod indirect dependencies (ran `go mod tidy`)
- Removed unused Config struct from `pkg/sync/sync.go`
- Removed unused exitOK constant from `cmd/examples/github-sync/main.go`
- Error handling for DB close operations (now logged instead of silently ignored)
- Version build injection in justfile

### Security

## [0.1.0] - 2026-01-01

### Added

- Initial release
- Basic GitHub event sync functionality
