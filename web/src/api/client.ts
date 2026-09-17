import type { CatalogEntry } from "../types";

// Relative paths: in a real deployment, this component is mounted by booth-design's
// shell and its requests reach booth-core's gateway at
// /modules/module-store/api/... (contracts/ui-integration.md's native-mode pattern),
// which strips the /modules/module-store prefix before forwarding here. The dev
// harness's Vite proxy (vite.config.ts) makes the same relative paths work standalone.
const BASE = "/api";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, init);
  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }
  if (res.status === 204 || res.status === 202) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export function fetchCatalog(): Promise<CatalogEntry[]> {
  return request<CatalogEntry[]>("/catalog");
}

export function installModule(id: string, namespace?: string): Promise<void> {
  return request<void>(`/catalog/${encodeURIComponent(id)}/install`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(namespace ? { namespace } : {}),
  });
}

export function uninstallModule(id: string, namespace?: string): Promise<void> {
  const query = namespace ? `?namespace=${encodeURIComponent(namespace)}` : "";
  return request<void>(`/catalog/${encodeURIComponent(id)}${query}`, {
    method: "DELETE",
  });
}
