// Package catalog implements booth-module-store's two-tier catalog (ADR 0027): a
// bundled, offline-capable list of every module in this project, merged with whatever
// zero or more configured external registries return
// (contracts/module-registry-protocol.md), cross-referenced against booth-core's
// installed-module registry for status.
package catalog

// ChartRef is the structured chart location booth-core's install API actually expects
// (see coreclient.InstallRequest) — either a local path or a repo-hosted chart resolved
// by RepoURL+ChartName+Version, the same shape `helm install --repo ...` uses.
//
// This is deliberately NOT the same shape as module-registry-protocol.md's single
// `chartRef` string field. See chartref.go's doc comment for why that's a real,
// flagged-back gap rather than a design choice made here.
type ChartRef struct {
	Path      string `yaml:"path,omitempty" json:"path,omitempty"`
	RepoURL   string `yaml:"repoUrl,omitempty" json:"repoUrl,omitempty"`
	ChartName string `yaml:"chartName,omitempty" json:"chartName,omitempty"`
	Version   string `yaml:"version,omitempty" json:"version,omitempty"`
}

// IsZero reports whether no chart location has been filled in at all — the expected
// state for a bundled entry whose module hasn't published a chart yet.
func (c ChartRef) IsZero() bool {
	return c.Path == "" && c.RepoURL == "" && c.ChartName == ""
}

// ManifestPreview is a best-effort subset of contracts/module-manifest.md, shown before
// install. Per module-registry-protocol.md, this is informational only — the
// authoritative manifest is whatever the module's own BoothModule CRD declares once
// actually installed; booth-core never reads this preview.
type ManifestPreview struct {
	HasOwnUI          bool   `yaml:"hasOwnUi,omitempty" json:"hasOwnUi,omitempty"`
	UIIntegrationMode string `yaml:"uiIntegrationMode,omitempty" json:"uiIntegrationMode,omitempty"`
	NavGroup          string `yaml:"navGroup,omitempty" json:"navGroup,omitempty"`
}

// SourceKind identifies which tier of the two-tier catalog an Entry came from (ADR
// 0027's "each entry labeled with its source... so a user isn't confused about
// provenance").
type SourceKind string

const (
	SourceBundled  SourceKind = "bundled"
	SourceRegistry SourceKind = "registry"
)

// Source records exactly where a catalog entry came from.
type Source struct {
	Kind SourceKind `json:"kind"`
	// Name identifies the specific registry when Kind == SourceRegistry (its
	// configured URL, since v0 has no separate display-name concept for a registry
	// connection). Empty when Kind == SourceBundled.
	Name string `json:"name,omitempty"`
}

// InstallState is what booth-core's own registry can actually tell us (see
// coreclient.Module) — no more, no less. booth-core has no desired-state store (its own
// decision 0005): there is no durable "installing" or "failed" phase to query. A
// synchronous install/uninstall call's own success/failure IS the result; "in progress"
// is a transient client-side UI state around that one HTTP call, not something this
// enum needs to represent.
type InstallState string

const (
	NotInstalled InstallState = "not_installed"
	Installed    InstallState = "installed"
)

// InstallStatus is an Entry's cross-referenced status against booth-core's registry.
type InstallStatus struct {
	State InstallState `json:"state"`
	// Health is booth-core's last-observed BoothModule health phase (Healthy/
	// Unhealthy/Unreachable/Unknown), only meaningful when State == Installed.
	Health string `json:"health,omitempty"`
}

// Entry is one catalog listing, after merging bundled + registry tiers and
// cross-referencing installed status.
type Entry struct {
	ID              string          `yaml:"id" json:"id"`
	DisplayName     string          `yaml:"displayName" json:"displayName"`
	Icon            string          `yaml:"icon,omitempty" json:"icon,omitempty"`
	Description     string          `yaml:"description,omitempty" json:"description,omitempty"`
	Category        string          `yaml:"category,omitempty" json:"category,omitempty"`
	Chart           ChartRef        `yaml:"chart,omitempty" json:"chart,omitempty"`
	ManifestPreview ManifestPreview `yaml:"manifestPreview,omitempty" json:"manifestPreview,omitempty"`

	Source Source        `yaml:"-" json:"source"`
	Status InstallStatus `yaml:"-" json:"status"`

	// SuggestedNamespace is a pre-fill hint for an install/uninstall confirmation UI
	// only (ADR 0029) — set by internal/api when building a catalog response, never
	// applied as a silent default for the actual mutating call.
	SuggestedNamespace string `yaml:"-" json:"suggestedNamespace,omitempty"`
}
