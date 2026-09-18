import { useEffect, useMemo, useState } from "react";
import type { CatalogEntry, WorkspaceRole } from "./types";
import { fetchCatalog } from "./api/client";
import { categoriesOf, filterCatalog } from "./catalogFilter";
import { SearchBar } from "./components/SearchBar";
import { CategoryFilter } from "./components/CategoryFilter";
import { CatalogGrid } from "./components/CatalogGrid";

/**
 * Props contract agreed with booth-design (ADR 0030's first real consumer pass — see
 * docs/decisions/0003-native-module-props-contract.md): plain React props, not a
 * shared context object, so this package never has to depend on anything
 * booth-design exports (that would invert the dependency direction ADR 0030 fixed).
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
  /** The bearer token booth-design obtained via its client-side OIDC PKCE flow (ADR
   *  0032) — booth-core has no cookie/session support, every API call needs this
   *  attached as `Authorization: Bearer <token>`. See
   *  docs/decisions/0004-native-module-access-token-prop.md. */
  accessToken: string;
}

// The native-mode component booth-design's shell mounts at its reserved "Module
// Store" slot (ADR 0027, ADR 0030). Published as @projectbooth/module-store-ui — see
// this repo's README for the package build/publish setup.
export function ModuleStoreApp({ workspace, role, theme, accessToken }: ModuleStoreAppProps) {
  const [entries, setEntries] = useState<CatalogEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string | null>(null);

  async function reload() {
    try {
      const data = await fetchCatalog(workspace, accessToken);
      setEntries(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  useEffect(() => {
    reload();
    // Only re-fetch when the active workspace or token changes, not on every render —
    // reload itself is redefined each render and intentionally left out of this
    // dependency list.
  }, [workspace, accessToken]);

  const categories = useMemo(() => categoriesOf(entries ?? []), [entries]);
  const filtered = useMemo(() => filterCatalog(entries ?? [], { query, category }), [entries, query, category]);

  return (
    <div data-theme={theme} className="flex flex-col gap-4 p-6">
      {error && <p className="text-sm text-red-600 dark:text-red-400">Couldn't load the Module Store: {error}</p>}
      {!error && entries === null && <p className="text-sm text-slate-500 dark:text-slate-400">Loading modules…</p>}
      {!error && entries !== null && (
        <>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <SearchBar value={query} onChange={setQuery} />
          </div>
          <CategoryFilter categories={categories} selected={category} onChange={setCategory} />
          <CatalogGrid entries={filtered} workspace={workspace} role={role} accessToken={accessToken} onChanged={reload} />
        </>
      )}
    </div>
  );
}
