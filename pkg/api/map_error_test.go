package api

import (
	"errors"
	"net/http"
	"testing"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

func TestMapSyncError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"rate limited maps to 429", pkgerrors.ErrRateLimited, http.StatusTooManyRequests},
		{"invalid token maps to 401", pkgerrors.ErrInvalidToken, http.StatusUnauthorized},
		{"user not found maps to 404", pkgerrors.ErrUserNotFound, http.StatusNotFound},
		{"not found maps to 404", pkgerrors.ErrNotFound, http.StatusNotFound},
		{"database error maps to 500", pkgerrors.ErrDatabase, http.StatusInternalServerError},
		{"invalid input maps to 400", pkgerrors.ErrInvalidInput, http.StatusBadRequest},
		{"unknown backend maps to 500", pkgerrors.ErrUnknownBackend, http.StatusInternalServerError},
		{"unknown error maps to 503", errors.New("something unexpected"), http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			humaErr := mapSyncError(tt.err)

			var httpErr interface{ GetStatus() int }
			if !errors.As(humaErr, &httpErr) {
				t.Fatalf("expected huma error, got %T: %v", humaErr, humaErr)
			}

			if httpErr.GetStatus() != tt.wantStatus {
				t.Errorf("got status %d, want %d", httpErr.GetStatus(), tt.wantStatus)
			}
		})
	}
}
