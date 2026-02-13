# go-localsync

A Go library and CLI for syncing GitHub User Events to a SQLite database.

## Features

- Sync GitHub user events to local SQLite database
- Incremental sync support (only fetch new events)
- Full JSON payload storage for 100% data fidelity
- Type-safe SQL queries with sqlc
- Pure Go SQLite driver (no CGO required)

## Installation

```bash
go install github.com/larsartmann/go-localsync/cmd/gh-sync@latest
```

## Usage

```bash
# Set your GitHub token
export GITHUB_TOKEN=your_token_here

# Sync events for a user
gh-sync -user octocat

# Sync with verbose output
gh-sync -user octocat -verbose

# Show database statistics
gh-sync -stats

# Full sync (not incremental)
gh-sync -user octocat -incremental=false
```

## CLI Flags

| Flag           | Default                                 | Description                        |
| -------------- | --------------------------------------- | ---------------------------------- |
| `-token`       | `$GITHUB_TOKEN`                         | GitHub personal access token       |
| `-user`        | (required)                              | GitHub username to sync events for |
| `-db`          | `~/.local/share/go-localsync/events.db` | Path to SQLite database            |
| `-pages`       | 10                                      | Maximum number of pages to fetch   |
| `-incremental` | true                                    | Only sync new events               |
| `-stats`       | false                                   | Show database statistics and exit  |
| `-version`     | false                                   | Show version information           |
| `-verbose`     | false                                   | Enable verbose logging             |

## Development

```bash
# Build
just build

# Run tests
just test

# Generate sqlc code
just sqlc

# Run linter
just lint
```

## Architecture

```
pkg/
├── github/     # GitHub API client
├── storage/    # Storage interface and SQLite implementation
└── sync/       # Sync logic (fetch → transform → store)

internal/
├── database/   # Database connection management
└── db/         # sqlc generated code

cmd/
└── gh-sync/    # CLI entrypoint
```

## License

MIT
