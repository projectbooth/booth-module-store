import type { CatalogEntry } from "./types";

// Pure filter/search logic, split out from any component so it's trivially
// unit-testable without a DOM (contracts/testing-strategy.md: catalog/parsing logic
// should be unit-tested in isolation, not left to slide into a real-cluster-only test).
export function filterCatalog(
  entries: CatalogEntry[],
  { query, category }: { query: string; category: string | null },
): CatalogEntry[] {
  const q = query.trim().toLowerCase();

  return entries.filter((e) => {
    if (category && e.category !== category) return false;
    if (!q) return true;
    return (
      e.displayName.toLowerCase().includes(q) ||
      (e.description?.toLowerCase().includes(q) ?? false) ||
      e.id.toLowerCase().includes(q)
    );
  });
}

export function categoriesOf(entries: CatalogEntry[]): string[] {
  const set = new Set<string>();
  for (const e of entries) {
    if (e.category) set.add(e.category);
  }
  return Array.from(set).sort();
}
