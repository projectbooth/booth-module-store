package catalog

import "testing"

type fakeInstalledLookup map[string]string // id -> health; absent key means not installed

func (f fakeInstalledLookup) Lookup(id string) (string, bool) {
	health, ok := f[id]
	return health, ok
}

func TestMerge_LabelsSourceAndStatus(t *testing.T) {
	bundled := []Entry{{ID: "storage", Source: Source{Kind: SourceBundled}}}
	registryTiers := [][]Entry{
		{{ID: "acme-forecast", Source: Source{Kind: SourceRegistry, Name: "https://registry.example.com"}}},
	}
	installed := fakeInstalledLookup{"storage": "Healthy"}

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

	forecast := byID["acme-forecast"]
	if forecast.Status.State != NotInstalled {
		t.Errorf("acme-forecast status = %+v, want not_installed", forecast.Status)
	}
	if forecast.Source.Kind != SourceRegistry || forecast.Source.Name != "https://registry.example.com" {
		t.Errorf("acme-forecast source = %+v, want labeled registry", forecast.Source)
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
