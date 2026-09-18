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

// workspace is required on every call: booth-core's own auth middleware requires the
// client-supplied X-Workspace header on every authenticated request (ADR 0025 §6) to
// resolve/forward X-Booth-Workspace downstream to this module's backend — without it,
// the request never even reaches us, it 400s at booth-core's gateway. Mirrors
// booth-design's own src/lib/api/client.ts convention exactly, for consistency across
// every native module's client.
async function request<T>(path: string, workspace: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("X-Workspace", workspace);

  const res = await fetch(BASE + path, { ...init, headers, credentials: "include" });
  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }
  if (res.status === 204 || res.status === 202) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export function fetchCatalog(workspace: string): Promise<CatalogEntry[]> {
  return request<CatalogEntry[]>("/catalog", workspace);
}

// namespace is required, not optional: ADR 0029 — there is no fleet-wide default
// namespace, so the installing user must have already seen and confirmed (or
// overridden) it before this is ever called. See components/InstallDialog.tsx for
// where that confirmation happens.
export function installModule(id: string, workspace: string, namespace: string): Promise<void> {
  return request<void>(`/catalog/${encodeURIComponent(id)}/install`, workspace, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ namespace }),
  });
}

export function uninstallModule(id: string, workspace: string, namespace: string): Promise<void> {
  return request<void>(`/catalog/${encodeURIComponent(id)}?namespace=${encodeURIComponent(namespace)}`, workspace, {
    method: "DELETE",
  });
}
