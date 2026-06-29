// Package provider defines the core interfaces for local-sync providers.
// Providers are data sources that can fetch items to be synced locally.
package provider

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"time"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// Item represents a single syncable item from any provider.
type Item struct {
	// ID is the internal ULID-based identifier for this item.
	// Generated on first insert and stable thereafter.
	ID id.ItemID `json:"id"`
	// ExternalID is the original identifier from the source system (e.g., GitHub event "1234567890").
	// Used for upsert conflict detection against the source.
	ExternalID id.ExternalID `json:"externalId"`
	// Source identifies which provider this item came from (e.g., "github", "gitlab").
	Source id.ProviderID `json:"source"`
	// Type categorizes the item (e.g., "PushEvent", "IssueEvent").
	Type id.EventTypeID `json:"type"`
	// ActorLogin is the username of the entity that triggered the item.
	ActorLogin id.ActorLogin `json:"actorLogin"`
	// ActorAvatarURL is the avatar URL of the actor.
	ActorAvatarURL string `json:"actorAvatarUrl,omitempty"`
	// RepoName is the repository name (e.g., "owner/repo").
	RepoName id.RepoID `json:"repoName"`
	// RepoURL is the repository URL.
	RepoURL string `json:"repoUrl,omitempty"`
	// CreatedAt is when the item was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the item was last modified at the source.
	// For immutable events (e.g., GitHub events), this equals CreatedAt.
	UpdatedAt time.Time `json:"updatedAt"`
	// RawJSON contains the complete original payload for full fidelity.
	RawJSON json.RawMessage `json:"rawJson"`
}

// String returns a human-readable summary of the Item for logging.
func (item *Item) String() string {
	return fmt.Sprintf(
		"Item{Source:%s ExternalID:%s Type:%s Actor:%s Repo:%s}",
		item.Source.Get(), item.ExternalID.Get(), item.Type.Get(),
		item.ActorLogin.Get(), item.RepoName.Get(),
	)
}

// Validate checks that the Item has all required fields set.
func (item *Item) Validate() error {
	var errs []error

	if item.ExternalID.IsZero() {
		errs = append(errs, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "item.ExternalID is required"))
	}

	if item.Source.IsZero() {
		errs = append(errs, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "item.Source is required"))
	}

	if item.Type.IsZero() {
		errs = append(errs, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "item.Type is required"))
	}

	if item.CreatedAt.IsZero() {
		errs = append(errs, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "item.CreatedAt is required"))
	}

	if item.UpdatedAt.IsZero() {
		errs = append(errs, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "item.UpdatedAt is required"))
	}

	return stderrors.Join(errs...)
}

// FetchOptions controls how items are fetched from a provider.
type FetchOptions struct {
	// Source identifies what to fetch (e.g., username for GitHub, project ID for GitLab).
	Source id.ProviderID
	// PerPage is the number of items per page.
	PerPage int
	// Page is the page number to fetch (1-indexed).
	Page int
}

// FetchResult contains the result of a fetch operation.
type FetchResult struct {
	// Items is the list of fetched items.
	Items []*Item
	// HasMore indicates if more pages are available.
	HasMore bool
	// RateLimit contains rate-limit info from the API response headers, if available.
	// nil when the provider does not expose rate-limit headers.
	RateLimit *RateLimitInfo
}

// RateLimitInfo contains rate limiting information.
type RateLimitInfo struct {
	// Limit is the total requests allowed per window.
	Limit int
	// Remaining is the requests left in current window.
	Remaining int
	// ResetAt is when the rate limit resets.
	ResetAt time.Time
}

// Provider defines the interface for a data source that can be synced.
type Provider interface {
	// Name returns the provider identifier (e.g., "github", "gitlab").
	Name() string

	// Fetch retrieves a single page of items.
	Fetch(ctx context.Context, opts *FetchOptions) (*FetchResult, error)

	// FetchAll retrieves all available items up to maxPages.
	FetchAll(ctx context.Context, source string, maxPages int) (*FetchResult, error)

	// GetRateLimit returns current rate limit information.
	// Returns nil if the provider doesn't have rate limiting.
	GetRateLimit(ctx context.Context) (*RateLimitInfo, error)
}

// RetryConfig configures retry behavior for transient errors.
type RetryConfig struct {
	// Enabled controls whether retry is performed.
	Enabled bool
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// InitialBackoff is the initial backoff duration.
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration.
	MaxBackoff time.Duration
}

// DefaultRetryConfig provides sensible defaults for retry behavior.
var DefaultRetryConfig = RetryConfig{
	Enabled:        true,
	MaxRetries:     3,
	InitialBackoff: 1 * time.Second,
	MaxBackoff:     30 * time.Second,
}
