// Mirrors internal/catalog/types.go's JSON shape — kept as a hand-written type rather
// than generated, since this repo has no shared-schema tooling yet. Keep these two in
// sync by hand until that changes.

export type SourceKind = "bundled" | "registry";

export interface Source {
  kind: SourceKind;
  name?: string;
}

export type InstallState = "not_installed" | "installed";

export interface InstallStatus {
  state: InstallState;
  health?: string;
}

export interface ManifestPreview {
  hasOwnUi?: boolean;
  uiIntegrationMode?: "native" | "iframe-proxy";
  navGroup?: "build" | "view" | "manage";
}

export interface CatalogEntry {
  id: string;
  displayName: string;
  icon?: string;
  description?: string;
  category?: string;
  chart?: {
    path?: string;
    repoUrl?: string;
    chartName?: string;
    version?: string;
  };
  /** A registry entry's chart location, passed through verbatim/unparsed (ADR 0028)
   *  — set instead of `chart` for registry-sourced entries. */
  chartRef?: string;
  chartVersion?: string;
  manifestPreview?: ManifestPreview;
  source: Source;
  status: InstallStatus;
  /** Pre-fill hint for an install/uninstall confirmation UI only (ADR 0029) — never
   *  submitted without the user seeing and confirming it first. A guess
   *  ("booth-<id>"), not a fact — the only thing available before install, and the
   *  fallback after install if `namespace` below is somehow still absent. */
  suggestedNamespace?: string;
  /** The module's real install namespace, from booth-core's registry (ADR 0060) —
   *  present when installed and booth-core supports it. A fact, not a guess: the
   *  confirm-uninstall pre-fill must prefer this over `suggestedNamespace` — using the
   *  guess for an already-installed module can pre-fill the wrong namespace, and
   *  booth-core's uninstall treats "not found in that namespace" as success, so the
   *  wrong guess silently removes nothing instead of erroring. */
  namespace?: string;
}

/** Caller's role in the active workspace (ADR 0025). Matches
 *  contracts/ui-integration.md's NativeModuleProps contract (ADR 0031). */
export type WorkspaceRole = "owner" | "editor" | "viewer";
