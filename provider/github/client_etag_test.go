package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	gh "github.com/google/go-github/v69/github"
	githubkit "github.com/LarsArtmann/go-github-kit"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// newETagTestServer serves one user's events with strong ETags. When the
// request's If-None-Match matches the current etag it answers 304 with an
// empty body; otherwise it serves the current body with a fresh ETag. The
// revision can be bumped between fetches to simulate upstream change.
type etagTestServer struct {
	mu       sync.Mutex
	server   *httptest.Server
	etag     string
	revision int
	requests int
}

func newETagTestServer(t *testing.T) *etagTestServer {
	t.Helper()

	s := &etagTestServer{etag: `"rev0"`}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests++
		etag, revision := s.etag, s.revision
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)

			return
		}

		w.Header().Set("ETag", etag)
		_ = json.MarshalWrite(w, []*gh.Event{
			newTestEvent(
				"etag-"+string(rune('a'+revision)),
				"PushEvent",
				time.Date(2024, 1, 1+revision, 12, 0, 0, 0, time.UTC),
			),
		})
	}))
	t.Cleanup(s.server.Close)

	return s
}

func (s *etagTestServer) bumpRevision(newETag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	s.etag = newETag
}

func (s *etagTestServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.requests
}

func TestETag_UnchangedRefetch_ServedFromCache(t *testing.T) {
	server := newETagTestServer(t)
	client := newTestClient(server.server).WithETagCache(githubkit.ETagOptions{})

	first, err := fetchFromTestClient(client, "octocat")
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, first.Items, 1, "first fetch items")

	hits, ok := client.ETagStats()
	if !ok {
		t.Fatal("expected ETagStats availability with cache enabled")
	}
	testutil.AssertEqual(t, hits.Stored, 1, "stored entries after first fetch")

	second, err := fetchFromTestClient(client, "octocat")
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, second.Items, 1, "second fetch items")
	testutil.AssertEqual(
		t,
		second.Items[0].ExternalID.Get(),
		first.Items[0].ExternalID.Get(),
		"cached body served for unchanged content",
	)

	hits, _ = client.ETagStats()
	testutil.AssertEqual(t, hits.Hits, 1, "304 revalidation served from cache")
	testutil.AssertEqual(t, hits.Entries, 1, "cache holds one entry")
}

func TestETag_ChangedContent_Refetches(t *testing.T) {
	server := newETagTestServer(t)
	client := newTestClient(server.server).WithETagCache(githubkit.ETagOptions{})

	first, err := fetchFromTestClient(client, "octocat")
	testutil.MustNoError(t, err)

	server.bumpRevision(`"rev1"`)

	second, err := fetchFromTestClient(client, "octocat")
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, second.Items, 1, "refetch items")

	if second.Items[0].ExternalID.Get() == first.Items[0].ExternalID.Get() {
		t.Fatalf("expected fresh item after content change, got same ID %s", first.Items[0].ExternalID.Get())
	}

	hits, _ := client.ETagStats()
	testutil.AssertEqual(t, hits.Hits, 0, "no cache hits for changed content")
	testutil.AssertEqual(t, hits.Stored, 2, "both revisions stored")
}

func TestETag_DisabledByDefault_EveryFetchHitsServer(t *testing.T) {
	server := newETagTestServer(t)
	client := newTestClient(server.server)

	for range 2 {
		_, err := fetchFromTestClient(client, "octocat")
		testutil.MustNoError(t, err)
	}

	testutil.AssertEqual(t, server.requestCount(), 2, "both fetches reached the server")

	_, ok := client.ETagStats()
	if ok {
		t.Fatal("expected ETagStats unavailability with cache disabled")
	}
}

func TestETag_DerivePreservesConfig(t *testing.T) {
	server := newETagTestServer(t)
	base := newTestClient(server.server).WithETagCache(githubkit.ETagOptions{})
	derived := base.WithFetchConfig(DefaultFetchConfig)

	_, err := fetchFromTestClient(base, "octocat")
	testutil.MustNoError(t, err)

	_, err = fetchFromTestClient(derived, "octocat")
	testutil.MustNoError(t, err)

	hits, ok := derived.ETagStats()
	if !ok {
		t.Fatal("expected derived client to keep the ETag cache")
	}
	testutil.AssertEqual(t, hits.Stored, 1, "derived client caches like the base")
}
