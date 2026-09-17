// Package api is booth-module-store's own HTTP surface: what its native-mode frontend
// calls (contracts/ui-integration.md), reached through booth-core's gateway at
// /modules/module-store/* once installed. This module never re-implements install/
// uninstall — every mutating call here is a thin, chart-reference-supplying wrapper
// over booth-core's existing API (contracts/core-platform-api.md).
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/projectbooth/booth-module-store/internal/auth"
	"github.com/projectbooth/booth-module-store/internal/catalog"
	"github.com/projectbooth/booth-module-store/internal/coreclient"
)

// Deps is everything the HTTP layer needs, assembled by cmd/module-store/main.go.
type Deps struct {
	Verifier       *auth.Verifier
	Core           *coreclient.Client
	Bundled        []catalog.Entry
	RegistryURLs   []string
	RegistryClient *catalog.RegistryClient
}

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger) // stdout logging only, per ADR 0022 — no logging API to integrate against
	r.Use(middleware.Recoverer)

	// Unauthenticated: our own liveness/readiness. Also the healthCheckPath this
	// module declares in its own BoothModule manifest (contracts/module-manifest.md)
	// for booth-core to poll.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(deps.Verifier))

		r.Get("/api/catalog", handleGetCatalog(deps))
		r.Post("/api/catalog/{id}/install", handleInstall(deps))
		r.Delete("/api/catalog/{id}", handleUninstall(deps))
	})

	return r
}

// fetchRegistryTiers calls every configured external registry concurrently. A single
// registry being unreachable does not fail the whole catalog view — ADR 0027's bundled
// tier must keep working regardless of any external registry's availability; a failed
// registry is logged and simply contributes no entries this request.
func fetchRegistryTiers(ctx context.Context, deps Deps) [][]catalog.Entry {
	tiers := make([][]catalog.Entry, len(deps.RegistryURLs))

	var wg sync.WaitGroup
	for i, url := range deps.RegistryURLs {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			entries, err := deps.RegistryClient.Fetch(ctx, url)
			if err != nil {
				log.Printf("registry %s unavailable: %v", url, err)
				return
			}
			tiers[i] = entries
		}(i, url)
	}
	wg.Wait()

	return tiers
}

type coreInstalledLookup map[string]string // id -> phase

func (l coreInstalledLookup) Lookup(id string) (string, bool) {
	phase, ok := l[id]
	return phase, ok
}

func handleGetCatalog(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.FromContext(r.Context())
		if !ok {
			http.Error(w, "no identity", http.StatusUnauthorized)
			return
		}

		tiers := fetchRegistryTiers(r.Context(), deps)

		modules, err := deps.Core.ListModules(r.Context(), identity.RawToken, identity.Workspace)
		if err != nil {
			http.Error(w, "listing installed modules: "+err.Error(), http.StatusBadGateway)
			return
		}
		lookup := make(coreInstalledLookup, len(modules))
		for _, m := range modules {
			lookup[m.ID] = m.Phase
		}

		merged := catalog.Merge(deps.Bundled, tiers, lookup)
		writeJSON(w, http.StatusOK, merged)
	}
}

// findEntry looks up a catalog entry by id, bundled tier first, then registries in
// configured order — the first match wins. A client wanting a specific duplicate
// across sources should pin the exact source client-side before deciding to install;
// v0 doesn't yet expose a way to disambiguate two same-id entries from different
// sources in the install request itself.
func findEntry(ctx context.Context, deps Deps, id string) (catalog.Entry, bool) {
	for _, e := range deps.Bundled {
		if e.ID == id {
			return e, true
		}
	}
	for _, tier := range fetchRegistryTiers(ctx, deps) {
		for _, e := range tier {
			if e.ID == id {
				return e, true
			}
		}
	}
	return catalog.Entry{}, false
}

type mutateRequest struct {
	Namespace string         `json:"namespace,omitempty"`
	Values    map[string]any `json:"values,omitempty"`
}

// defaultNamespace is this repo's interim convention (docs/decisions/0002-install-
// namespace-convention.md) for where a module installs when the caller doesn't specify
// one explicitly. No architecture-level ADR has settled this yet.
func defaultNamespace(moduleID string) string {
	return "booth-" + moduleID
}

func handleInstall(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.FromContext(r.Context())
		if !ok {
			http.Error(w, "no identity", http.StatusUnauthorized)
			return
		}

		id := chi.URLParam(r, "id")
		entry, found := findEntry(r.Context(), deps, id)
		if !found {
			http.Error(w, "unknown catalog entry: "+id, http.StatusNotFound)
			return
		}

		var req mutateRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		namespace := req.Namespace
		if namespace == "" {
			namespace = defaultNamespace(id)
		}

		if err := deps.Core.Install(r.Context(), identity.RawToken, identity.Workspace, id, entry.Chart, namespace, req.Values); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func handleUninstall(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.FromContext(r.Context())
		if !ok {
			http.Error(w, "no identity", http.StatusUnauthorized)
			return
		}

		id := chi.URLParam(r, "id")
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = defaultNamespace(id)
		}

		if err := deps.Core.Uninstall(r.Context(), identity.RawToken, identity.Workspace, id, namespace); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
