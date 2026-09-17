package coreclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/projectbooth/booth-module-store/internal/catalog"
)

func TestListModules(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/modules" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Workspace"); got != "acme" {
			t.Errorf("X-Workspace = %q", got)
		}
		json.NewEncoder(w).Encode([]Module{{ID: "storage", Phase: "Healthy"}})
	}))
	defer srv.Close()

	client := New(srv.URL)
	modules, err := client.ListModules(context.Background(), "test-token", "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(modules) != 1 || modules[0].ID != "storage" {
		t.Fatalf("got %+v", modules)
	}
}

func TestInstall_RejectsZeroChart(t *testing.T) {
	client := New("http://unused")
	err := client.Install(context.Background(), "tok", "acme", "storage", catalog.ChartRef{}, "booth-storage", nil)
	if err == nil {
		t.Fatal("expected error for unresolved chart, got nil")
	}
}

func TestInstall_SendsExpectedBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/modules/storage/install" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	client := New(srv.URL)
	chart := catalog.ChartRef{RepoURL: "oci://registry.example.com/charts", ChartName: "storage", Version: "1.0.0"}
	err := client.Install(context.Background(), "tok", "acme", "storage", chart, "booth-storage", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody["namespace"] != "booth-storage" {
		t.Errorf("namespace = %v", gotBody["namespace"])
	}
	chartBody, ok := gotBody["chart"].(map[string]any)
	if !ok {
		t.Fatalf("chart field missing or wrong type: %v", gotBody["chart"])
	}
	if chartBody["chartName"] != "storage" || chartBody["repoUrl"] != "oci://registry.example.com/charts" {
		t.Errorf("unexpected chart body: %+v", chartBody)
	}
}

func TestUninstall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/modules/storage" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("namespace") != "booth-storage" {
			t.Errorf("namespace query = %q", r.URL.Query().Get("namespace"))
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.Uninstall(context.Background(), "tok", "acme", "storage", "booth-storage"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
