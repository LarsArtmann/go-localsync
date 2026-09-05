package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// WithAPIKey requires a bearer or X-API-Key credential on every request
// except /health (which stays unauthenticated for liveness probes). Requests
// without a matching key get 401 via the standard error mapper. An empty key
// disables the option (defense in NewServer: the option is ignored).
func WithAPIKey(key string) ServerOption {
	return func(o *serverOptions) {
		o.apiKey = key
	}
}

// apiKeyHeader is the canonical header; Authorization: Bearer is also
// accepted so standard HTTP clients work without custom headers.
const apiKeyHeader = "X-API-Key"

func (s *Server) authenticated(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.opts == nil || s.opts.apiKey == "" || isPublicPath(r.URL.Path) {
			h.ServeHTTP(w, r)

			return
		}

		presented := r.Header.Get(apiKeyHeader)
		if presented == "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				presented = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if subtle.ConstantTimeCompare([]byte(presented), []byte(s.opts.apiKey)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="localsync"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			//nolint:errcheck // static body; nothing sensible to do on short write
			_, _ = w.Write([]byte(`{"error":"unauthorized","message":"missing or invalid API key"}` + "\n"))

			return
		}

		h.ServeHTTP(w, r)
	})
}

// isPublicPath lists endpoints that must stay reachable without credentials:
// /health backs liveness probes; the OpenAPI schema is docs, not data.
func isPublicPath(path string) bool {
	return path == "/health" || path == "/openapi.json" || path == "/openapi.yaml" || path == "/docs"
}

// registerSecurityScheme declares the API-key scheme on the OpenAPI document
// so generated clients surface the requirement.
func registerSecurityScheme(api huma.API) {
	if api.OpenAPI().Security == nil {
		api.OpenAPI().Security = make(map[string][]string)
	}

	if api.OpenAPI().Components.SecuritySchemes == nil {
		api.OpenAPI().Components.SecuritySchemes = make(map[string]*huma.SecurityScheme)
	}

	api.OpenAPI().Components.SecuritySchemes["apiKey"] = &huma.SecurityScheme{
		Type: "apiKey",
		In:   "header",
		Name: apiKeyHeader,
	}
}
