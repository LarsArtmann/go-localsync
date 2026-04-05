# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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
