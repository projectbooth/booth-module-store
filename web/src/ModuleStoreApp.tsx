import { useEffect, useMemo, useState } from "react";
import type { CatalogEntry } from "./types";
import { fetchCatalog } from "./api/client";
import { categoriesOf, filterCatalog } from "./catalogFilter";
import { SearchBar } from "./components/SearchBar";
import { CategoryFilter } from "./components/CategoryFilter";
import { CatalogGrid } from "./components/CatalogGrid";

// The native-mode component booth-design's shell mounts at its reserved "Module
// Store" slot (ADR 0027, contracts/ui-integration.md). Deliberately has no props of
// its own today — it authenticates the same way any native-mode module's calls do,
// via the browser's existing session through booth-core's gateway — so booth-design
// can mount it with no wiring beyond rendering it at that slot.
export function ModuleStoreApp() {
  const [entries, setEntries] = useState<CatalogEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string | null>(null);

  async function reload() {
    try {
      const data = await fetchCatalog();
      setEntries(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  useEffect(() => {
    reload();
  }, []);

  const categories = useMemo(() => categoriesOf(entries ?? []), [entries]);
  const filtered = useMemo(() => filterCatalog(entries ?? [], { query, category }), [entries, query, category]);

  if (error) {
    return <p className="p-6 text-sm text-red-600 dark:text-red-400">Couldn't load the Module Store: {error}</p>;
  }

  if (entries === null) {
    return <p className="p-6 text-sm text-slate-500 dark:text-slate-400">Loading modules…</p>;
  }

  return (
    <div className="flex flex-col gap-4 p-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SearchBar value={query} onChange={setQuery} />
      </div>
      <CategoryFilter categories={categories} selected={category} onChange={setCategory} />
      <CatalogGrid entries={filtered} onChanged={reload} />
    </div>
  );
}
