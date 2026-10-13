import { useEffect, useMemo, useState } from "react";
import type { CatalogEntry, WorkspaceRole } from "./types";
import { fetchCatalog, type GetAccessToken } from "./api/client";
import { categoriesOf, filterCatalog } from "./catalogFilter";
import { INSTALL_POLL_INITIAL_DELAY_MS, INSTALL_POLL_TIMEOUT_MS, nextInstallPollDelay } from "./installPolling";
import { SearchBar } from "./components/SearchBar";
import { CategoryFilter } from "./components/CategoryFilter";
import { CatalogGrid } from "./components/CatalogGrid";

/**
 * Props contract agreed with booth-design and pinned into contracts/ui-integration.md
 * by ADR 0031 (workspace/role/theme) and ADR 0033 (getAccessToken): plain React
 * props, not a shared context object, so this package never has to depend on
 * anything booth-design exports (that would invert the dependency direction ADR 0030
 * established).
 *
 * `theme` is included for any JS-driven decision a native module might need, even
 * though CSS-only styling here already follows the ambient `data-theme` attribute on
 * a document ancestor (the same convention booth-design's own useTheme hook
 * documents). This component applies `theme` directly to its own root element too, so
 * it renders correctly even if mounted somewhere that hasn't already set that
 * attribute on an ancestor — the explicit prop is authoritative, ambient DOM state is
 * just a fallback for callers that don't pass one (e.g. this repo's own dev harness
 * before it existed).
 */
export interface ModuleStoreAppProps {
  /** Active workspace slug (ADR 0025) — required for every API call this component makes. */
  workspace: string;
  /** Caller's role in the active workspace (ADR 0025) — gates install/uninstall
   *  actions in this UI. booth-core still independently enforces owner-only on the
   *  actual install/uninstall calls (ADR 0023); this is a UX nicety, not the security
   *  boundary. */
  role: WorkspaceRole;
  theme: "dark" | "light";
  /** Returns booth-design's current bearer token (its client-side OIDC PKCE flow,
   *  ADR 0032), or null if not yet authenticated / logged out. A callback rather than
   *  a plain value specifically to survive booth-design's silent token refresh
   *  without going stale (ADR 0033) — this component calls it fresh immediately
   *  before every API request, never caches the result. */
  getAccessToken: GetAccessToken;
}

// The native-mode component booth-design's shell mounts at its reserved "Module
// Store" slot (ADR 0027, ADR 0030). Published as @projectbooth/module-store-ui — see
// this repo's README for the package build/publish setup.
export function ModuleStoreApp({ workspace, role, theme, getAccessToken }: ModuleStoreAppProps) {
  const [entries, setEntries] = useState<CatalogEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string | null>(null);

  // ARCHITECTURE.md item 70: which modules this page itself has accepted an install for
  // this session (module id -> when that install was accepted), and any error/timeout
  // text for a module whose install this page gave up waiting on. Tracked here, at the
  // top level, rather than in each ModuleCard, so the "Installing" state survives a
  // catalog reload (including this component's own poll below) and a card
  // unmounting/remounting (e.g. a search/category filter hiding and reshowing it) — core
  // itself has no durable install-in-progress status to query instead (decision 0005).
  const [installingIds, setInstallingIds] = useState<Map<string, number>>(new Map());
  const [installErrors, setInstallErrors] = useState<Record<string, string>>({});

  async function reload() {
    try {
      const data = await fetchCatalog(workspace, getAccessToken);
      setEntries(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  useEffect(() => {
    reload();
    // Only re-fetch when the active workspace changes, not on every render — reload
    // itself is redefined each render and intentionally left out of this dependency
    // list. getAccessToken is deliberately not a dependency either: it's called fresh
    // at request time regardless of when this effect last ran (ADR 0033) — there's no
    // "token changed" render signal to react to in the first place.
  }, [workspace]);

  // The install-poll loop (ARCHITECTURE item 70): while any module is mid-install from
  // this page, repeatedly re-fetch the catalog with backoff and check whether core now
  // lists it as installed. A module that never shows up within INSTALL_POLL_TIMEOUT_MS
  // is treated as failed/timed out — core gives this page no other signal to distinguish
  // "still installing" from "the Helm install actually failed" once the initial 202 has
  // been accepted (ADR 0113's module-store, like core, has no durable desired state).
  useEffect(() => {
    if (installingIds.size === 0) return;

    let cancelled = false;
    let delay = INSTALL_POLL_INITIAL_DELAY_MS;
    let timer: ReturnType<typeof setTimeout>;

    async function poll() {
      if (cancelled) return;
      try {
        const data = await fetchCatalog(workspace, getAccessToken);
        if (cancelled) return;
        setEntries(data);
        setError(null);

        const now = Date.now();
        setInstallingIds((prev) => {
          if (prev.size === 0) return prev;
          const next = new Map(prev);
          for (const [id, startedAt] of prev) {
            const found = data.find((e) => e.id === id);
            if (found?.status.state === "installed") {
              next.delete(id);
            } else if (now - startedAt >= INSTALL_POLL_TIMEOUT_MS) {
              next.delete(id);
              setInstallErrors((errs) => ({
                ...errs,
                [id]: "Timed out waiting for core to finish installing this module. It may still complete — check back shortly, or try again.",
              }));
            }
          }
          return next;
        });
      } catch {
        // A transient catalog-fetch failure while polling shouldn't abandon the wait —
        // the next scheduled poll will retry on its own.
      }
      if (!cancelled) {
        delay = nextInstallPollDelay(delay);
        timer = setTimeout(poll, delay);
      }
    }

    timer = setTimeout(poll, delay);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
    // Re-arms whenever the pending set transitions between empty and non-empty. The loop
    // itself always reads the latest pending set via setInstallingIds's updater function
    // above, so a module added to (or removed from) the set mid-loop doesn't need to be
    // a dependency here — this effect only needs to start (or stop) the loop.
  }, [installingIds.size > 0, workspace, getAccessToken]);

  function beginInstalling(id: string) {
    setInstallingIds((prev) => {
      const next = new Map(prev);
      next.set(id, Date.now());
      return next;
    });
    setInstallErrors((prev) => {
      if (!(id in prev)) return prev;
      const next = { ...prev };
      delete next[id];
      return next;
    });
  }

  const categories = useMemo(() => categoriesOf(entries ?? []), [entries]);
  const filtered = useMemo(() => filterCatalog(entries ?? [], { query, category }), [entries, query, category]);

  return (
    <div data-theme={theme} className="flex flex-col gap-4">
      {error && <p className="text-sm text-red-600 dark:text-red-400">Couldn't load the Module Store: {error}</p>}
      {!error && entries === null && <p className="text-sm text-slate-500 dark:text-slate-400">Loading modules…</p>}
      {!error && entries !== null && (
        <>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <SearchBar value={query} onChange={setQuery} />
          </div>
          <CategoryFilter categories={categories} selected={category} onChange={setCategory} />
          <CatalogGrid
            entries={filtered}
            workspace={workspace}
            role={role}
            getAccessToken={getAccessToken}
            onChanged={reload}
            installingIds={installingIds}
            installErrors={installErrors}
            onInstallAccepted={beginInstalling}
          />
        </>
      )}
    </div>
  );
}
