package catalog

import "testing"

type fakeInstalledLookup map[string]InstalledInfo // absent key means not installed

func (f fakeInstalledLookup) Lookup(id string) (InstalledInfo, bool) {
	info, ok := f[id]
	return info, ok
}

func TestMerge_LabelsSourceAndStatus(t *testing.T) {
	bundled := []Entry{{ID: "storage", Source: Source{Kind: SourceBundled}}}
	registryTiers := [][]Entry{
		{{ID: "acme-forecast", Source: Source{Kind: SourceRegistry, Name: "https://registry.example.com"}}},
	}
	installed := fakeInstalledLookup{"storage": {Health: "Healthy", Namespace: "booth-storage"}}

	out := Merge(bundled, registryTiers, installed)
	if len(out) != 2 {
		t.Fatalf("got %d entries, want 2", len(out))
	}

	byID := map[string]Entry{}
	for _, e := range out {
		byID[e.ID] = e
	}

	storage := byID["storage"]
	if storage.Status.State != Installed || storage.Status.Health != "Healthy" {
		t.Errorf("storage status = %+v, want installed/Healthy", storage.Status)
	}
	if storage.Namespace != "booth-storage" {
		t.Errorf("storage.Namespace = %q, want the real namespace from booth-core's registry (ADR 0060)", storage.Namespace)
	}

	forecast := byID["acme-forecast"]
	if forecast.Status.State != NotInstalled {
		t.Errorf("acme-forecast status = %+v, want not_installed", forecast.Status)
	}
	if forecast.Namespace != "" {
		t.Errorf("acme-forecast.Namespace = %q, want empty for a not-installed entry", forecast.Namespace)
	}
	if forecast.Source.Kind != SourceRegistry || forecast.Source.Name != "https://registry.example.com" {
		t.Errorf("acme-forecast source = %+v, want labeled registry", forecast.Source)
	}
}

// TestMerge_TolerantOfMissingNamespace is the case where booth-core hasn't shipped ADR
// 0060 yet (or a real BoothModule's namespace is otherwise unavailable): installed
// status must still populate, just without a real Namespace to offer.
func TestMerge_TolerantOfMissingNamespace(t *testing.T) {
	bundled := []Entry{{ID: "storage", Source: Source{Kind: SourceBundled}}}
	installed := fakeInstalledLookup{"storage": {Health: "Healthy"}} // no Namespace

	out := Merge(bundled, nil, installed)
	if out[0].Status.State != Installed {
		t.Fatalf("status = %+v, want installed", out[0].Status)
	}
	if out[0].Namespace != "" {
		t.Errorf("Namespace = %q, want empty when booth-core didn't supply one", out[0].Namespace)
	}
}

func TestMerge_DoesNotDedupeAcrossSources(t *testing.T) {
	bundled := []Entry{{ID: "storage", Source: Source{Kind: SourceBundled}}}
	registryTiers := [][]Entry{
		{{ID: "storage", Source: Source{Kind: SourceRegistry, Name: "https://registry.example.com"}}},
	}

	out := Merge(bundled, registryTiers, fakeInstalledLookup{})
	if len(out) != 2 {
		t.Fatalf("got %d entries, want 2 (same id from two sources should both appear)", len(out))
	}
}
