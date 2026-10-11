package catalog

import (
	"fmt"
	"strings"
	"testing"
)

// parseChartRefLikeCore replicates booth-core's internal/lifecycle.ParseChartRef
// (ADR 0028) exactly — same oci://host/path/chart-name rule, same rejection of
// anything else. Duplicated rather than imported: this repo and booth-core are
// separate Go modules, and the whole point of ADR 0028 is that booth-core is the one
// place that actually resolves a chartRef at install time. This test exists so a
// chartRef that booth-core would reject at install time (a typo, a trailing slash
// nothing follows, a non-oci scheme) is caught here instead, against bundled data
// this repo ships and controls, not discovered live against a real install.
func parseChartRefLikeCore(chartRef string) error {
	if chartRef == "" {
		return fmt.Errorf("chartRef is empty")
	}
	if !strings.HasPrefix(chartRef, "oci://") {
		return fmt.Errorf("chartRef %q: only oci:// references are understood (ADR 0028)", chartRef)
	}
	trimmed := strings.TrimSuffix(chartRef, "/")
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastSlash < len("oci://") {
		return fmt.Errorf("chartRef %q: expected oci://host/path/chart-name", chartRef)
	}
	if trimmed[lastSlash+1:] == "" {
		return fmt.Errorf("chartRef %q: empty chart name after last '/'", chartRef)
	}
	return nil
}

// TestBundled_EveryNonEmptyChartRefParsesLikeCore is the ADR 0111 item 3 regression
// guard: every bundled entry's chartRef must be something booth-core's real
// ParseChartRef would accept, not just a plausible-looking string. A bundled entry
// with no chart yet (superset, metabase as of this seed) is explicitly allowed to have
// an empty chartRef — this test only checks entries that claim to have one.
func TestBundled_EveryNonEmptyChartRefParsesLikeCore(t *testing.T) {
	entries, err := LoadBundled("")
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, e := range entries {
		if e.ChartRef == "" {
			continue
		}
		checked++
		if err := parseChartRefLikeCore(e.ChartRef); err != nil {
			t.Errorf("entry %q: chartRef %q fails booth-core's ParseChartRef grammar: %v", e.ID, e.ChartRef, err)
		}
		if e.ChartVersion == "" {
			t.Errorf("entry %q: chartRef is set but chartVersion is empty", e.ID)
		}
	}

	if checked == 0 {
		t.Fatal("expected at least one bundled entry with a non-empty chartRef to actually check")
	}
}

// TestBundled_ChartRefMatchesPublishedRegistryLayout pins the exact host/path this
// repo assumes every module publishes its chart under (ADR 0111) — catches a typo'd
// org/path segment that parseChartRefLikeCore's grammar alone wouldn't catch (any
// "oci://host/path/name" passes that check regardless of host).
func TestBundled_ChartRefMatchesPublishedRegistryLayout(t *testing.T) {
	entries, err := LoadBundled("")
	if err != nil {
		t.Fatal(err)
	}

	const wantPrefix = "oci://ghcr.io/projectbooth/charts/"
	for _, e := range entries {
		if e.ChartRef == "" {
			continue
		}
		if !strings.HasPrefix(e.ChartRef, wantPrefix) {
			t.Errorf("entry %q: chartRef %q does not start with %q", e.ID, e.ChartRef, wantPrefix)
		}
	}
}

// TestBundled_LakehouseAndDatabasePresent is the ADR 0111 item 2 regression guard:
// both are real module-type MODULE_REGISTRY.md rows with a published chart and were
// simply missing from this file (a staleness gap, not a deliberate exclusion) until
// this pass.
func TestBundled_LakehouseAndDatabasePresent(t *testing.T) {
	entries, err := LoadBundled("")
	if err != nil {
		t.Fatal(err)
	}

	byID := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}

	for _, id := range []string{"lakehouse", "database"} {
		e, ok := byID[id]
		if !ok {
			t.Errorf("expected a bundled entry for %q", id)
			continue
		}
		if e.ChartRef == "" || e.ChartVersion == "" {
			t.Errorf("entry %q: expected a non-empty chartRef/chartVersion, got %+v", id, e)
		}
	}
}

// TestBundled_SupersetAndMetabaseHaveNoChartYet documents, rather than just omits, that
// these two are intentionally chart-less as of this seed (no published chart exists to
// point at — checked directly against the registry) — distinct from a bug where a
// chartRef was simply forgotten.
func TestBundled_SupersetAndMetabaseHaveNoChartYet(t *testing.T) {
	entries, err := LoadBundled("")
	if err != nil {
		t.Fatal(err)
	}

	byID := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}

	for _, id := range []string{"superset", "metabase"} {
		e, ok := byID[id]
		if !ok {
			t.Fatalf("expected a bundled entry for %q (listed even with no chart yet)", id)
		}
		if e.ChartRef != "" {
			t.Errorf("entry %q: expected empty chartRef (no chart published yet), got %q — update this test once one exists", id, e.ChartRef)
		}
		if e.HasChart() {
			t.Errorf("entry %q: HasChart() = true, want false (not installable yet)", id)
		}
	}
}
