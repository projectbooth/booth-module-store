package catalog

// InstalledInfo is what booth-core's registry (GET /api/modules) can tell us about one
// installed module.
type InstalledInfo struct {
	Health string
	// Namespace is the module's real install namespace (ADR 0060), empty if
	// booth-core hasn't shipped that field yet — an older booth-core is not a hard
	// dependency, this repo just falls back to SuggestedNamespace's guess.
	Namespace string
}

// InstalledLookup answers "is module id installed, and what does booth-core's own
// registry say about it", per booth-core's own registry (coreclient.Module). Declared
// as an interface here so this package doesn't need to import the HTTP client just to
// merge.
type InstalledLookup interface {
	Lookup(id string) (info InstalledInfo, installed bool)
}

// Merge combines the bundled tier with zero or more registry tiers into one catalog
// view, then cross-references installed status. Entries are not deduplicated across
// tiers — a module listed both in the bundled catalog and an external registry appears
// twice, each labeled with its own source (ADR 0027's "showing each entry's source...
// so a user isn't confused about provenance"), since collapsing them would have to
// guess which listing is authoritative.
func Merge(bundled []Entry, registryTiers [][]Entry, installed InstalledLookup) []Entry {
	total := len(bundled)
	for _, tier := range registryTiers {
		total += len(tier)
	}

	out := make([]Entry, 0, total)
	out = append(out, bundled...)
	for _, tier := range registryTiers {
		out = append(out, tier...)
	}

	for i := range out {
		info, isInstalled := installed.Lookup(out[i].ID)
		if isInstalled {
			out[i].Status = InstallStatus{State: Installed, Health: info.Health}
			out[i].Namespace = info.Namespace
		} else {
			out[i].Status = InstallStatus{State: NotInstalled}
		}
	}

	return out
}
