import { useEffect, useState } from "react";
import { ModuleStoreApp } from "../ModuleStoreApp";
import type { WorkspaceRole } from "../types";

const ROLES: WorkspaceRole[] = ["owner", "editor", "viewer"];

// Stand-in for booth-design's real shell (nav rail + content pane, ARCHITECTURE.md
// §3/§7's "resolved: the booth-design mockup implements a working light/dark toggle").
// This is local-dev scaffolding only — it never ships, and gets deleted once
// booth-design's actual shell exists and mounts ModuleStoreApp for real. It exists so
// this repo can be built and visually checked standalone, per this repo's brief:
// "You can build your catalog logic, bundled-data model, and UI against a mocked
// shell/install-API before either is fully ready."
//
// Also doubles as a manual check of the ModuleStoreAppProps contract itself (ADR
// 0030, docs/decisions/0003-native-module-props-contract.md) — the workspace/role
// selectors below exist so a developer can exercise what a real shell would pass in.
export function DevShell() {
  const [theme, setTheme] = useState<"light" | "dark">(
    () => (localStorage.getItem("module-store-dev-theme") as "light" | "dark") ?? "light",
  );
  const [workspace, setWorkspace] = useState("acme-analytics");
  const [role, setRole] = useState<WorkspaceRole>("owner");

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    try {
      localStorage.setItem("module-store-dev-theme", theme);
    } catch {
      // best-effort only — a private window or blocked storage just resets on reload
    }
  }, [theme]);

  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-white px-6 py-3 dark:border-slate-800 dark:bg-slate-900">
        <div>
          <p className="text-xs uppercase tracking-wide text-slate-400">Dev harness — not the real shell</p>
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Module Store</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <label className="text-xs text-slate-500 dark:text-slate-400">
            Workspace
            <input
              type="text"
              value={workspace}
              onChange={(e) => setWorkspace(e.target.value)}
              className="ml-1.5 w-36 rounded-md border border-slate-300 px-2 py-1 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            />
          </label>
          <label className="text-xs text-slate-500 dark:text-slate-400">
            Role
            <select
              value={role}
              onChange={(e) => setRole(e.target.value as WorkspaceRole)}
              className="ml-1.5 rounded-md border border-slate-300 px-2 py-1 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            onClick={() => setTheme((t) => (t === "light" ? "dark" : "light"))}
            className="rounded-md border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            {theme === "light" ? "Dark mode" : "Light mode"}
          </button>
        </div>
      </header>
      <main>
        <ModuleStoreApp workspace={workspace} role={role} theme={theme} />
      </main>
    </div>
  );
}
