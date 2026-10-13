import { useState } from "react";
import type { CatalogEntry, WorkspaceRole } from "../types";
import { installModule, uninstallModule, type GetAccessToken } from "../api/client";
import { SourceBadge } from "./SourceBadge";
import { StatusBadge } from "./StatusBadge";

type Action = "install" | "uninstall";

// booth-core has no durable "installing"/"failed" desired-state (its own decision
// 0005) — a synchronous install/uninstall call's success or failure IS the result.
// "In progress" is therefore this component's own transient state around that one HTTP
// call, not something the catalog API returns. An install's 202-accepted "Installing"
// state is the exception — that one is tracked by the parent (ARCHITECTURE item 70),
// via the `installing`/`installError` props below, precisely so it survives this
// component re-rendering with a stale catalog reload or unmounting/remounting.
type CardState =
  | { kind: "idle" }
  | { kind: "confirming"; action: Action; namespace: string }
  | { kind: "pending"; action: Action };

export function ModuleCard({
  entry,
  workspace,
  role,
  getAccessToken,
  onChanged,
  installing,
  installError,
  onInstallAccepted,
}: {
  entry: CatalogEntry;
  workspace: string;
  role: WorkspaceRole;
  getAccessToken: GetAccessToken;
  onChanged: () => void;
  /** True once this module's install POST has been accepted (202) and the parent is
   *  polling for it to show up installed (ARCHITECTURE item 70) — distinct from local
   *  `state.kind === "pending"`, which only covers the initial synchronous call. */
  installing: boolean;
  /** Set by the parent if polling gave up waiting for this module to show up installed
   *  (ARCHITECTURE item 70) — shown instead of silently falling back to a bare
   *  "Install" button. */
  installError: string | undefined;
  /** Tells the parent to start tracking this module as installing (ARCHITECTURE item
   *  70) — called once the install POST itself is accepted, before anything confirms
   *  core has actually registered the module yet. */
  onInstallAccepted: () => void;
}) {
  const [state, setState] = useState<CardState>({ kind: "idle" });
  const [localError, setLocalError] = useState<string | null>(null);
  const installable = Boolean(entry.chart?.chartName || entry.chart?.path || entry.chartRef);
  const installed = entry.status.state === "installed";
  const canManage = role === "owner";

  function startConfirm(action: Action) {
    // ARCHITECTURE item 70: never let a second install start for a module this page
    // already sent one for — the button below is disabled while `installing`, but guard
    // the entry point too rather than relying on that alone.
    if (action === "install" && installing) return;

    // ADR 0060: for uninstall, prefer the module's real namespace over the
    // "booth-<id>" guess — an already-installed module may live somewhere else, and
    // pre-filling the guess would let uninstall silently no-op (booth-core treats
    // "not found in that namespace" as success). The guess is the only thing
    // available for install, since nothing real exists there yet.
    const namespace = (action === "uninstall" ? entry.namespace : undefined) ?? entry.suggestedNamespace ?? "";
    setLocalError(null);
    setState({ kind: "confirming", action, namespace });
  }

  // ADR 0029: there is no fleet-wide default install namespace. The installing user
  // must see and confirm (or change) the target namespace here — this is the only
  // path that calls installModule/uninstallModule; neither is ever called with a
  // namespace the user hasn't explicitly seen on screen first.
  async function confirm() {
    if (state.kind !== "confirming") return;
    const { action, namespace } = state;
    if (!namespace.trim()) return;
    if (action === "install" && installing) {
      // ARCHITECTURE item 70: defense in depth, same guard as startConfirm — this
      // module is already mid-install from this page, so this is a no-op, not a second
      // install request that Helm would apply as an upgrade.
      setState({ kind: "idle" });
      return;
    }

    setState({ kind: "pending", action });
    try {
      if (action === "install") {
        await installModule(entry.id, workspace, getAccessToken, namespace.trim());
        // Accepted (202) only — core has not necessarily registered the module yet
        // (ARCHITECTURE item 70: the BoothModule CR this page's next reload depends on
        // appears a few seconds later). Hand off to the parent's poll-with-backoff
        // instead of treating "accepted" as "done" and reloading immediately, which is
        // what used to flip this button back to "Install" before the install had
        // actually finished — and let a second click send a second install that Helm
        // applies as an upgrade.
        onInstallAccepted();
        setState({ kind: "idle" });
      } else {
        await uninstallModule(entry.id, workspace, getAccessToken, namespace.trim());
        onChanged();
        setState({ kind: "idle" });
      }
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : String(err));
      setState({ kind: "idle" });
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <div className="flex items-start justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100">{entry.displayName}</h3>
          {entry.description && <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{entry.description}</p>}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <SourceBadge source={entry.source} />
        <StatusBadge status={entry.status} />
      </div>

      {(localError ?? installError) && (
        <p className="text-xs text-red-600 dark:text-red-400">{localError ?? installError}</p>
      )}

      {!canManage && (
        <p className="text-xs text-slate-400 dark:text-slate-500">Only workspace owners can install or uninstall modules.</p>
      )}

      {canManage && state.kind === "confirming" && (
        <div className="flex flex-col gap-2 rounded-md border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-800">
          <label className="text-xs font-medium text-slate-600 dark:text-slate-300" htmlFor={`namespace-${entry.id}`}>
            Target namespace
          </label>
          <input
            id={`namespace-${entry.id}`}
            type="text"
            value={state.namespace}
            onChange={(e) => setState({ kind: "confirming", action: state.action, namespace: e.target.value })}
            className="rounded-md border border-slate-300 bg-white px-2 py-1 text-sm text-slate-900 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
          />
          <div className="flex gap-2">
            <button
              type="button"
              onClick={confirm}
              disabled={!state.namespace.trim()}
              className="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
            >
              Confirm {state.action === "install" ? "install" : "uninstall"}
            </button>
            <button
              type="button"
              onClick={() => setState({ kind: "idle" })}
              className="rounded-md border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-100 dark:border-slate-600 dark:text-slate-300 dark:hover:bg-slate-700"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {canManage && state.kind !== "confirming" && (
        <div>
          {installed ? (
            <button
              type="button"
              onClick={() => startConfirm("uninstall")}
              disabled={state.kind === "pending"}
              className="rounded-md border border-red-300 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-50 disabled:opacity-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-950"
            >
              {state.kind === "pending" && state.action === "uninstall" ? "Uninstalling…" : "Uninstall"}
            </button>
          ) : (
            <button
              type="button"
              onClick={() => startConfirm("install")}
              disabled={state.kind === "pending" || installing || !installable}
              title={installable ? undefined : "No chart reference published for this module yet"}
              className="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
            >
              {installing || (state.kind === "pending" && state.action === "install") ? "Installing…" : "Install"}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
