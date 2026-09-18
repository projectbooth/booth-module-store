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
   *  submitted without the user seeing and confirming it first. */
  suggestedNamespace?: string;
}

/** Caller's role in the active workspace (ADR 0025). Matches
 *  contracts/ui-integration.md's NativeModuleProps contract (ADR 0031). */
export type WorkspaceRole = "owner" | "editor" | "viewer";
