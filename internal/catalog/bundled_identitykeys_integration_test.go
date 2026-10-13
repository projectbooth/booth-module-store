//go:build integration

package catalog

import (
	"encoding/json"
	"os/exec"
	"testing"

	"gopkg.in/yaml.v3"
)

// identityKeysExemptIDs lists bundled entries whose currently-pinned chart version does
// not yet carry the ADR 0113 booth.projectbooth.io/identity-keys annotation. A module
// installed from the Store without the annotation simply gets no shared identity
// settings filled in — not a failure, but worth a reminder in the log rather than a
// silent pass. Remove an id from this list once its chart release adds the annotation
// (booth-architecture/decisions/0113-core-supplies-the-shared-identity-settings-to-module-installs.md).
var identityKeysExemptIDs = map[string]bool{
	// booth-streamlit's identity-keys annotation shipped in its v0.1.1 release (PR #20);
	// it's no longer exempt. Every bundled, installable chart is now expected to carry
	// the annotation — add an id back here only if a module's chart genuinely has none
	// yet, same as streamlit did until its release landed.
}

// TestBundled_ChartRefsExistAndCarryIdentityKeys is the network-dependent half of the
// ADR 0111/0113 chartRef regression guard: for every bundled entry with a chartRef, the
// pinned version must actually exist at that OCI reference (a real-install failure mode
// found live before, see ADR 0111 item 6), and — new — the published chart must carry
// the identity-keys annotation, so a module installed through the Store's Install button
// actually comes up configured with core's shared identity settings (ADR 0113) rather
// than silently missing them. Requires network access to the real registry, so this
// runs only under the "integration" build tag, in .github/workflows/integration.yml —
// never in ci.yml's per-push/PR, network-free unit test run.
func TestBundled_ChartRefsExistAndCarryIdentityKeys(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed; this integration test runs in CI where it is (see .github/workflows/integration.yml)")
	}

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
		e := e
		t.Run(e.ID, func(t *testing.T) {
			out, err := exec.Command("helm", "show", "chart", e.ChartRef, "--version", e.ChartVersion).CombinedOutput()
			if err != nil {
				t.Fatalf("chartRef %s @ %s does not resolve on the registry: %v\n%s", e.ChartRef, e.ChartVersion, err, out)
			}

			var chart struct {
				Annotations map[string]string `yaml:"annotations"`
			}
			if err := yaml.Unmarshal(out, &chart); err != nil {
				t.Fatalf("parsing `helm show chart` output for %s @ %s: %v\n%s", e.ChartRef, e.ChartVersion, err, out)
			}

			raw, hasAnnotation := chart.Annotations["booth.projectbooth.io/identity-keys"]
			if identityKeysExemptIDs[e.ID] {
				if hasAnnotation {
					t.Logf("entry %q now carries the identity-keys annotation at %s @ %s — remove it from identityKeysExemptIDs (ADR 0113)", e.ID, e.ChartRef, e.ChartVersion)
				}
				return
			}
			if !hasAnnotation {
				t.Fatalf("entry %q (chart %s @ %s) has no booth.projectbooth.io/identity-keys annotation (ADR 0113) — a module installed from the Store would come up without core's shared identity settings", e.ID, e.ChartRef, e.ChartVersion)
			}
			var mapping map[string]string
			if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
				t.Fatalf("entry %q: identity-keys annotation is not valid JSON: %v\nvalue: %s", e.ID, err, raw)
			}
			if len(mapping) == 0 {
				t.Fatalf("entry %q: identity-keys annotation decoded to an empty object", e.ID)
			}
		})
	}

	if checked == 0 {
		t.Fatal("expected at least one bundled entry with a non-empty chartRef to actually check")
	}
}
