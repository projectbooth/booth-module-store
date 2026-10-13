//go:build integration

package catalog

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Render-guard tuning (ARCHITECTURE.md item 68): the Install button installs a module
// through booth-core's install API with no hand-set values at all — core supplies only
// the shared identity settings a chart's own booth.projectbooth.io/identity-keys
// annotation maps (ADR 0113/amendment 1), nothing else. booth-catalog 0.1.1 could not be
// installed that way at all (it needed a hand-made database Secret its manifest never
// declared) and nothing in CI would have caught it — this is that catch, generalized to
// the whole bundled fleet, run against a kube-version/capability set offline helm
// template can't otherwise see.
const (
	renderGuardKubeVersion = "1.36.4"
	renderGuardAPIVersions = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"
)

// stubIdentityValues maps each ADR 0113 shared identity-setting name to an obviously
// fake stand-in for what core would really supply — RFC 2606 .invalid domains and
// clearly-labeled ids, never a real issuer, credential or address. Sufficient only to
// satisfy a chart's own `required` checks the same way core's real install-time values
// would, nothing more.
var stubIdentityValues = map[string]string{
	"issuerUrl":         "https://stub-issuer.example.invalid/realms/booth",
	"clientId":          "stub-client-id",
	"jwksUrl":           "https://stub-jwks.example.invalid/protocol/openid-connect/certs",
	"workloadIssuerUrl": "https://stub-workload-issuer.example.invalid",
	"groupsClaim":       "groups",
	"iframeIssuerUrl":   "https://stub-iframe-issuer.example.invalid",
	"coreUrl":           "http://stub-core.example.invalid:8080",
}

// renderGuardExemptIDs lists bundled entries exempted from this check for a specific,
// user-approved reason — empty until the user approves one; a failure here is reported,
// never silently exempted (ARCHITECTURE item 68, step 3).
var renderGuardExemptIDs = map[string]bool{}

// chartAnnotations reads a published chart's Chart.yaml annotations off the real
// registry, the same way TestBundled_ChartRefsExistAndCarryIdentityKeys does.
func chartAnnotations(chartRef, version string) (map[string]string, error) {
	out, err := exec.Command("helm", "show", "chart", chartRef, "--version", version).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	var chart struct {
		Annotations map[string]string `yaml:"annotations"`
	}
	if err := yaml.Unmarshal(out, &chart); err != nil {
		return nil, fmt.Errorf("parsing Chart.yaml: %w", err)
	}
	return chart.Annotations, nil
}

// TestBundled_RendersWithOnlyCoreSuppliedValues is the ARCHITECTURE item 68 render
// guard: for every chart the Store bundles (discovered from the same LoadBundled list
// the Store itself serves — a chart added later needs nothing registered here to be
// covered), it reads the chart's own identity-keys annotation, builds stub values for
// exactly the settings that annotation names (a chart with no annotation gets no
// values at all, per the contract: "a chart with no annotation gets no merge"), then
// renders the chart exactly as a Store-triggered install would — no other values, and a
// kube-version/capability set offline `helm template` can't otherwise see
// (admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy, kube 1.36.4). A chart
// whose manifest expects a value core never supplies (a hand-made Secret, a
// provisioning flag nothing declares) fails here with the exact Helm error, instead of
// failing silently for the first real user who clicks Install.
func TestBundled_RendersWithOnlyCoreSuppliedValues(t *testing.T) {
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
			continue // no published chart yet (superset, metabase as of this seed) — nothing to render
		}
		if renderGuardExemptIDs[e.ID] {
			continue
		}
		checked++
		e := e
		t.Run(e.ID, func(t *testing.T) {
			annotations, err := chartAnnotations(e.ChartRef, e.ChartVersion)
			if err != nil {
				t.Fatalf("reading Chart.yaml for %s @ %s: %v", e.ChartRef, e.ChartVersion, err)
			}

			var mapping map[string]string
			if raw, ok := annotations["booth.projectbooth.io/identity-keys"]; ok {
				if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
					t.Fatalf("entry %q: identity-keys annotation is not valid JSON: %v\nvalue: %s", e.ID, err, raw)
				}
			}
			// No annotation at all (mapping stays nil) is valid and means core supplies
			// nothing for this chart ("a chart with no annotation gets no merge") — this
			// chart is rendered below with zero values, exactly like a real such install.

			args := []string{
				"template", "render-guard-" + e.ID, e.ChartRef,
				"--version", e.ChartVersion,
				"--kube-version", renderGuardKubeVersion,
				"--api-versions", renderGuardAPIVersions,
			}
			for sharedKey, path := range mapping {
				stub, known := stubIdentityValues[sharedKey]
				if !known {
					t.Fatalf("entry %q: identity-keys annotation names unknown shared setting %q", e.ID, sharedKey)
				}
				args = append(args, "--set", fmt.Sprintf("%s=%s", path, stub))
			}

			out, err := exec.Command("helm", args...).CombinedOutput()
			if err != nil {
				t.Fatalf("chart %s @ %s does not render with only the values core would supply (ARCHITECTURE item 68) — a Store install would fail the same way:\n%s",
					e.ChartRef, e.ChartVersion, strings.TrimSpace(string(out)))
			}
		})
	}

	if checked == 0 {
		t.Fatal("expected at least one bundled entry with a published chart to actually check")
	}
}
