// Package coreclient is booth-module-store's HTTP client for booth-core's existing
// module registry and install/uninstall API (contracts/core-platform-api.md,
// booth-core's own docs/decisions/0005). This repo never re-implements install/
// uninstall — it only ever calls this API with a chart reference it already resolved
// from its own catalog (ADR 0027).
package coreclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/projectbooth/booth-module-store/internal/catalog"
)

// Module is booth-core's GET /api/modules response shape (internal/api's moduleView in
// the booth-core repo) — trimmed to the fields this repo actually uses.
type Module struct {
	ID    string `json:"id"`
	Phase string `json:"phase"`
	// Namespace is the module's real install namespace (ADR 0060). Empty against an
	// older booth-core that doesn't serialize it yet — an additive, optional field,
	// not a hard version dependency.
	Namespace string `json:"namespace,omitempty"`
}

// Client calls booth-core's own API, authenticating as whichever caller's identity it
// was constructed for — every call reuses the caller's already-verified bearer token
// and active workspace rather than minting module-store's own service identity, since
// install/uninstall are owner-gated actions booth-core itself authorizes per-caller
// (internal/api's requireAdmin in the booth-core repo).
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTPClient: http.DefaultClient}
}

// ListModules calls GET /api/modules to learn what's currently installed.
func (c *Client) ListModules(ctx context.Context, bearerToken, workspace string) ([]Module, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/modules", bearerToken, workspace, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling booth-core GET /api/modules: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("booth-core GET /api/modules returned status %d", resp.StatusCode)
	}

	var modules []Module
	if err := json.NewDecoder(resp.Body).Decode(&modules); err != nil {
		return nil, fmt.Errorf("decoding booth-core /api/modules response: %w", err)
	}
	return modules, nil
}

// installRequest mirrors booth-core's own internal/api.installRequest — the exact body
// its POST /api/modules/{id}/install endpoint expects. A caller supplies exactly one
// of ChartRef (a raw string, e.g. "oci://host/path/chart-name" — a registry entry's
// chartRef, passed through unparsed per ADR 0028) or the structured Chart object.
type installRequest struct {
	Namespace    string         `json:"namespace"`
	Chart        *chartRefBody  `json:"chart,omitempty"` // pointer so omitempty actually omits it — encoding/json never omits a zero-value struct
	ChartRef     string         `json:"chartRef,omitempty"`
	ChartVersion string         `json:"chartVersion,omitempty"`
	Values       map[string]any `json:"values,omitempty"`
}

type chartRefBody struct {
	Path      string `json:"path,omitempty"`
	RepoURL   string `json:"repoUrl,omitempty"`
	ChartName string `json:"chartName,omitempty"`
	Version   string `json:"version,omitempty"`
}

// Install calls POST /api/modules/{id}/install with the given entry's chart reference
// — its raw ChartRef string if set (a registry entry, passed through unparsed per ADR
// 0028), otherwise its structured Chart (a bundled entry, authored directly in that
// form). Returns an error if neither is set (catalog.Entry.HasChart) — this repo never
// guesses at a chart location.
func (c *Client) Install(ctx context.Context, bearerToken, workspace, moduleID string, entry catalog.Entry, namespace string, values map[string]any) error {
	if !entry.HasChart() {
		return fmt.Errorf("module %q has no chart reference yet; cannot install", moduleID)
	}
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	reqBody := installRequest{Namespace: namespace, Values: values}
	if entry.ChartRef != "" {
		reqBody.ChartRef = entry.ChartRef
		reqBody.ChartVersion = entry.ChartVersion
	} else {
		reqBody.Chart = &chartRefBody{
			Path:      entry.Chart.Path,
			RepoURL:   entry.Chart.RepoURL,
			ChartName: entry.Chart.ChartName,
			Version:   entry.Chart.Version,
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encoding install request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/modules/"+moduleID+"/install", bearerToken, workspace, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling booth-core install for %q: %w", moduleID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("booth-core install for %q returned status %d", moduleID, resp.StatusCode)
	}
	return nil
}

// Uninstall calls DELETE /api/modules/{id}?namespace=....
func (c *Client) Uninstall(ctx context.Context, bearerToken, workspace, moduleID, namespace string) error {
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	req, err := c.newRequest(ctx, http.MethodDelete, "/api/modules/"+moduleID+"?namespace="+namespace, bearerToken, workspace, nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling booth-core uninstall for %q: %w", moduleID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("booth-core uninstall for %q returned status %d", moduleID, resp.StatusCode)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path, bearerToken, workspace string, body *bytes.Reader) (*http.Request, error) {
	var req *http.Request
	var err error
	if body == nil {
		req, err = http.NewRequestWithContext(ctx, method, c.BaseURL+path, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	}
	if err != nil {
		return nil, fmt.Errorf("building request for %s %s: %w", method, path, err)
	}

	req.Header.Set("Authorization", "Bearer "+bearerToken)
	// X-Workspace, not X-Booth-Workspace: booth-core's own auth.Middleware expects the
	// client-supplied header name (contracts/core-platform-api.md), distinct from the
	// X-Booth-Workspace name it forwards downstream to backing modules.
	req.Header.Set("X-Workspace", workspace)

	return req, nil
}
