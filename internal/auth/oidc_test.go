package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
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
