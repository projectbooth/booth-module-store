import { describe, expect, it } from "vitest";
import { categoriesOf, filterCatalog } from "../catalogFilter";
import type { CatalogEntry } from "../types";

function entry(overrides: Partial<CatalogEntry>): CatalogEntry {
  return {
    id: "storage",
    displayName: "Storage",
    category: "data",
    source: { kind: "bundled" },
    status: { state: "not_installed" },
    ...overrides,
  };
}

describe("filterCatalog", () => {
  it("returns everything for an empty query and no category", () => {
    const entries = [entry({ id: "a" }), entry({ id: "b" })];
    expect(filterCatalog(entries, { query: "", category: null })).toHaveLength(2);
  });

  it("matches displayName case-insensitively", () => {
    const entries = [entry({ id: "storage", displayName: "Storage" }), entry({ id: "catalog", displayName: "Catalog" })];
    const result = filterCatalog(entries, { query: "STOR", category: null });
    expect(result.map((e) => e.id)).toEqual(["storage"]);
  });

  it("matches on description and id too", () => {
    const entries = [entry({ id: "spark", displayName: "Spark", description: "Compute engine" })];
    expect(filterCatalog(entries, { query: "compute", category: null })).toHaveLength(1);
    expect(filterCatalog(entries, { query: "spark", category: null })).toHaveLength(1);
    expect(filterCatalog(entries, { query: "nope", category: null })).toHaveLength(0);
  });

  it("filters by category", () => {
    const entries = [entry({ id: "a", category: "data" }), entry({ id: "b", category: "build" })];
    expect(filterCatalog(entries, { query: "", category: "build" }).map((e) => e.id)).toEqual(["b"]);
  });

  it("combines category and query", () => {
    const entries = [
      entry({ id: "a", category: "data", displayName: "Storage" }),
      entry({ id: "b", category: "data", displayName: "Catalog" }),
    ];
    expect(filterCatalog(entries, { query: "cat", category: "data" }).map((e) => e.id)).toEqual(["b"]);
  });
});

describe("categoriesOf", () => {
  it("returns unique, sorted categories", () => {
    const entries = [entry({ category: "view" }), entry({ category: "build" }), entry({ category: "build" }), entry({ category: undefined })];
    expect(categoriesOf(entries)).toEqual(["build", "view"]);
  });
});
