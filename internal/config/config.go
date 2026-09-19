// Package config loads booth-module-store's runtime configuration from environment
// variables. Every value maps 1:1 to a Helm chart value/env var, mirroring booth-core's
// own internal/config — there is no config file format of our own to version.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config is booth-module-store's full runtime configuration.
type Config struct {
	// HTTPAddr is the address the HTTP server listens on.
	HTTPAddr string

	// CoreBaseURL is where booth-core's own API lives (GET /api/modules, POST/DELETE
	// /api/modules/{id}) — this module is a caller of that API, never a re-implementer
	// (ADR 0027, booth-core decision 0005). In-cluster, this is core's Service DNS name.
	CoreBaseURL string

	// OIDC is the identity-provider configuration used to independently re-verify a
	// forwarded bearer token (core-platform-api.md's defense-in-depth requirement).
	// Deliberately the same shape as booth-core's own OIDCConfig — every module
	// verifies against the same provider.
	OIDC OIDCConfig

	// BundledCatalogPath, if set, overrides the embedded bundled catalog with a file on
	// disk — how a deployment extends the bundled list with its own private/custom
	// modules without an external registry (ADR 0027's "extend via own config").
	BundledCatalogPath string

	// RegistryURLs is the deployment-configured list of external module registry
	// endpoints (contracts/module-registry-protocol.md). Empty by default — the
	// external-registry tier is off/unconfigured until a deployment opts in (ADR 0027),
	// never a hardcoded guess at an official registry's address.
	RegistryURLs []string
}

// OIDCConfig mirrors booth-core's internal/config.OIDCConfig — see that package's doc
// comment for field-by-field reasoning. Kept as a separate type (not imported from
// booth-core) so this repo has no build dependency on booth-core's module.
type OIDCConfig struct {
	IssuerURL       string
	ClientID        string
	RequireAudience bool

	// GroupsClaim names the token claim carrying workspace memberships, read to
	// re-derive the caller's role from the token itself (ADR 0041). Configurable because
	// not every OIDC provider calls it "groups"; must match booth-core's own
	// BOOTH_OIDC_GROUPS_CLAIM. Empty means DefaultGroupsClaim.
	GroupsClaim string
}

// DefaultGroupsClaim matches booth-core's default (ADR 0025).
const DefaultGroupsClaim = "groups"

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:           getEnv("BOOTH_HTTP_ADDR", ":8080"),
		CoreBaseURL:        getEnv("BOOTH_CORE_BASE_URL", "http://booth-core"),
		BundledCatalogPath: os.Getenv("BOOTH_MODULE_STORE_BUNDLED_CATALOG_PATH"),
		RegistryURLs:       splitNonEmpty(os.Getenv("BOOTH_MODULE_STORE_REGISTRIES")),
		OIDC: OIDCConfig{
			IssuerURL:       os.Getenv("BOOTH_OIDC_ISSUER_URL"),
			ClientID:        os.Getenv("BOOTH_OIDC_CLIENT_ID"),
			RequireAudience: os.Getenv("BOOTH_OIDC_REQUIRE_AUDIENCE") == "true",
			GroupsClaim:     getEnv("BOOTH_OIDC_GROUPS_CLAIM", DefaultGroupsClaim),
		},
	}

	if cfg.OIDC.IssuerURL == "" {
		return Config{}, fmt.Errorf("BOOTH_OIDC_ISSUER_URL is required")
	}
	if cfg.OIDC.ClientID == "" {
		return Config{}, fmt.Errorf("BOOTH_OIDC_CLIENT_ID is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitNonEmpty(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
