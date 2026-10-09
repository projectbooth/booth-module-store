package auth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/projectbooth/booth-module-store/internal/config"
)

const testIssuer = "https://idp.example.com/realms/booth"

// newTestVerifier builds a Verifier around a static key set (no network discovery) and
// returns a function that signs tokens with the matching private key — real signature
// verification, real claim parsing, no live OIDC provider.
func newTestVerifier(t *testing.T, groupsClaim string) (*Verifier, func(claims map[string]any) string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v := &Verifier{
		idTokenVerifier: oidc.NewVerifier(testIssuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{SkipClientIDCheck: true}),
		groupsClaim:     groupsClaim,
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	sign := func(claims map[string]any) string {
		base := map[string]any{"iss": testIssuer, "sub": "user-1", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
		for k, val := range claims {
			base[k] = val
		}
		payload, _ := json.Marshal(base)
		obj, err := signer.Sign(payload)
		if err != nil {
			t.Fatal(err)
		}
		tok, err := obj.CompactSerialize()
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	return v, sign
}

func TestVerifier_ReadsGroupsClaim(t *testing.T) {
	v, sign := newTestVerifier(t, "groups")
	claims, err := v.Verify(context.Background(), sign(map[string]any{"groups": []string{"/workspaces/acme/owner", "/other"}}))
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("Subject = %q", claims.Subject)
	}
	if got := RoleInWorkspace(claims.Groups, "acme"); got != RoleOwner {
		t.Errorf("role from a real signed token = %q, want owner (groups = %v)", got, claims.Groups)
	}
}

// The claim name is deployment config (some IdPs don't call it "groups"), and must be
// honored — a token carrying the role only under a different claim name grants nothing.
func TestVerifier_HonorsConfiguredClaimName(t *testing.T) {
	v, sign := newTestVerifier(t, "roles")
	tok := sign(map[string]any{
		"roles":  []string{"/workspaces/acme/editor"},
		"groups": []string{"/workspaces/acme/owner"}, // decoy under the default name
	})
	claims, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	if got := RoleInWorkspace(claims.Groups, "acme"); got != RoleEditor {
		t.Errorf("role = %q, want editor from the configured %q claim, ignoring the default-named decoy", got, "roles")
	}
}

func TestVerifier_MissingOrMalformedGroupsFailClosed(t *testing.T) {
	v, sign := newTestVerifier(t, "groups")
	for name, extra := range map[string]map[string]any{
		"no groups claim":       {},
		"groups is a string":    {"groups": "/workspaces/acme/owner"},
		"groups is an object":   {"groups": map[string]any{"a": 1}},
		"groups contains a num": {"groups": []any{1, 2}},
	} {
		t.Run(name, func(t *testing.T) {
			claims, err := v.Verify(context.Background(), sign(extra))
			if err != nil {
				t.Fatalf("a genuine token with a bad groups claim is not an auth error, got %v", err)
			}
			if got := RoleInWorkspace(claims.Groups, "acme"); got != "" {
				t.Errorf("role = %q, want none (fail closed)", got)
			}
		})
	}
}

func TestVerifier_RejectsWrongIssuerAndExpiredTokens(t *testing.T) {
	v, sign := newTestVerifier(t, "groups")
	if _, err := v.Verify(context.Background(), sign(map[string]any{"iss": "https://evil.example.com"})); err == nil {
		t.Error("token from a different issuer must be rejected")
	}
	if _, err := v.Verify(context.Background(), sign(map[string]any{"exp": time.Now().Add(-time.Hour).Unix()})); err == nil {
		t.Error("expired token must be rejected")
	}
}

func TestVerifier_RejectsTokenSignedWithADifferentKey(t *testing.T) {
	v, _ := newTestVerifier(t, "groups")
	_, otherSign := newTestVerifier(t, "groups") // different keypair
	if _, err := v.Verify(context.Background(), otherSign(map[string]any{"groups": []string{"/workspaces/acme/owner"}})); err == nil {
		t.Error("a token signed by an unknown key must be rejected")
	}
}

// testJWKSOnlyServer stands up a real HTTP server that serves only a JWKS endpoint, no
// discovery document — exercising the ADR 0108 key-fetch-override path through the
// real NewVerifier, which must never touch discovery at all when JWKSURL is set.
type testJWKSOnlyServer struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	keyID  string
}

func newTestJWKSOnlyServer(t *testing.T) *testJWKSOnlyServer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	s := &testJWKSOnlyServer{key: key, keyID: "jwks-only-key-1"}

	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		jwks := jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: &s.key.PublicKey, KeyID: s.keyID, Algorithm: "RS256", Use: "sig"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		t.Error("discovery was fetched even though JWKSURL was set — the key-fetch override must bypass it entirely")
		w.WriteHeader(http.StatusNotFound)
	})

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

func (s *testJWKSOnlyServer) issueToken(t *testing.T, issuer string, claims map[string]any) string {
	t.Helper()

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       s.key,
	}, (&jose.SignerOptions{}).WithHeader("kid", s.keyID).WithType("JWT"))
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	base := map[string]any{
		"iss": issuer,
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	for k, v := range claims {
		base[k] = v
	}
	raw, err := jwt.Signed(signer).Claims(base).Serialize()
	if err != nil {
		t.Fatalf("serializing token: %v", err)
	}
	return raw
}

