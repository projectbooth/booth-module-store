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
// chartRef/chartVersion are passed through verbatim, unparsed (ADR 0028) — booth-core's
// install API now accepts them directly and is the one place that parsing lives; this
// repo doesn't interpret or validate the chart itself beyond passing it through, per
// contracts/module-registry-protocol.md's original intent.
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
		entries = append(entries, Entry{
			ID:              m.ID,
			DisplayName:     m.DisplayName,
			Icon:            m.Icon,
			Description:     m.Description,
			Category:        m.Category,
			ManifestPreview: m.ManifestPreview,
			ChartRef:        m.ChartRef,
			ChartVersion:    m.ChartVersion,
			Source:          Source{Kind: SourceRegistry, Name: registryURL},
		})
	}

	return entries, nil
}
