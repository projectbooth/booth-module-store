package contract

import (
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type deployment struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Env []struct {
						Name  string `yaml:"name"`
						Value string `yaml:"value"`
					} `yaml:"env"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// renderedEnv renders the chart's Deployment with the given extra --set flags and
// returns its container env as a name -> value map.
func renderedEnv(t *testing.T, extraSet ...string) map[string]string {
	t.Helper()

	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed; this contract test runs in CI where it is (see .github/workflows/ci.yml)")
	}

	args := []string{"template", "module-store-contract-test", filepath.Join("..", "..", "charts", "booth-module-store"),
		"--namespace", "some-other-namespace", // deliberately NOT booth-system: the whole point is installs into their own namespace
		"--set", "oidc.issuerUrl=https://keycloak.example.com/realms/booth",
		"--set", "oidc.clientId=booth-module-store",
		"--show-only", "templates/deployment.yaml",
	}
	for _, s := range extraSet {
		args = append(args, "--set", s)
	}
	out, err := exec.Command("helm", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}

	var d deployment
	if err := yaml.Unmarshal(out, &d); err != nil {
		t.Fatalf("parsing rendered Deployment: %v\n%s", err, out)
	}
	if len(d.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected one container, got %d", len(d.Spec.Template.Spec.Containers))
	}
	env := map[string]string{}
	for _, e := range d.Spec.Template.Spec.Containers[0].Env {
		env[e.Name] = e.Value
	}
	return env
}

// TestDeployment_CoreBaseURLIsNamespaceQualifiedByDefault is the regression guard for a
// bug found live against a real cluster: the chart defaulted BOOTH_CORE_BASE_URL to the
// bare hostname "http://booth-core", which Kubernetes DNS resolves only from inside
// booth-core's own namespace. This module installs into its own, so every default
// install failed at runtime with "dial tcp: lookup booth-core: no such host".
func TestDeployment_CoreBaseURLIsNamespaceQualifiedByDefault(t *testing.T) {
	got := renderedEnv(t)["BOOTH_CORE_BASE_URL"]

	u, err := url.Parse(got)
	if err != nil || u.Hostname() == "" {
		t.Fatalf("BOOTH_CORE_BASE_URL = %q is not a valid URL", got)
	}
	// A bare Service name has no dots; a resolvable cross-namespace name has at least
	// <service>.<namespace>.
	if strings.Count(u.Hostname(), ".") < 2 {
		t.Errorf("BOOTH_CORE_BASE_URL host %q is not namespace-qualified (want <service>.<namespace>.svc); a bare name only resolves within booth-core's own namespace", u.Hostname())
	}
	if u.Port() == "" {
		t.Errorf("BOOTH_CORE_BASE_URL = %q has no explicit port; booth-core listens on 8080, not the implicit 80", got)
	}
	if got != "http://booth-core.booth-system.svc:8080" {
		t.Errorf("BOOTH_CORE_BASE_URL = %q, want booth-core's chart defaults (booth-core.booth-system.svc:8080), matching booth-design's chart", got)
	}
}

func TestDeployment_CoreBaseURLIsOverridable(t *testing.T) {
	got := renderedEnv(t, "core.baseUrl=http://core.platform.svc:9090")["BOOTH_CORE_BASE_URL"]
	if got != "http://core.platform.svc:9090" {
		t.Errorf("BOOTH_CORE_BASE_URL = %q, want the override to be honored (booth-core may be installed under a different release name/namespace)", got)
	}
}
