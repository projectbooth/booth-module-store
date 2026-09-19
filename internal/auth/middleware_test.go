package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeVerifier accepts any token and returns fixed claims, so the middleware's role
// logic can be tested without a live OIDC provider.
type fakeVerifier struct {
	claims *Claims
	err    error
}

func (f fakeVerifier) Verify(context.Context, string) (*Claims, error) { return f.claims, f.err }

// serve runs one request through Middleware and reports the status plus the identity a
// downstream handler saw (nil if the middleware rejected before reaching it).
func serve(t *testing.T, v TokenVerifier, headers map[string]string) (int, *Identity) {
	t.Helper()
	var seen *Identity
	h := Middleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := FromContext(r.Context())
		seen = &id
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	for k, val := range headers {
		req.Header.Set(k, val)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, seen
}

func viewerToken() fakeVerifier {
	return fakeVerifier{claims: &Claims{Subject: "u1", Groups: []string{"/workspaces/acme/viewer"}}}
}

func ownerToken() fakeVerifier {
	return fakeVerifier{claims: &Claims{Subject: "u1", Groups: []string{"/workspaces/acme/owner"}}}
}

func hdr(role string) map[string]string {
	h := map[string]string{"Authorization": "Bearer tok", HeaderBoothWorkspace: "acme"}
	if role != "" {
		h[HeaderBoothRole] = role
	}
	return h
}

// TestMiddleware_RejectsForgedRoleHeader is the ADR 0041 regression guard, the exact
// scenario booth-storage verified against a real Keycloak: a genuine viewer token sent
// alongside a forged "X-Booth-Role: owner" must not be honored.
func TestMiddleware_RejectsForgedRoleHeader(t *testing.T) {
	code, seen := serve(t, viewerToken(), hdr("owner"))
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a viewer token with a forged owner header", code)
	}
	if seen != nil {
		t.Fatal("downstream handler must not run when the forged header is rejected")
	}
}

func TestMiddleware_RejectsEditorClaimStrongerThanViewerToken(t *testing.T) {
	if code, _ := serve(t, viewerToken(), hdr("editor")); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
}

func TestMiddleware_RejectsUnrecognizedRoleHeader(t *testing.T) {
	if code, _ := serve(t, ownerToken(), hdr("superuser")); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a header value that isn't a role at all", code)
	}
}

func TestMiddleware_RoleComesFromTheToken(t *testing.T) {
	code, seen := serve(t, ownerToken(), hdr("owner"))
	if code != http.StatusOK || seen == nil || seen.Role != RoleOwner {
		t.Fatalf("status = %d, identity = %+v, want 200 with owner", code, seen)
	}
}

func TestMiddleware_AbsentRoleHeaderFallsBackToTokenRole(t *testing.T) {
	code, seen := serve(t, ownerToken(), hdr(""))
	if code != http.StatusOK || seen == nil || seen.Role != RoleOwner {
		t.Fatalf("status = %d, identity = %+v, want the token's owner role when no header is sent", code, seen)
	}
}

// A gateway may narrow a caller's role, never widen it.
func TestMiddleware_HeaderMayNarrowButNotWiden(t *testing.T) {
	code, seen := serve(t, ownerToken(), hdr("viewer"))
	if code != http.StatusOK || seen == nil || seen.Role != RoleViewer {
		t.Fatalf("status = %d, identity = %+v, want a weaker forwarded role to be honored", code, seen)
	}
}

func TestMiddleware_RejectsTokenWithNoRoleInWorkspace(t *testing.T) {
	v := fakeVerifier{claims: &Claims{Subject: "u1", Groups: []string{"/workspaces/other/owner"}}}
	if code, _ := serve(t, v, hdr("owner")); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: the token grants nothing in workspace acme", code)
	}
}

func TestMiddleware_RejectsTokenWithNoGroupsClaim(t *testing.T) {
	v := fakeVerifier{claims: &Claims{Subject: "u1"}}
	if code, _ := serve(t, v, hdr("owner")); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (fail closed)", code)
	}
}

func TestMiddleware_AuthenticationFailures(t *testing.T) {
	if code, _ := serve(t, ownerToken(), map[string]string{HeaderBoothWorkspace: "acme"}); code != http.StatusUnauthorized {
		t.Errorf("missing bearer token: status = %d, want 401", code)
	}
	bad := fakeVerifier{err: errors.New("expired")}
	if code, _ := serve(t, bad, hdr("owner")); code != http.StatusUnauthorized {
		t.Errorf("invalid token: status = %d, want 401", code)
	}
	if code, _ := serve(t, ownerToken(), map[string]string{"Authorization": "Bearer tok"}); code != http.StatusBadRequest {
		t.Errorf("missing workspace header: status = %d, want 400", code)
	}
}
