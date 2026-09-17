// Package auth implements booth-module-store's side of the defense-in-depth
// requirement in contracts/core-platform-api.md: "a module must independently verify
// the identity core forwards it rather than trusting the network path implicitly."
//
// Unlike booth-core, this package does not derive workspace/role membership from a
// groups claim itself — booth-core's gateway already resolved that and forwards it as
// the X-Booth-Workspace/X-Booth-Role headers (ADR 0025), which this package trusts once
// the request's bearer token has independently verified. This mirrors the same trust
// boundary every other native-mode module sits behind.
package auth

import (
	"context"
	"fmt"

	oidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/projectbooth/booth-module-store/internal/config"
)

// Claims is the subset of a verified token this module cares about.
type Claims struct {
	Subject string
}

// Verifier verifies bearer tokens against the same OIDC provider booth-core is
// configured against (signature via JWKS, issuer, expiry, and, per deployment policy,
// audience).
type Verifier struct {
	idTokenVerifier *oidc.IDTokenVerifier
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

	return &Verifier{idTokenVerifier: provider.Verifier(verifierCfg)}, nil
}

func (v *Verifier) Verify(ctx context.Context, rawToken string) (*Claims, error) {
	idToken, err := v.idTokenVerifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}
	if idToken.Subject == "" {
		return nil, fmt.Errorf("token missing subject")
	}
	return &Claims{Subject: idToken.Subject}, nil
}
