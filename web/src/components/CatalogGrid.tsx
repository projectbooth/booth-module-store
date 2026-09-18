import type { CatalogEntry, WorkspaceRole } from "../types";
import type { GetAccessToken } from "../api/client";
import { ModuleCard } from "./ModuleCard";

export function CatalogGrid({
  entries,
  workspace,
  role,
  getAccessToken,
  onChanged,
}: {
  entries: CatalogEntry[];
  workspace: string;
  role: WorkspaceRole;
  getAccessToken: GetAccessToken;
  onChanged: () => void;
}) {
  if (entries.length === 0) {
    return <p className="py-8 text-center text-sm text-slate-500 dark:text-slate-400">No modules match your search.</p>;
  }

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {entries.map((entry) => (
        <ModuleCard
          key={`${entry.source.kind}:${entry.source.name ?? ""}:${entry.id}`}
          entry={entry}
          workspace={workspace}
          role={role}
          getAccessToken={getAccessToken}
          onChanged={onChanged}
        />
      ))}
    </div>
  );
}
