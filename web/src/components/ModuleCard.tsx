import { useState } from "react";
import type { CatalogEntry } from "../types";
import { installModule, uninstallModule } from "../api/client";
import { SourceBadge } from "./SourceBadge";
import { StatusBadge } from "./StatusBadge";

// booth-core has no durable "installing"/"failed" desired-state (its own decision
// 0005) — a synchronous install/uninstall call's success or failure IS the result.
// "In progress" is therefore this component's own transient state around that one HTTP
// call, not something the catalog API returns.
type ActionState = { kind: "idle" } | { kind: "pending" } | { kind: "error"; message: string };

export function ModuleCard({ entry, onChanged }: { entry: CatalogEntry; onChanged: () => void }) {
  const [action, setAction] = useState<ActionState>({ kind: "idle" });
  const installable = entry.chart?.chartName || entry.chart?.path;
  const installed = entry.status.state === "installed";

  async function handleInstall() {
    setAction({ kind: "pending" });
    try {
      await installModule(entry.id);
      onChanged();
      setAction({ kind: "idle" });
    } catch (err) {
      setAction({ kind: "error", message: err instanceof Error ? err.message : String(err) });
    }
  }

  async function handleUninstall() {
    setAction({ kind: "pending" });
    try {
      await uninstallModule(entry.id);
      onChanged();
      setAction({ kind: "idle" });
    } catch (err) {
      setAction({ kind: "error", message: err instanceof Error ? err.message : String(err) });
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

      {action.kind === "error" && <p className="text-xs text-red-600 dark:text-red-400">{action.message}</p>}

      <div>
        {installed ? (
          <button
            type="button"
            onClick={handleUninstall}
            disabled={action.kind === "pending"}
            className="rounded-md border border-red-300 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-50 disabled:opacity-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-950"
          >
            {action.kind === "pending" ? "Uninstalling…" : "Uninstall"}
          </button>
        ) : (
          <button
            type="button"
            onClick={handleInstall}
            disabled={action.kind === "pending" || !installable}
            title={installable ? undefined : "No chart reference published for this module yet"}
            className="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
          >
            {action.kind === "pending" ? "Installing…" : "Install"}
          </button>
        )}
      </div>
    </div>
  );
}
