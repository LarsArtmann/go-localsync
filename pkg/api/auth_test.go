package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/log/v2"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func newAuthTestServer(t *testing.T, key string) *Server {
	t.Helper()

	store := &mockSyncStore{}
	syncer := synclib.NewSyncer(&testutil.MockProvider{}, store, log.Default())

	return NewServer(syncer, log.Default(), WithAPIKey(key))
}

func TestAPIKeyAuth_Required(t *testing.T) {
	t.Parallel()

	srv := newAuthTestServer(t, "secret-key")

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"no credential", nil, http.StatusUnauthorized},
		{"wrong credential", map[string]string{apiKeyHeader: "wrong"}, http.StatusUnauthorized},
		{"wrong bearer", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
		{"header credential", map[string]string{apiKeyHeader: "secret-key"}, http.StatusOK},
		{"bearer credential", map[string]string{"Authorization": "Bearer secret-key"}, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/stats", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			testutil.AssertStatus(t, rec, tc.want)
		})
	}
}

func TestAPIKeyAuth_HealthStaysPublic(t *testing.T) {
	t.Parallel()

	srv := newAuthTestServer(t, "secret-key")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusOK)
}

func TestAPIKeyAuth_OffByDefault(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&mockSyncStore{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatal("without WithAPIKey every endpoint must stay open")
	}
}

func TestAPIKeyAuth_401BodyIsJSON(t *testing.T) {
	t.Parallel()

	srv := newAuthTestServer(t, "secret-key")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusUnauthorized)

	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Error("401 must carry WWW-Authenticate")
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("401 body must be JSON, got %q: %v", rec.Body.String(), err)
	}

	if body["error"] != "unauthorized" {
		t.Errorf("401 body error field = %q, want unauthorized", body["error"])
	}
}

func TestAPIKeyAuth_OpenAPIDeclaresScheme(t *testing.T) {
	t.Parallel()

	srv := newAuthTestServer(t, "secret-key")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusOK)

	var doc struct {
		Components struct {
			SecuritySchemes map[string]struct {
				Type string `json:"type"`
				In   string `json:"in"`
				Name string `json:"name"`
			} `json:"securitySchemes"`
		} `json:"components"`
		Security []map[string][]string `json:"security"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("openapi.json is not JSON: %v", err)
	}

	scheme, ok := doc.Components.SecuritySchemes["apiKey"]
	if !ok {
		t.Fatal("openapi.json must declare the apiKey security scheme")
	}

	if scheme.Type != "apiKey" || scheme.In != "header" || scheme.Name != apiKeyHeader {
		t.Errorf("scheme = %+v, want apiKey in header %s", scheme, apiKeyHeader)
	}

	if len(doc.Security) == 0 {
		t.Error("openapi.json must apply the security requirement globally")
	}
}
