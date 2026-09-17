import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ModuleStoreApp } from "../ModuleStoreApp";
import type { CatalogEntry } from "../types";

const bundledStorage: CatalogEntry = {
  id: "storage",
  displayName: "Storage",
  description: "Object storage backends",
  category: "data",
  chart: { repoUrl: "oci://registry.example.com/charts", chartName: "storage", version: "1.0.0" },
  source: { kind: "bundled" },
  status: { state: "not_installed" },
};

const registryForecast: CatalogEntry = {
  id: "acme-forecast",
  displayName: "Acme Forecast",
  category: "analytics",
  source: { kind: "registry", name: "https://registry.example.com" },
  status: { state: "installed", health: "Healthy" },
};

function mockFetchOnce(entries: CatalogEntry[]) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => entries,
      text: async () => "",
    }),
  );
}

describe("ModuleStoreApp", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("loads and renders the catalog", async () => {
    mockFetchOnce([bundledStorage, registryForecast]);
    render(<ModuleStoreApp />);

    expect(await screen.findByText("Storage")).toBeInTheDocument();
    expect(screen.getByText("Acme Forecast")).toBeInTheDocument();
    expect(screen.getByText(/Bundled/)).toBeInTheDocument();
    expect(screen.getByText(/Registry:/)).toBeInTheDocument();
  });

  it("filters by search query", async () => {
    mockFetchOnce([bundledStorage, registryForecast]);
    render(<ModuleStoreApp />);
    await screen.findByText("Storage");

    await userEvent.type(screen.getByLabelText("Search modules"), "forecast");

    expect(screen.queryByText("Storage")).not.toBeInTheDocument();
    expect(screen.getByText("Acme Forecast")).toBeInTheDocument();
  });

  it("shows an error state when the catalog fails to load", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: false, status: 502, text: async () => "bad gateway" }),
    );
    render(<ModuleStoreApp />);

    await waitFor(() => expect(screen.getByText(/Couldn't load the Module Store/)).toBeInTheDocument());
  });
});
