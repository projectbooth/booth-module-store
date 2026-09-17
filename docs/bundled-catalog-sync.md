# Keeping the bundled catalog in sync with MODULE_REGISTRY.md

Per ADR 0027's consequences and `agent-briefs/module-store.md`'s second open question:
this is a manual step today, flagged as v1 automation opportunity — not solved, not
blocking v0. This document proposes an approach for whenever that automation is worth
building; it doesn't implement one.

## Current state (v0, manual)

`internal/catalog/bundled.yaml` is hand-authored, seeded once from every `module`-type
row in `../booth-architecture/MODULE_REGISTRY.md` as of 2026-09-17 (excluding the
mandatory/foundation/meta rows — `booth-architecture`, `booth-design`, `booth-core`,
`booth-module-store` itself, `booth-e2e` — since those aren't things a user installs
from the Module Store). Every field beyond `id`/`displayName` (`category`, `icon`,
`manifestPreview`) is this repo's own best-effort classification, not yet confirmed by
each module's own agent — expected and fine per `module-registry-protocol.md`'s
"informational only" framing, but worth each module agent double-checking once their
own manifest is real.

As new modules are added to `MODULE_REGISTRY.md`, or an existing module publishes its
first real chart reference, `bundled.yaml` needs a matching hand-edit. Nothing enforces
this today.

## Proposed v1 approach

`MODULE_REGISTRY.md` is a Markdown table meant for human/coordinator reading, not
machine parsing — turning this repo's sync job into "scrape that table" would be
brittle (a wording tweak breaks the scraper) and couples this repo to
`booth-architecture`'s prose formatting.

A more robust shape, if/when this is worth building:

1. Each module repo's own Helm chart, once it exists, is the real source of truth for
   its chart reference/version (it already has to define these to be installable at
   all).
2. A lightweight, structured `module.yaml` (id, displayName, icon, description,
   category) could live in each module repo alongside its chart — the same
   free-form-metadata idea `ARCHITECTURE.md` §6 already carries forward from
   `OpenDataPlatform` as "a workable, low-ceremony way... to render a catalog without
   hardcoding per-module knowledge," applied here to this repo's *bundled* tier
   specifically rather than a live registry call.
3. A small CI job in this repo (or a scheduled one, since it needs to reach across
   repos) fetches each module repo's `module.yaml` + published chart reference and
   regenerates `internal/catalog/bundled.yaml`, opening a PR here rather than writing
   directly to `main` — keeping a human in the loop for review, consistent with this
   architecture's general preference for explicit, reviewable steps over silent
   automation.

## Why this isn't built now

No module repo has a chart or `module.yaml`-equivalent yet (`MODULE_REGISTRY.md`:
every module status is "not started" except `booth-core`). Building step 3's
automation ahead of step 1/2 existing anywhere would have nothing real to sync from.
Revisit once the first non-mandatory module (per `MODULE_REGISTRY.md`'s suggested build
order, likely `booth-storage`) actually publishes a chart.
