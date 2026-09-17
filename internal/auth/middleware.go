package auth

import (
	"context"
	"net/http"
	"strings"
)

// Header names the gateway forwards to a backing module, per ADR 0025 and
// core-platform-api.md. This module trusts these only because it also independently
// re-verifies the forwarded JWT below (defense in depth) — never on their own.
const (
	HeaderBoothWorkspace = "X-Booth-Workspace"
	HeaderBoothRole      = "X-Booth-Role"
)

type contextKey string

const identityContextKey contextKey = "booth-module-store-identity"

// Identity is the caller identity attached to a request's context by Middleware.
type Identity struct {
	Claims    *Claims
	Workspace string
	Role      string
	// RawToken is the original bearer token, kept so this module can forward it
	// unchanged when it calls back into booth-core's own API (module-to-module
	// synchronous calls go through core, per ADR 0007 — reusing the caller's already-
	// verified token is simpler and no less trusted than minting a new one).
	RawToken string
}

func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityContextKey).(Identity)
	return id, ok
}

// Middleware verifies the request's bearer token and reads the gateway-forwarded
// workspace/role headers, attaching the result to the request context.
func Middleware(verifier *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			claims, err := verifier.Verify(r.Context(), token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			workspace := r.Header.Get(HeaderBoothWorkspace)
			if workspace == "" {
				http.Error(w, "missing "+HeaderBoothWorkspace+" header", http.StatusBadRequest)
				return
			}

			identity := Identity{
				Claims:    claims,
				Workspace: workspace,
				Role:      r.Header.Get(HeaderBoothRole),
				RawToken:  token,
			}
			ctx := context.WithValue(r.Context(), identityContextKey, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithIdentityForTesting attaches an Identity the same way Middleware does, for tests
// exercising handlers downstream of it without a real OIDC provider.
func WithIdentityForTesting(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}
