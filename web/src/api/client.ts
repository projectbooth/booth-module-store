import type { CatalogEntry } from "../types";

// This component is mounted by booth-design's shell, so its requests resolve against the
// shell's origin and must go through booth-core's gateway at /modules/{id}/* — the same
// proxy every module's own backend is reached through — which strips the
// /modules/module-store prefix before forwarding to this repo's own backend routes
// (/api/catalog...). A bare "/api/..." here would instead hit booth-core's own API,
// which has no /api/catalog route at all. The dev harness's Vite proxy (vite.config.ts)
// mimics the same prefix-stripping so the identical paths work standalone.
const BASE = "/modules/module-store/api";

export type GetAccessToken = () => string | null;

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

// workspace sets the X-Workspace header booth-core's gateway requires on every
// authenticated request (ADR 0025 §6) to resolve/forward X-Booth-Workspace downstream
// to this module's backend.
//
// getAccessToken is called fresh immediately before each request, not read once and
// cached — booth-design's token can be silently renewed at any time (ADR 0032), and a
// value captured earlier can go stale with no guarantee a re-render would refresh it.
// A null return (not-yet-authenticated boot window, logged out) omits the
// Authorization header entirely rather than sending the literal string "null" (ADR
// 0033). booth-core has no cookie/session support at all — there is no fallback
// credentials mode to lean on if the token is missing.
async function request<T>(path: string, workspace: string, getAccessToken: GetAccessToken, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("X-Workspace", workspace);
  const token = getAccessToken();
  if (token !== null) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const res = await fetch(BASE + path, { ...init, headers });
  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }
  if (res.status === 204 || res.status === 202) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export function fetchCatalog(workspace: string, getAccessToken: GetAccessToken): Promise<CatalogEntry[]> {
  return request<CatalogEntry[]>("/catalog", workspace, getAccessToken);
}

// namespace is required, not optional: ADR 0029 — there is no fleet-wide default
// namespace, so the installing user must have already seen and confirmed (or
// overridden) it before this is ever called. See components/ModuleCard.tsx for
// where that confirmation happens.
export function installModule(id: string, workspace: string, getAccessToken: GetAccessToken, namespace: string): Promise<void> {
  return request<void>(`/catalog/${encodeURIComponent(id)}/install`, workspace, getAccessToken, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ namespace }),
  });
}

export function uninstallModule(id: string, workspace: string, getAccessToken: GetAccessToken, namespace: string): Promise<void> {
  return request<void>(`/catalog/${encodeURIComponent(id)}?namespace=${encodeURIComponent(namespace)}`, workspace, getAccessToken, {
    method: "DELETE",
  });
}
