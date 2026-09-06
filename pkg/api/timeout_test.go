package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestSync_TimeoutStatusMapping pins the /sync timeout contract: a canceled
// request surfaces 499 (Client Closed Request) and a deadline-exceeded run
// 504 (Gateway Timeout) through pkgerrors.HTTPStatus, and the OpenAPI
// document declares exactly those — the previously declared 408 was
// unreachable by the central mapping.
func TestSync_TimeoutStatusMapping(t *testing.T) {
	t.Parallel()

	syncer := synclib.NewSyncer(&testutil.BlockingProvider{}, &mockSyncStore{}, log.Default())
	srv := NewServer(syncer, log.Default())

	post := func(ctx context.Context) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(
			ctx, http.MethodPost, "/sync", strings.NewReader(`{"source":"github","maxPages":1}`),
		)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		return rec
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	testutil.AssertStatus(t, post(canceledCtx), pkgerrors.StatusClientClosedRequest)

	deadlineCtx, cancelDeadline := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelDeadline()

	testutil.AssertStatus(t, post(deadlineCtx), http.StatusGatewayTimeout)

	responses := srv.api.OpenAPI().Paths["/sync"].Post.Responses
	if responses == nil {
		t.Fatal("/sync must declare error responses")
	}

	for _, want := range []string{"499", "504"} {
		if _, ok := responses[want]; !ok {
			t.Errorf("/sync OpenAPI must declare response %s", want)
		}
	}

	if _, ok := responses["408"]; ok {
		t.Error("/sync must no longer declare the unreachable 408")
	}
}
