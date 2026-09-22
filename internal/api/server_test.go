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
	if got[0].SuggestedNamespace != "booth-storage" {
		t.Errorf("SuggestedNamespace = %q, want booth-storage (a UI pre-fill hint only, ADR 0029)", got[0].SuggestedNamespace)
	}
}

// TestHandleGetCatalog_RealNamespaceOverridesGuess is the ADR 0060 regression guard: a
// module installed into a namespace other than the "booth-<id>" guess must surface its
// real namespace, so the confirm-uninstall UI doesn't pre-fill a value that silently
// no-ops against booth-core's idempotent uninstall.
func TestHandleGetCatalog_RealNamespaceOverridesGuess(t *testing.T) {
	core := fakeCoreServer(t, []coreclient.Module{{ID: "storage", Phase: "Healthy", Namespace: "acme-storage-team-3"}})
	defer core.Close()

	deps := Deps{
		Core:           coreclient.New(core.URL),
		Bundled:        []catalog.Entry{{ID: "storage", DisplayName: "Storage"}},
		RegistryClient: catalog.NewRegistryClient(),
	}

	req := withTestIdentity(httptest.NewRequest(http.MethodGet, "/api/catalog", nil))
	rec := httptest.NewRecorder()
	handleGetCatalog(deps)(rec, req)

	var got []catalog.Entry
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got[0].Namespace != "acme-storage-team-3" {
		t.Errorf("Namespace = %q, want the real namespace from booth-core's registry, not the booth-<id> guess", got[0].Namespace)
	}
	if got[0].SuggestedNamespace != "booth-storage" {
		t.Errorf("SuggestedNamespace = %q, want the guess to still be present as the install-flow fallback", got[0].SuggestedNamespace)
	}
}

// TestHandleGetCatalog_MissingNamespaceFromOlderCore is tolerance for a booth-core that
// hasn't shipped ADR 0060 yet — Namespace stays empty rather than erroring, leaving
// SuggestedNamespace as the only pre-fill available.
func TestHandleGetCatalog_MissingNamespaceFromOlderCore(t *testing.T) {
	core := fakeCoreServer(t, []coreclient.Module{{ID: "storage", Phase: "Healthy"}}) // no Namespace field
	defer core.Close()

	deps := Deps{
		Core:           coreclient.New(core.URL),
		Bundled:        []catalog.Entry{{ID: "storage", DisplayName: "Storage"}},
		RegistryClient: catalog.NewRegistryClient(),
	}

	req := withTestIdentity(httptest.NewRequest(http.MethodGet, "/api/catalog", nil))
	rec := httptest.NewRecorder()
	handleGetCatalog(deps)(rec, req)

	var got []catalog.Entry
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got[0].Namespace != "" {
		t.Errorf("Namespace = %q, want empty when booth-core doesn't supply one", got[0].Namespace)
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

// TestHandleInstall_RequiresExplicitNamespace is the ADR 0029 regression guard: this
// endpoint must never silently pick a namespace on the caller's behalf, even though it
// still suggests one in the catalog response (handleGetCatalog) for a confirmation UI
// to pre-fill.
func TestHandleInstall_RequiresExplicitNamespace(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("booth-core should never be called when namespace is missing")
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

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (namespace is required, never defaulted — ADR 0029)", rec.Code)
	}
}

func TestHandleInstall_PassesThroughExplicitNamespace(t *testing.T) {
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

	req := withTestIdentity(httptest.NewRequest(http.MethodPost, "/api/catalog/storage/install", strings.NewReader(`{"namespace":"acme-storage-team-3"}`)))
	req = withURLParam(req, "id", "storage")
	rec := httptest.NewRecorder()
	handleInstall(deps)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotNamespace != "acme-storage-team-3" {
		t.Errorf("namespace = %q, want the caller-confirmed value passed through unchanged", gotNamespace)
	}
}

// TestHandleInstall_RegistryEntryPassesThroughChartRef is the ADR 0028 regression
// guard, exercised through the full handler: a registry-sourced entry's raw
// chartRef/chartVersion must reach booth-core unparsed.
func TestHandleInstall_RegistryEntryPassesThroughChartRef(t *testing.T) {
	var gotBody map[string]any
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/modules" {
			json.NewEncoder(w).Encode([]coreclient.Module{})
			return
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer core.Close()

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"acme-forecast","displayName":"Acme Forecast","chartRef":"oci://registry.example.com/charts/acme-forecast","chartVersion":"1.4.2"}]`))
	}))
	defer registry.Close()

	deps := Deps{
		Core:           coreclient.New(core.URL),
		RegistryURLs:   []string{registry.URL},
		RegistryClient: catalog.NewRegistryClient(),
	}

	req := withTestIdentity(httptest.NewRequest(http.MethodPost, "/api/catalog/acme-forecast/install", strings.NewReader(`{"namespace":"booth-acme-forecast"}`)))
	req = withURLParam(req, "id", "acme-forecast")
	rec := httptest.NewRecorder()
	handleInstall(deps)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotBody["chartRef"] != "oci://registry.example.com/charts/acme-forecast" {
		t.Errorf("chartRef = %v, want the registry's raw value passed through unchanged", gotBody["chartRef"])
	}
	if gotBody["chartVersion"] != "1.4.2" {
		t.Errorf("chartVersion = %v", gotBody["chartVersion"])
	}
}

func TestHandleUninstall_RequiresExplicitNamespace(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("booth-core should never be called when namespace is missing")
	}))
	defer core.Close()

	deps := Deps{Core: coreclient.New(core.URL)}

	req := withTestIdentity(httptest.NewRequest(http.MethodDelete, "/api/catalog/storage", nil))
	req = withURLParam(req, "id", "storage")
	rec := httptest.NewRecorder()
	handleUninstall(deps)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (namespace is required, never defaulted — ADR 0029)", rec.Code)
	}
}

func TestHandleUninstall_WithExplicitNamespace(t *testing.T) {
	core := fakeCoreServer(t, nil)
	defer core.Close()

	deps := Deps{Core: coreclient.New(core.URL)}

	req := withTestIdentity(httptest.NewRequest(http.MethodDelete, "/api/catalog/storage?namespace=booth-storage", nil))
	req = withURLParam(req, "id", "storage")
	rec := httptest.NewRecorder()
	handleUninstall(deps)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type staticVerifier struct{ claims *auth.Claims }

func (s staticVerifier) Verify(context.Context, string) (*auth.Claims, error) { return s.claims, nil }

// TestRouter_ForgedRoleHeaderNeverReachesCore is the ADR 0041 guard at the routing level:
// a genuine viewer token sent straight to this pod with a forged "X-Booth-Role: owner"
// must be rejected by the middleware, without booth-core ever being called.
func TestRouter_ForgedRoleHeaderNeverReachesCore(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("booth-core must not be called for a forged-role request, got %s %s", r.Method, r.URL.Path)
	}))
	defer core.Close()

	router := NewRouter(Deps{
		Verifier:       staticVerifier{claims: &auth.Claims{Subject: "u1", Groups: []string{"/workspaces/acme/viewer"}}},
		Core:           coreclient.New(core.URL),
		Bundled:        []catalog.Entry{{ID: "storage", Chart: catalog.ChartRef{ChartName: "storage"}}},
		RegistryClient: catalog.NewRegistryClient(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/catalog/storage/install", strings.NewReader(`{"namespace":"booth-storage"}`))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("X-Booth-Workspace", "acme")
	req.Header.Set("X-Booth-Role", "owner")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
