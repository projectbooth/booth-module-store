// Package auth implements booth-module-store's side of the defense-in-depth
// requirement in contracts/core-platform-api.md: "a module must independently verify
// the identity core forwards it rather than trusting the network path implicitly."
//
// That covers both authentication (the bearer token's signature, issuer and expiry,
// verified here) and — per ADR 0041 — authorization: the workspace role is re-derived
// from the verified token's own groups claim (role.go), never taken on trust from the
// gateway-forwarded X-Booth-Role header, which anyone reaching this pod directly could
// forge alongside a valid low-privilege token.
package auth

import (
	"context"
	"encoding/json"
	"fmt"

	oidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/projectbooth/booth-module-store/internal/config"
)

// Claims is the subset of a verified token this module cares about.
type Claims struct {
	Subject string
	// Groups is the token's workspace-membership claim (ADR 0025), e.g.
	// "/workspaces/acme/owner". Empty if the token carries none.
	Groups []string
}

// TokenVerifier verifies a raw bearer token. *Verifier is the production
// implementation; the interface exists so the HTTP middleware can be tested without a
// live OIDC provider.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (*Claims, error)
}

// Verifier verifies bearer tokens against the same OIDC provider booth-core is
// configured against (signature via JWKS, issuer, expiry, and, per deployment policy,
// audience), and reads the configured groups claim.
type Verifier struct {
	idTokenVerifier *oidc.IDTokenVerifier
	groupsClaim     string
}

func NewVerifier(ctx context.Context, cfg config.OIDCConfig) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery against %s: %w", cfg.IssuerURL, err)
	}

	verifierCfg := &oidc.Config{
		SkipClientIDCheck: !cfg.RequireAudience,
		ClientID:          cfg.ClientID,
	}

	claim := cfg.GroupsClaim
	if claim == "" {
		claim = config.DefaultGroupsClaim
	}

	return &Verifier{idTokenVerifier: provider.Verifier(verifierCfg), groupsClaim: claim}, nil
}

func (v *Verifier) Verify(ctx context.Context, rawToken string) (*Claims, error) {
	idToken, err := v.idTokenVerifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}
	if idToken.Subject == "" {
		return nil, fmt.Errorf("token missing subject")
	}

	var raw map[string]json.RawMessage
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("reading token claims: %w", err)
	}
	var groups []string
	if g, ok := raw[v.groupsClaim]; ok {
		// A claim of the wrong shape is treated as "no groups" (fail closed), not an
		// error: the token is genuine, it simply grants no workspace role.
		_ = json.Unmarshal(g, &groups)
	}

	return &Claims{Subject: idToken.Subject, Groups: groups}, nil
}
