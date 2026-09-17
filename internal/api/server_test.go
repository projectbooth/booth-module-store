package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/projectbooth/booth-module-store/internal/auth"
	"github.com/projectbooth/booth-module-store/internal/catalog"
	"github.com/projectbooth/booth-module-store/internal/coreclient"
)

func withTestIdentity(r *http.Request) *http.Request {
	identity := auth.Identity{
		Claims:    &auth.Claims{Subject: "user-1"},
		Workspace: "acme",
		Role:      "owner",
		RawToken:  "test-token",
	}
	return r.WithContext(auth.WithIdentityForTesting(r.Context(), identity))
}

// withURLParam attaches a chi route param the way the real router would after
// matching "/api/catalog/{id}/..." — needed because these tests call handlers
// directly, bypassing chi's own routing.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func fakeCoreServer(t *testing.T, installed []coreclient.Module) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/modules":
			json.NewEncoder(w).Encode(installed)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/install"):
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestHealthz_Unauthenticated(t *testing.T) {
	router := NewRouter(Deps{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleGetCatalog_MergesAndCrossReferences(t *testing.T) {
	core := fakeCoreServer(t, []coreclient.Module{{ID: "storage", Phase: "Healthy"}})
	defer core.Close()

	deps := Deps{
		Core:           coreclient.New(core.URL),
		Bundled:        []catalog.Entry{{ID: "storage", DisplayName: "Storage", Source: catalog.Source{Kind: catalog.SourceBundled}}},
		RegistryClient: catalog.NewRegistryClient(),
	}

	req := withTestIdentity(httptest.NewRequest(http.MethodGet, "/api/catalog", nil))
	rec := httptest.NewRecorder()
	handleGetCatalog(deps)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got []catalog.Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].Status.State != catalog.Installed || got[0].Status.Health != "Healthy" {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleInstall_UnknownEntry(t *testing.T) {
	core := fakeCoreServer(t, nil)
	defer core.Close()

	deps := Deps{
		Core:           coreclient.New(core.URL),
		RegistryClient: catalog.NewRegistryClient(),
	}

	req := withTestIdentity(httptest.NewRequest(http.MethodPost, "/api/catalog/does-not-exist/install", strings.NewReader(`{}`)))
	req = withURLParam(req, "id", "does-not-exist")
	rec := httptest.NewRecorder()
	handleInstall(deps)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleInstall_DefaultsNamespace(t *testing.T) {
	var gotNamespace string
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotNamespace, _ = body["namespace"].(string)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer core.Close()

	deps := Deps{
		Core: coreclient.New(core.URL),
		Bundled: []catalog.Entry{{
			ID:    "storage",
			Chart: catalog.ChartRef{RepoURL: "oci://registry.example.com/charts", ChartName: "storage", Version: "1.0.0"},
		}},
		RegistryClient: catalog.NewRegistryClient(),
	}

	req := withTestIdentity(httptest.NewRequest(http.MethodPost, "/api/catalog/storage/install", strings.NewReader(`{}`)))
	req = withURLParam(req, "id", "storage")
	rec := httptest.NewRecorder()
	handleInstall(deps)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotNamespace != "booth-storage" {
		t.Errorf("namespace = %q, want booth-storage", gotNamespace)
	}
}

func TestHandleUninstall(t *testing.T) {
	core := fakeCoreServer(t, nil)
	defer core.Close()

	deps := Deps{Core: coreclient.New(core.URL)}

	req := withTestIdentity(httptest.NewRequest(http.MethodDelete, "/api/catalog/storage", nil))
	req = withURLParam(req, "id", "storage")
	rec := httptest.NewRecorder()
	handleUninstall(deps)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