// This is ADR 0108's central claim: a verifying service can fetch signing keys from a
// plain-http, purely-in-cluster URL while the token's iss is an https URL this test
// process cannot even reach — proving the key fetch and the issuer check are genuinely
// decoupled, not just configured with different-looking strings that happen to agree.
func TestNewVerifier_JWKSURLOverride_KeysDecoupledFromUnreachableIssuer(t *testing.T) {
	jwks := newTestJWKSOnlyServer(t)
	const unreachableHTTPSIssuer = "https://booth.home.arpa.invalid/realms/booth"

	token := jwks.issueToken(t, unreachableHTTPSIssuer, map[string]any{
		"groups": []string{"/workspaces/acme/owner"},
	})

	verifier, err := NewVerifier(context.Background(), config.OIDCConfig{
		IssuerURL: unreachableHTTPSIssuer,
		JWKSURL:   jwks.server.URL + "/jwks",
		ClientID:  "test-client",
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	claims, err := verifier.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, want user-123", claims.Subject)
	}
	if got := RoleInWorkspace(claims.Groups, "acme"); got != RoleOwner {
		t.Errorf("role = %q, want owner (groups = %v)", got, claims.Groups)
	}
}

// iss is still validated exactly even when keys come from a separate URL: a token
// signed by the same key but claiming a different issuer must still be rejected.
func TestNewVerifier_JWKSURLOverride_StillValidatesIssuerExactly(t *testing.T) {
	jwks := newTestJWKSOnlyServer(t)

	token := jwks.issueToken(t, "https://wrong-issuer.invalid/realms/booth", nil)

	verifier, err := NewVerifier(context.Background(), config.OIDCConfig{
		IssuerURL: "https://booth.home.arpa.invalid/realms/booth",
		JWKSURL:   jwks.server.URL + "/jwks",
		ClientID:  "test-client",
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	if _, err := verifier.Verify(context.Background(), token); err == nil {
		t.Fatal("expected an issuer-mismatch error, got nil")
	}
}

func TestNewVerifier_RejectsJWKSURLWithoutIssuerURL(t *testing.T) {
	jwks := newTestJWKSOnlyServer(t)

	_, err := NewVerifier(context.Background(), config.OIDCConfig{
		JWKSURL:  jwks.server.URL + "/jwks",
		ClientID: "test-client",
	})
	if err == nil {
		t.Fatal("expected an error: JWKSURL set without IssuerURL")
	}
}

// ADR 0108 condition 2: the effective key URL and issuer are logged once at startup, so
// an operator can see which key source is actually in effect without reading config.
func TestNewVerifier_LogsIssuerAndKeySourceOnce(t *testing.T) {
	jwks := newTestJWKSOnlyServer(t)
	const issuer = "https://booth.home.arpa.invalid/realms/booth"

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	if _, err := NewVerifier(context.Background(), config.OIDCConfig{
		IssuerURL: issuer,
		JWKSURL:   jwks.server.URL + "/jwks",
		ClientID:  "test-client",
	}); err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	out := buf.String()
	if strings.Count(out, "oidc: verifying tokens") != 1 {
		t.Fatalf("expected exactly one startup log line, got: %q", out)
	}
	if !strings.Contains(out, issuer) {
		t.Errorf("log line does not mention the issuer: %q", out)
	}
	if !strings.Contains(out, jwks.server.URL+"/jwks") {
		t.Errorf("log line does not mention the effective key URL: %q", out)
	}
}

// Never logs a token — only the issuer and key source, both non-secret config values.
func TestNewVerifier_LogDoesNotContainTheToken(t *testing.T) {
	jwks := newTestJWKSOnlyServer(t)
	const issuer = "https://booth.home.arpa.invalid/realms/booth"
	token := jwks.issueToken(t, issuer, nil)

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	verifier, err := NewVerifier(context.Background(), config.OIDCConfig{
		IssuerURL: issuer,
		JWKSURL:   jwks.server.URL + "/jwks",
		ClientID:  "test-client",
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if _, err := verifier.Verify(context.Background(), token); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if strings.Contains(buf.String(), token) {
		t.Error("log output contains the raw token")
	}
}

// With JWKSURL unset, NewVerifier's real discovery path (not the static-keyset shortcut
// the rest of this file uses) still works end to end against a real discovery+JWKS
// server, and behaves identically to before this change.
func TestNewVerifier_DiscoveryPathUnchangedWhenJWKSURLUnset(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const keyID = "discovery-key-1"

	mux := http.NewServeMux()
	var issuerURL string
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                issuerURL,
			"jwks_uri":                              issuerURL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: keyID, Algorithm: "RS256", Use: "sig"}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	issuerURL = server.URL

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", keyID).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.Signed(signer).Claims(map[string]any{
		"iss": issuerURL, "sub": "user-123", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	}).Serialize()
	if err != nil {
		t.Fatal(err)
	}

	verifier, err := NewVerifier(context.Background(), config.OIDCConfig{IssuerURL: issuerURL, ClientID: "test-client"})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	claims, err := verifier.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, want user-123", claims.Subject)
	}
}
