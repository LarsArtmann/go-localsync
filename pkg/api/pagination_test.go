package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// pageStore serves len(items) items through the ItemFilter contract so the
// pagination headers and cursor semantics can be asserted precisely.
type pageStore struct {
	synclib.SyncStore

	items []*model.Item
}

func (p *pageStore) List(_ context.Context, filter model.ItemFilter) ([]*model.Item, error) {
	start := min(filter.Offset, len(p.items))

	end := start + filter.Limit
	if filter.Limit <= 0 || end > len(p.items) {
		end = len(p.items)
	}

	return p.items[start:end], nil
}

func (p *pageStore) Count(context.Context, model.ItemFilter) (int64, error) {
	return int64(len(p.items)), nil
}

func newPagedServer(t *testing.T, count int) *Server {
	t.Helper()

	items := make([]*model.Item, count)
	for i := range items {
		items[i] = &model.Item{
			ID:          id.NewItemID(),
			ExternalID:  id.NewExternalID(string(rune('a' + i))),
			Source:      id.NewProviderID("github"),
			Type:        id.NewEventTypeID("PushEvent"),
			Attributes:  map[string]string{"actor_login": "u"},
			ContentHash: "h" + string(rune('a'+i)),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}

	syncer := synclib.NewSyncer(&testutil.MockProvider{}, &pageStore{items: items}, log.Default())

	return NewServer(syncer, log.Default())
}

func TestPagination_TotalCountHeader(t *testing.T) {
	t.Parallel()

	srv := newPagedServer(t, 7)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?limit=3", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusOK)

	if got := rec.Header().Get("X-Total-Count"); got != "7" {
		t.Errorf("X-Total-Count = %q, want 7", got)
	}

	if got := rec.Header().Get("X-Next-Cursor"); got == "" {
		t.Error("expected X-Next-Cursor on a partial page")
	}
}

func TestPagination_LastPageHasNoCursor(t *testing.T) {
	t.Parallel()

	srv := newPagedServer(t, 7)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?limit=5&offset=5", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusOK)

	if got := rec.Header().Get("X-Next-Cursor"); got != "" {
		t.Errorf("last page must have empty X-Next-Cursor, got %q", got)
	}
}

func TestPagination_CursorWalksPages(t *testing.T) {
	t.Parallel()

	srv := newPagedServer(t, 9)

	type page struct {
		Items []map[string]any
		Total int64
	}

	var pages []page

	url := "/items?limit=4"

	for range 5 {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		testutil.AssertStatus(t, rec, http.StatusOK)

		var body struct {
			Items []map[string]any `json:"items"`
			Total int64            `json:"total"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("bad body %q: %v", rec.Body.String(), err)
		}

		pages = append(pages, page{Items: body.Items, Total: body.Total})

		next := rec.Header().Get("X-Next-Cursor")
		if next == "" {
			break
		}

		url = "/items?limit=4&cursor=" + next
	}

	if len(pages) != 3 {
		t.Fatalf("expected 3 pages (4+4+1), got %d", len(pages))
	}

	total := 0

	for _, p := range pages {
		total += len(p.Items)
	}

	if total != 9 {
		t.Errorf("walked %d items across pages, want 9", total)
	}
}

func TestPagination_BadCursorIs400(t *testing.T) {
	t.Parallel()

	srv := newPagedServer(t, 5)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?cursor=Y3Vyc29y", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusBadRequest)
}
