package auth

import "net/http"

// NewAllowlistMiddleware returns an HTTP middleware that marks requests to
// allowlisted paths as not requiring authentication. Uses exact path matching.
//
// Currently all requests pass through to next - auth is enforced by the
// ConnectRPC interceptor on RPC routes. This middleware establishes the
// allowlist pattern for future HTTP-level auth enforcement.
func NewAllowlistMiddleware(allowedPaths []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedPaths))
	for _, p := range allowedPaths {
		allowed[p] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = allowed
		next.ServeHTTP(w, r)
	})
}
