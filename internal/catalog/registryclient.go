package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// registryModule is the wire shape one entry of a GET <registry-url>/v0/modules
// response takes, per contracts/module-registry-protocol.md.
type registryModule struct {
	ID              string          `json:"id"`
	DisplayName     string          `json:"displayName"`
	Icon            string          `json:"icon"`
	Description     string          `json:"description"`
	Category        string          `json:"category"`
	ChartRef        string          `json:"chartRef"`
	ChartVersion    string          `json:"chartVersion"`
	ManifestPreview ManifestPreview `json:"manifestPreview"`
}

// RegistryClient fetches a single external registry's module listing.
type RegistryClient struct {
	HTTPClient *http.Client
}

func NewRegistryClient() *RegistryClient {
	return &RegistryClient{HTTPClient: &http.Client{Timeout: 10 * time.Second}}
}

// Fetch calls GET <registryURL>/v0/modules and returns its listing translated into
// catalog Entry values, each labeled with this registry as its Source (ADR 0027).
//
// An entry whose chartRef this repo's interim translation (chartref.go) can't parse is
// kept in the result — a user should still see it exists — but with an empty Chart, so
// the UI can show it as present-but-not-installable rather than silently dropping it.
func (c *RegistryClient) Fetch(ctx context.Context, registryURL string) ([]Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL+"/v0/modules", nil)
	if err != nil {
		return nil, fmt.Errorf("building request for registry %s: %w", registryURL, err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling registry %s: %w", registryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry %s returned status %d", registryURL, resp.StatusCode)
	}

	var raw []registryModule
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding registry %s response: %w", registryURL, err)
	}

	entries := make([]Entry, 0, len(raw))
	for _, m := range raw {
		if m.ID == "" {
			continue
		}
		entry := Entry{
			ID:              m.ID,
			DisplayName:     m.DisplayName,
			Icon:            m.Icon,
			Description:     m.Description,
			Category:        m.Category,
			ManifestPreview: m.ManifestPreview,
			Source:          Source{Kind: SourceRegistry, Name: registryURL},
		}

		if chart, err := ParseRegistryChartRef(m.ChartRef, m.ChartVersion); err == nil {
			entry.Chart = chart
		}
		// A chartRef this translation can't parse is not treated as fatal for the
		// whole registry fetch — the entry is still listed, just not installable
		// (Chart stays zero) until docs/decisions/0001 is resolved.

		entries = append(entries, entry)
	}

	return entries, nil
}
