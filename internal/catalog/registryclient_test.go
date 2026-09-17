package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistryClient_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/modules" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{
				"id": "acme-forecast",
				"displayName": "Acme Forecast",
				"icon": "https://example.com/icon.svg",
				"description": "Demand forecasting models as a Booth module.",
				"category": "analytics",
				"chartRef": "oci://registry.example.com/charts/acme-forecast",
				"chartVersion": "1.4.2",
				"manifestPreview": {
					"hasOwnUi": true,
					"uiIntegrationMode": "iframe-proxy",
					"navGroup": "build"
				}
			},
			{
				"id": "unparseable-chart",
				"displayName": "Unparseable",
				"chartRef": "https://not-oci.example.com/chart",
				"chartVersion": "1.0.0"
			}
		]`))
	}))
	defer srv.Close()

	client := NewRegistryClient()
	entries, err := client.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	first := entries[0]
	if first.ID != "acme-forecast" || first.Source.Kind != SourceRegistry || first.Source.Name != srv.URL {
		t.Errorf("unexpected first entry: %+v", first)
	}
	wantChart := ChartRef{RepoURL: "oci://registry.example.com/charts", ChartName: "acme-forecast", Version: "1.4.2"}
	if first.Chart != wantChart {
		t.Errorf("Chart = %+v, want %+v", first.Chart, wantChart)
	}
	if !first.ManifestPreview.HasOwnUI || first.ManifestPreview.UIIntegrationMode != "iframe-proxy" {
		t.Errorf("unexpected manifestPreview: %+v", first.ManifestPreview)
	}

	second := entries[1]
	if !second.Chart.IsZero() {
		t.Errorf("expected unparseable chartRef to leave Chart zero, got %+v", second.Chart)
	}
}

func TestRegistryClient_Fetch_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewRegistryClient()
	if _, err := client.Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}
