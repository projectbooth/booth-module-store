import type { InstallStatus } from "../types";

export function StatusBadge({ status }: { status: InstallStatus }) {
  if (status.state === "not_installed") {
    return (
      <span className="inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-400">
        Not installed
      </span>
    );
  }

  const healthTone =
    status.health === "Healthy"
      ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"
      : "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300";

  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${healthTone}`}>
      Installed{status.health ? ` · ${status.health}` : ""}
    </span>
  );
}
