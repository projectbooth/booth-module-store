package auth

import (
	"context"
	"log"
	"net/http"
	"strings"
)

// Header names the gateway forwards to a backing module, per ADR 0025 and
// core-platform-api.md. X-Booth-Workspace names which workspace the request is for;
// X-Booth-Role is only ever a convenience — see Middleware for why it is never
// sufficient on its own (ADR 0041).
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
	// Role is derived from the verified token's groups claim for Workspace (ADR 0041),
	// never copied from the forwarded header.
	Role Role
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

// Middleware verifies the request's bearer token, then derives the caller's role in the
// requested workspace from that token's own groups claim (ADR 0041) and attaches the
// result to the request context.
//
// Verifying the token proves who the caller is, not that they hold whatever role a
// header claims: anyone able to reach this pod without going through booth-core's
// gateway could otherwise pair a valid viewer token with a forged "X-Booth-Role:
// owner". So the forwarded role is cross-checked against the token:
//   - the token grants no role in the workspace: 403.
//   - the header claims a role stronger than the token grants (or isn't a recognized
//     role at all): 403. A legitimate gateway request can never trip this, since the
//     gateway derives the header from this same token.
//   - otherwise the effective role is the token's, or the header's if it is weaker (a
//     gateway may narrow, never widen). An absent header means "use the token's".
func Middleware(verifier TokenVerifier) func(http.Handler) http.Handler {
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

			granted := RoleInWorkspace(claims.Groups, workspace)
			if granted == "" {
				http.Error(w, "your token grants no role in this workspace", http.StatusForbidden)
				return
			}

			role := granted
			if forwarded := Role(r.Header.Get(HeaderBoothRole)); forwarded != "" {
				if rank(forwarded) == 0 || rank(forwarded) > rank(granted) {
					log.Printf("auth: rejecting request: forwarded role %q is not backed by the token (token-derived role %q) for sub=%s workspace=%s", forwarded, granted, claims.Subject, workspace)
					http.Error(w, "forwarded role is not granted by your token", http.StatusForbidden)
					return
				}
				role = forwarded
			}

			identity := Identity{Claims: claims, Workspace: workspace, Role: role, RawToken: token}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityContextKey, identity)))
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
