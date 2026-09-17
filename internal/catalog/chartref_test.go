package catalog

import "testing"

func TestParseRegistryChartRef_OCI(t *testing.T) {
	got, err := ParseRegistryChartRef("oci://registry.example.com/charts/acme-forecast", "1.4.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ChartRef{
		RepoURL:   "oci://registry.example.com/charts",
		ChartName: "acme-forecast",
		Version:   "1.4.2",
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseRegistryChartRef_TrailingSlash(t *testing.T) {
	got, err := ParseRegistryChartRef("oci://registry.example.com/charts/acme-forecast/", "1.4.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ChartName != "acme-forecast" {
		t.Errorf("ChartName = %q, want acme-forecast", got.ChartName)
	}
}

func TestParseRegistryChartRef_Rejects(t *testing.T) {
	cases := []string{
		"",
		"https://charts.example.com/acme-forecast",
		"oci://acme-forecast",
		"oci://host/",
	}
	for _, c := range cases {
		if _, err := ParseRegistryChartRef(c, "1.0.0"); err == nil {
			t.Errorf("ParseRegistryChartRef(%q): expected error, got nil", c)
		}
	}
}
