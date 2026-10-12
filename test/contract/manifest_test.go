// Package contract validates booth-module-store's own manifest — the BoothModule
// custom resource its Helm chart templates — against contracts/module-manifest.md's
// schema. Per contracts/testing-strategy.md, this runs against a rendered template
// (via `helm template`), not a real deployed cluster — no cluster needed, but a `helm`
// binary is: CI's ci.yml sets one up before running this alongside the rest of the
// unit/contract layer.
package contract

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type boothModule struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		ID                string `yaml:"id"`
		DisplayName       string `yaml:"displayName"`
		Version           string `yaml:"version"`
		ContractVersion   string `yaml:"contractVersion"`
		HasOwnUI          bool   `yaml:"hasOwnUi"`
		UIIntegrationMode string `yaml:"uiIntegrationMode"`
		HealthCheckPath   string `yaml:"healthCheckPath"`
		NavGroup          string `yaml:"navGroup"`
		NavPath           string `yaml:"navPath"`
		ServiceRef        struct {
			Name string `yaml:"name"`
			Port int    `yaml:"port"`
		} `yaml:"serviceRef"`
	} `yaml:"spec"`
}

func renderBoothModule(t *testing.T) boothModule {
	t.Helper()

	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed; this contract test runs in CI where it is (see .github/workflows/ci.yml)")
	}

	chartDir := filepath.Join("..", "..", "charts", "booth-module-store")
	cmd := exec.Command("helm", "template", "module-store-contract-test", chartDir,
		"--namespace", "booth-system",
		"--set", "oidc.issuerUrl=https://keycloak.example.com/realms/booth",
		"--set", "oidc.clientId=booth-module-store",
		"--show-only", "templates/boothmodule.yaml",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}

	var m boothModule
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("parsing rendered BoothModule: %v\nrendered:\n%s", err, out)
	}
	return m
}

// TestManifest_RequiredFields checks every field contracts/module-manifest.md marks
// required is actually populated in our rendered manifest.
func TestManifest_RequiredFields(t *testing.T) {
	m := renderBoothModule(t)

	if m.Kind != "BoothModule" {
		t.Errorf("Kind = %q, want BoothModule", m.Kind)
	}
	if m.Spec.ID != "module-store" {
		t.Errorf("spec.id = %q, want module-store", m.Spec.ID)
	}
	if m.Spec.DisplayName == "" {
		t.Error("spec.displayName is required but empty")
	}
	if m.Spec.Version == "" {
		t.Error("spec.version is required but empty")
	}
	if m.Spec.ContractVersion == "" {
		t.Error("spec.contractVersion is required but empty")
	}
	if m.Spec.HealthCheckPath == "" {
		t.Error("spec.healthCheckPath is required but empty")
	}
	if m.Spec.ServiceRef.Name == "" || m.Spec.ServiceRef.Port == 0 {
		t.Errorf("spec.serviceRef is required (name+port) but got %+v", m.Spec.ServiceRef)
	}
}

// TestManifest_UIIntegrationMode checks the "required if hasOwnUi" rule
// (contracts/module-manifest.md) and this module's own shell-level-slot carve-out:
// navGroup/navPath must stay unset since Module Store isn't a Build/View/Manage nav
// entry (ADR 0027).
func TestManifest_UIIntegrationMode(t *testing.T) {
	m := renderBoothModule(t)

	if !m.Spec.HasOwnUI {
		t.Fatal("spec.hasOwnUi = false, want true (booth-module-store ships a native UI)")
	}
	if m.Spec.UIIntegrationMode != "native" && m.Spec.UIIntegrationMode != "iframe-proxy" {
		t.Errorf("spec.uiIntegrationMode = %q, required to be native|iframe-proxy when hasOwnUi is true", m.Spec.UIIntegrationMode)
	}
	if m.Spec.NavGroup != "" {
		t.Errorf("spec.navGroup = %q, want empty: Module Store is a shell-level slot, not a Build/View/Manage entry (contracts/module-manifest.md)", m.Spec.NavGroup)
	}
	if m.Spec.NavPath != "" {
		t.Errorf("spec.navPath = %q, want empty: same shell-level-slot reasoning as navGroup", m.Spec.NavPath)
	}
}

// valueAtPath walks a dotted values.yaml path (e.g. "oidc.jwksUrl") through a generic
// YAML tree and reports whether it resolves to a leaf (which is all core's merge needs:
// a default to be absent-or-present under, per ADR 0113 item 8).
func valueAtPath(root map[string]any, dotted string) (any, bool) {
	cur := any(root)
	for _, part := range strings.Split(dotted, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// TestChart_IdentityKeysAnnotation is the ADR 0113 / module-manifest.md "Chart
// annotation: identity keys" contract check: the annotation must be valid JSON, and
// every path it names must actually exist (with a default) in values.yaml, or core's
// install-time merge (and a plain `helm template`/CLI install) would silently do nothing
// for a typo'd path.
func TestChart_IdentityKeysAnnotation(t *testing.T) {
	chartBytes, err := os.ReadFile(filepath.Join("..", "..", "charts", "booth-module-store", "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var chart struct {
		Annotations map[string]string `yaml:"annotations"`
	}
	if err := yaml.Unmarshal(chartBytes, &chart); err != nil {
		t.Fatalf("parsing Chart.yaml: %v", err)
	}
	raw, ok := chart.Annotations["booth.projectbooth.io/identity-keys"]
	if !ok {
		t.Fatal("Chart.yaml is missing the booth.projectbooth.io/identity-keys annotation (ADR 0113)")
	}

	var mapping map[string]string
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		t.Fatalf("booth.projectbooth.io/identity-keys is not valid JSON: %v\nvalue: %s", err, raw)
	}
	if len(mapping) == 0 {
		t.Fatal("booth.projectbooth.io/identity-keys decoded to an empty object")
	}

	knownKeys := map[string]bool{
		"issuerUrl": true, "clientId": true, "jwksUrl": true,
		"workloadIssuerUrl": true, "groupsClaim": true, "iframeIssuerUrl": true,
	}

	valuesBytes, err := os.ReadFile(filepath.Join("..", "..", "charts", "booth-module-store", "values.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(valuesBytes, &values); err != nil {
		t.Fatalf("parsing values.yaml: %v", err)
	}

	for key, path := range mapping {
		if !knownKeys[key] {
			t.Errorf("identity-keys annotation names unknown shared setting %q (want one of issuerUrl, clientId, jwksUrl, workloadIssuerUrl, groupsClaim, iframeIssuerUrl)", key)
		}
		if _, ok := valueAtPath(values, path); !ok {
			t.Errorf("identity-keys annotation maps %q to values.yaml path %q, which has no default in values.yaml", key, path)
		}
	}

	for _, want := range []struct{ sharedKey, path string }{
		{"issuerUrl", "oidc.issuerUrl"},
		{"clientId", "oidc.clientId"},
		{"jwksUrl", "oidc.jwksUrl"},
		{"groupsClaim", "oidc.groupsClaim"},
	} {
		if got := mapping[want.sharedKey]; got != want.path {
			t.Errorf("identity-keys[%q] = %q, want %q", want.sharedKey, got, want.path)
		}
	}
}
