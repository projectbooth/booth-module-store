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

// workspace and accessToken are both required on every call:
// - workspace sets the X-Workspace header booth-core's gateway requires on every
//   authenticated request (ADR 0025 §6) to resolve/forward X-Booth-Workspace
//   downstream to this module's backend.
// - accessToken is attached as `Authorization: Bearer <token>` — booth-core has no
//   cookie/session support at all (ADR 0032); it's the module's own backend
//   (internal/auth/middleware.go) that expects this, unchanged since this repo's own
//   API was always bearer-token-shaped. What was wrong was this client never sending
//   one, relying on cookies that don't exist on booth-core's side. See ADR 0032 and
//   docs/decisions/0004-native-module-access-token-prop.md.
async function request<T>(path: string, workspace: string, accessToken: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("X-Workspace", workspace);
  headers.set("Authorization", `Bearer ${accessToken}`);

  const res = await fetch(BASE + path, { ...init, headers });
  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }
  if (res.status === 204 || res.status === 202) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export function fetchCatalog(workspace: string, accessToken: string): Promise<CatalogEntry[]> {
  return request<CatalogEntry[]>("/catalog", workspace, accessToken);
}

// namespace is required, not optional: ADR 0029 — there is no fleet-wide default
// namespace, so the installing user must have already seen and confirmed (or
// overridden) it before this is ever called. See components/ModuleCard.tsx for
// where that confirmation happens.
export function installModule(id: string, workspace: string, accessToken: string, namespace: string): Promise<void> {
  return request<void>(`/catalog/${encodeURIComponent(id)}/install`, workspace, accessToken, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ namespace }),
  });
}

export function uninstallModule(id: string, workspace: string, accessToken: string, namespace: string): Promise<void> {
  return request<void>(`/catalog/${encodeURIComponent(id)}?namespace=${encodeURIComponent(namespace)}`, workspace, accessToken, {
    method: "DELETE",
  });
}
