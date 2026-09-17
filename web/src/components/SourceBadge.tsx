import type { Source } from "../types";

// Stand-in for a badge/tag primitive from booth-design's component library, which
// doesn't exist yet. Kept as a thin, self-contained component so swapping it for the
// real one later is a one-file change (ADR 0027: "showing each entry's source... so a
// user isn't confused about provenance").
export function SourceBadge({ source }: { source: Source }) {
  const label = source.kind === "bundled" ? "Bundled" : `Registry: ${source.name ?? "unknown"}`;
  const tone = source.kind === "bundled" ? "bg-slate-100 text-slate-700" : "bg-indigo-100 text-indigo-700";

  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium dark:bg-slate-800 dark:text-slate-300 ${tone}`}
      title={source.kind === "registry" ? source.name : undefined}
    >
      {label}
    </span>
  );
}
