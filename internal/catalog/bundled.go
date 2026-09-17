package catalog

import (
	_ "embed"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

//go:embed bundled.yaml
var embeddedBundled []byte

// LoadBundled returns the bundled catalog tier (ADR 0027): the embedded seed data by
// default, or the file at path if one is given — how a deployment extends the bundled
// list with its own private/custom modules without an external registry, per ADR
// 0027's "a deployment can extend the bundled list via its own config" consequence.
// Works with zero network access either way.
func LoadBundled(path string) ([]Entry, error) {
	data := embeddedBundled
	if path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading bundled catalog override %s: %w", path, err)
		}
	}

	var entries []Entry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing bundled catalog: %w", err)
	}

	seen := make(map[string]bool, len(entries))
	for i := range entries {
		e := &entries[i]
		if e.ID == "" {
			return nil, fmt.Errorf("bundled catalog entry at index %d is missing id", i)
		}
		if seen[e.ID] {
			return nil, fmt.Errorf("bundled catalog has duplicate id %q", e.ID)
		}
		seen[e.ID] = true
		e.Source = Source{Kind: SourceBundled}
	}

	return entries, nil
}
