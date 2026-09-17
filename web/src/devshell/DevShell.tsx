import { useEffect, useState } from "react";
import { ModuleStoreApp } from "../ModuleStoreApp";

// Stand-in for booth-design's real shell (nav rail + content pane, ARCHITECTURE.md
// §3/§7's "resolved: the booth-design mockup implements a working light/dark toggle").
// This is local-dev scaffolding only — it never ships, and gets deleted once
// booth-design's actual shell exists and mounts ModuleStoreApp for real. It exists so
// this repo can be built and visually checked standalone, per this repo's brief:
// "You can build your catalog logic, bundled-data model, and UI against a mocked
// shell/install-API before either is fully ready."
export function DevShell() {
  const [theme, setTheme] = useState<"light" | "dark">(
    () => (localStorage.getItem("module-store-dev-theme") as "light" | "dark") ?? "light",
  );

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
      <header className="flex items-center justify-between border-b border-slate-200 bg-white px-6 py-3 dark:border-slate-800 dark:bg-slate-900">
        <div>
          <p className="text-xs uppercase tracking-wide text-slate-400">Dev harness — not the real shell</p>
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Module Store</h1>
        </div>
        <button
          type="button"
          onClick={() => setTheme((t) => (t === "light" ? "dark" : "light"))}
          className="rounded-md border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          {theme === "light" ? "Dark mode" : "Light mode"}
        </button>
      </header>
      <main>
        <ModuleStoreApp />
      </main>
    </div>
  );
}
