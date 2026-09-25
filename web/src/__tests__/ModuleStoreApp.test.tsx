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
  suggestedNamespace: "booth-storage",
};

const registryForecast: CatalogEntry = {
  id: "acme-forecast",
  displayName: "Acme Forecast",
  category: "analytics",
  source: { kind: "registry", name: "https://registry.example.com" },
  status: { state: "installed", health: "Healthy" },
  suggestedNamespace: "booth-acme-forecast",
};

function mockFetch(entries: CatalogEntry[]) {
  const fetchMock = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => entries,
    text: async () => "",
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("ModuleStoreApp", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // ADR 0072 regression guard: booth-design's shell now owns outer padding around
  // every native module (NativeModulePane wraps mounted components in its own p-6,
  // shipped in booth-design commit 5929bf9); this component's root must not add its
  // own, or the two double up.
  it("does not add its own outer padding — the shell owns that (ADR 0072)", async () => {
    mockFetch([bundledStorage]);
    const { container } = render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => "test-token"} />);
    await screen.findByText("Storage");

    const root = container.firstElementChild;
    expect(root?.className).not.toMatch(/(^|\s)p-\d/);
  });

  it("loads and renders the catalog", async () => {
    mockFetch([bundledStorage, registryForecast]);
    render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => "test-token"} />);

    expect(await screen.findByText("Storage")).toBeInTheDocument();
    expect(screen.getByText("Acme Forecast")).toBeInTheDocument();
    expect(screen.getByText(/Bundled/)).toBeInTheDocument();
    expect(screen.getByText(/Registry:/)).toBeInTheDocument();
  });

  it("sends the workspace as an X-Workspace header and calls getAccessToken fresh for the Bearer Authorization header", async () => {
    const fetchMock = mockFetch([bundledStorage]);
    const getAccessToken = vi.fn(() => "the-real-token");
    render(<ModuleStoreApp workspace="acme-analytics" role="owner" theme="light" getAccessToken={getAccessToken} />);
    await screen.findByText("Storage");

    expect(getAccessToken).toHaveBeenCalled();
    const [, init] = fetchMock.mock.calls[0];
    const headers = new Headers(init.headers);
    expect(headers.get("X-Workspace")).toBe("acme-analytics");
    expect(headers.get("Authorization")).toBe("Bearer the-real-token");
  });

  // Regression guard: this component is mounted in booth-design's shell, so its calls must
  // go through booth-core's /modules/{id}/* gateway proxy. A bare /api/catalog resolves
  // against booth-core's own API, which has no such route (404).
  it("calls the catalog through booth-core's /modules/module-store gateway prefix, not a bare /api path", async () => {
    const installedStorage: CatalogEntry = { ...bundledStorage, status: { state: "installed", health: "Healthy" } };
    const fetchMock = mockFetch([installedStorage]);
    render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => "test-token"} />);
    await screen.findByText("Storage");

    expect(fetchMock.mock.calls[0][0]).toBe("/modules/module-store/api/catalog");

    await userEvent.click(screen.getByRole("button", { name: "Uninstall" }));
    fetchMock.mockResolvedValueOnce({ ok: true, status: 202, json: async () => undefined, text: async () => "" });
    fetchMock.mockResolvedValueOnce({ ok: true, status: 200, json: async () => [bundledStorage], text: async () => "" });
    await userEvent.click(screen.getByRole("button", { name: "Confirm uninstall" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    const [uninstallUrl, uninstallInit] = fetchMock.mock.calls[1];
    expect(uninstallUrl).toBe("/modules/module-store/api/catalog/storage?namespace=booth-storage");
    expect(uninstallInit.method).toBe("DELETE");
  });

  // ADR 0060 regression guard: a module installed into a namespace other than the
  // "booth-<id>" guess must pre-fill its real namespace on uninstall — pre-filling the
  // guess would let uninstall silently no-op (booth-core treats "not found in that
  // namespace" as success).
  it("pre-fills the real namespace on uninstall, not the booth-<id> guess, when the module lives elsewhere", async () => {
    const installedElsewhere: CatalogEntry = {
      ...bundledStorage,
      status: { state: "installed", health: "Healthy" },
      namespace: "acme-storage-team-3",
    };
    const fetchMock = mockFetch([installedElsewhere]);
    render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => "test-token"} />);
    await screen.findByText("Storage");

    await userEvent.click(screen.getByRole("button", { name: "Uninstall" }));

    const namespaceInput = await screen.findByLabelText("Target namespace");
    expect(namespaceInput).toHaveValue("acme-storage-team-3");

    fetchMock.mockResolvedValueOnce({ ok: true, status: 202, json: async () => undefined, text: async () => "" });
    fetchMock.mockResolvedValueOnce({ ok: true, status: 200, json: async () => [installedElsewhere], text: async () => "" });
    await userEvent.click(screen.getByRole("button", { name: "Confirm uninstall" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    expect(fetchMock.mock.calls[1][0]).toBe("/modules/module-store/api/catalog/storage?namespace=acme-storage-team-3");
  });

  it("omits the Authorization header entirely when getAccessToken returns null, rather than sending the literal string (ADR 0033)", async () => {
    const fetchMock = mockFetch([bundledStorage]);
    render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => null} />);
    await screen.findByText("Storage");

    const [, init] = fetchMock.mock.calls[0];
    const headers = new Headers(init.headers);
    expect(headers.has("Authorization")).toBe(false);
  });

  it("filters by search query", async () => {
    mockFetch([bundledStorage, registryForecast]);
    render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => "test-token"} />);
    await screen.findByText("Storage");

    await userEvent.type(screen.getByLabelText("Search modules"), "forecast");

    expect(screen.queryByText("Storage")).not.toBeInTheDocument();
    expect(screen.getByText("Acme Forecast")).toBeInTheDocument();
  });

  it("shows an error state when the catalog fails to load", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 502, text: async () => "bad gateway" }));
    render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => "test-token"} />);

    await waitFor(() => expect(screen.getByText(/Couldn't load the Module Store/)).toBeInTheDocument());
  });

  it("hides install/uninstall actions for non-owner roles", async () => {
    mockFetch([bundledStorage]);
    render(<ModuleStoreApp workspace="acme" role="viewer" theme="light" getAccessToken={() => "test-token"} />);
    await screen.findByText("Storage");

    expect(screen.queryByRole("button", { name: "Install" })).not.toBeInTheDocument();
    expect(screen.getByText(/Only workspace owners can install or uninstall modules/)).toBeInTheDocument();
  });

  it("requires confirming a namespace before calling install (ADR 0029)", async () => {
    const fetchMock = mockFetch([bundledStorage]);
    render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => "test-token"} />);
    await screen.findByText("Storage");

    await userEvent.click(screen.getByRole("button", { name: "Install" }));

    // Clicking "Install" opens a confirmation step, pre-filled with the suggested
    // namespace, rather than calling the install API immediately.
    const namespaceInput = await screen.findByLabelText("Target namespace");
    expect(namespaceInput).toHaveValue("booth-storage");
    expect(fetchMock).toHaveBeenCalledTimes(1); // only the initial catalog GET so far

    fetchMock.mockResolvedValueOnce({ ok: true, status: 202, json: async () => undefined, text: async () => "" });
    fetchMock.mockResolvedValueOnce({ ok: true, status: 200, json: async () => [bundledStorage], text: async () => "" });

    await userEvent.click(screen.getByRole("button", { name: "Confirm install" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3)); // + install POST + catalog reload

    const [installUrl, installInit] = fetchMock.mock.calls[1];
    expect(installUrl).toBe("/modules/module-store/api/catalog/storage/install");
    expect(JSON.parse(installInit.body)).toEqual({ namespace: "booth-storage" });
  });

  it("lets the user change the namespace before confirming install", async () => {
    const fetchMock = mockFetch([bundledStorage]);
    render(<ModuleStoreApp workspace="acme" role="owner" theme="light" getAccessToken={() => "test-token"} />);
    await screen.findByText("Storage");

    await userEvent.click(screen.getByRole("button", { name: "Install" }));
    const namespaceInput = await screen.findByLabelText("Target namespace");
    await userEvent.clear(namespaceInput);
    await userEvent.type(namespaceInput, "team-3-storage");

    fetchMock.mockResolvedValueOnce({ ok: true, status: 202, json: async () => undefined, text: async () => "" });
    fetchMock.mockResolvedValueOnce({ ok: true, status: 200, json: async () => [bundledStorage], text: async () => "" });
    await userEvent.click(screen.getByRole("button", { name: "Confirm install" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    const [, installInit] = fetchMock.mock.calls[1];
    expect(JSON.parse(installInit.body)).toEqual({ namespace: "team-3-storage" });
  });
});
