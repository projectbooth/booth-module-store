package config

import (
	"net/url"
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BOOTH_OIDC_ISSUER_URL", "https://idp.example.com/realms/booth")
	t.Setenv("BOOTH_OIDC_CLIENT_ID", "booth-module-store")
}

// The default must be namespace-qualified: a bare "http://booth-core" only resolves from
// inside booth-core's own namespace, and this module is installed into its own.
func TestLoad_CoreBaseURLDefaultIsNamespaceQualified(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("BOOTH_CORE_BASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(cfg.CoreBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(u.Hostname(), ".") < 2 || u.Port() == "" {
		t.Errorf("CoreBaseURL default %q must be <service>.<namespace>.svc with an explicit port", cfg.CoreBaseURL)
	}
	if cfg.CoreBaseURL != DefaultCoreBaseURL {
		t.Errorf("CoreBaseURL = %q, want DefaultCoreBaseURL %q", cfg.CoreBaseURL, DefaultCoreBaseURL)
	}
}

func TestLoad_CoreBaseURLOverride(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("BOOTH_CORE_BASE_URL", "http://core.platform.svc:9090")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CoreBaseURL != "http://core.platform.svc:9090" {
		t.Errorf("CoreBaseURL = %q, want the env override", cfg.CoreBaseURL)
	}
}

func TestLoad_RequiresOIDCConfig(t *testing.T) {
	t.Setenv("BOOTH_OIDC_ISSUER_URL", "")
	t.Setenv("BOOTH_OIDC_CLIENT_ID", "")
	if _, err := Load(); err == nil {
		t.Error("expected an error when the OIDC issuer/client are unset")
	}
}

// JWKSURL defaults to empty — unchanged discovery-based verification (ADR 0108).
func TestLoad_JWKSURLDefaultsEmpty(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("BOOTH_OIDC_JWKS_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDC.JWKSURL != "" {
		t.Errorf("OIDC.JWKSURL = %q, want empty by default", cfg.OIDC.JWKSURL)
	}
}

func TestLoad_JWKSURLOverride(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("BOOTH_OIDC_JWKS_URL", "http://keycloak.booth-system.svc:8080/realms/booth/protocol/openid-connect/certs")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDC.JWKSURL != "http://keycloak.booth-system.svc:8080/realms/booth/protocol/openid-connect/certs" {
		t.Errorf("OIDC.JWKSURL = %q, want the env override", cfg.OIDC.JWKSURL)
	}
}

// ADR 0108: setting the key-fetch override without an issuer is a startup error.
func TestLoad_JWKSURLWithoutIssuerURLIsAnError(t *testing.T) {
	t.Setenv("BOOTH_OIDC_ISSUER_URL", "")
	t.Setenv("BOOTH_OIDC_CLIENT_ID", "booth-module-store")
	t.Setenv("BOOTH_OIDC_JWKS_URL", "http://keycloak.booth-system.svc:8080/realms/booth/protocol/openid-connect/certs")

	if _, err := Load(); err == nil {
		t.Error("expected an error: BOOTH_OIDC_JWKS_URL set without BOOTH_OIDC_ISSUER_URL")
	}
}
