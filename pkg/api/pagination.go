package api

import (
	"encoding/base64"
	"strconv"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

// Cursor encoding: base64("offset=<n>"). Opaque to clients, trivially
// debuggable by hand, and stable across restarts (pure function of offset).

func encodeCursor(offset int) string {
	return base64.URLEncoding.EncodeToString([]byte("offset=" + strconv.Itoa(offset)))
}

func decodeCursor(cursor string) (int, error) {
	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, pkgerrors.Wrapf(pkgerrors.ErrInvalidInput, "invalid cursor %q: not base64", cursor)
	}

	prefix := "offset="

	body := string(raw)
	if len(body) <= len(prefix) || body[:len(prefix)] != prefix {
		return 0, pkgerrors.Wrapf(pkgerrors.ErrInvalidInput, "invalid cursor %q: missing offset prefix", cursor)
	}

	offset, err := strconv.Atoi(body[len(prefix):])
	if err != nil || offset < 0 {
		return 0, pkgerrors.Wrapf(pkgerrors.ErrInvalidInput, "invalid cursor %q: bad offset", cursor)
	}

	return offset, nil
}

// nextCursor returns the opaque cursor for the next page, or "" when this
// page is the last one (fewer items than the limit, or the offset already
// reaches past the total).
func nextCursor(offset, limit, got int, total int64) string {
	if limit <= 0 || got < limit {
		return ""
	}

	next := offset + limit
	if int64(next) >= total {
		return ""
	}

	return encodeCursor(next)
}
