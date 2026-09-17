package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBundled_Embedded(t *testing.T) {
	entries, err := LoadBundled("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected a non-empty bundled catalog (must work with zero network access, ADR 0027)")
	}

	for _, e := range entries {
		if e.ID == "" {
			t.Errorf("entry has empty id: %+v", e)
		}
		if e.DisplayName == "" {
			t.Errorf("entry %q has empty displayName", e.ID)
		}
		if e.Source.Kind != SourceBundled {
			t.Errorf("entry %q: Source.Kind = %q, want %q", e.ID, e.Source.Kind, SourceBundled)
		}
	}
}

func TestLoadBundled_RejectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundled.yaml")
	if err := os.WriteFile(path, []byte(`
- id: dup
  displayName: One
- id: dup
  displayName: Two
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadBundled(path); err == nil {
		t.Fatal("expected an error for duplicate ids, got nil")
	}
}

func TestLoadBundled_OverridePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundled.yaml")
	if err := os.WriteFile(path, []byte(`
- id: acme-private
  displayName: Acme Private Module
  category: custom
`), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadBundled(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "acme-private" {
		t.Fatalf("got %+v, want one entry with id acme-private", entries)
	}
}
