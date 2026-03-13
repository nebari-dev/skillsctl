package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

// NewInterceptor returns a ConnectRPC unary interceptor that validates
// Bearer tokens using the given validator. If validator is nil (local dev
// mode), all requests pass through without authentication.
func NewInterceptor(v TokenValidator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if v == nil {
				return next(ctx, req)
			}

			authHeader := req.Header().Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			// HasPrefix requires "Bearer " (7 chars) but the token after
			// trimming may still be empty, e.g. "Bearer " with no value.
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			claims, err := v.Validate(ctx, token)
			if err != nil {
				// Return a generic message to avoid leaking OIDC error details to clients.
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
			}

			ctx = WithClaims(ctx, claims)
			return next(ctx, req)
		}
	}
}
