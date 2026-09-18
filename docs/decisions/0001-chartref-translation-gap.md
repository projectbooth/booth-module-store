# booth-module-store decision 0001: chartRef shape mismatch between the registry
# protocol and booth-core's actual install API

Status: **resolved and implemented**, per
`../../../booth-architecture/decisions/0028-install-api-accepts-chartref-strings.md`
— `booth-core`'s install API now accepts a raw `chartRef` string directly. This repo's
registry-consuming install path passes a registry's `chartRef`/`chartVersion` straight
through unparsed (`internal/coreclient/client.go`), and the interim parser this doc
originally shipped (`internal/catalog/chartref.go`, `ParseRegistryChartRef`) has been
deleted. Original flag text kept below for context on how the gap was found.

---

Status (original, superseded above): **flagged back to the coordinator, not resolved here** — this repo ships a
narrow, clearly-scoped interim answer (below) rather than guessing at a general one.

## Context

`../../../booth-architecture/contracts/module-registry-protocol.md` says an external
registry's `chartRef` (a single string, e.g.
`"oci://registry.example.com/charts/acme-forecast"`) plus `chartVersion` are:

> passed verbatim to booth-core's existing install API... booth-module-store doesn't
> interpret or validate the chart itself beyond passing it through.

That's not actually possible. Reading `booth-core`'s real implementation
(`internal/api/server.go`'s `installRequest`), `POST /api/modules/{id}/install` has
never accepted a single opaque string — only a structured body:

```json
{
  "namespace": "...",
  "chart": { "path": "...", "repoUrl": "...", "chartName": "...", "version": "..." },
  "values": {}
}
```

There is no `chartRef`-shaped field anywhere in that API. Something has to parse a
registry's `chartRef` string into `repoUrl`/`chartName` (or `path`), and the protocol
document doesn't say how — it was written assuming a pass-through that the actual
booth-core API doesn't support.

## What this repo does about it (v0 interim, not a general answer)

`internal/catalog/chartref.go`'s `ParseRegistryChartRef` understands exactly one shape:
`oci://host/path/chart-name`, splitting the last path segment off as the chart name and
the rest as the repo URL — covering the protocol document's own example and nothing
more. Any other `chartRef` shape (a plain `https://` chart repo URL with no way to
separate repo from chart name, a local path, anything else) is rejected with an error
at parse time rather than guessed at, so a bad assumption fails loudly when an entry is
listed (falls back to "present but not installable" — see `registryclient.go`) rather
than silently installing the wrong chart.

This is intentionally narrow. It unblocks bundled-catalog and native-UI scaffolding
(which don't depend on this at all — see 0002 for why) without pretending the general
question is settled.

## What needs a real decision

- Does `module-registry-protocol.md` need a structured `chart` object (mirroring
  booth-core's actual API) instead of a single `chartRef` string? That would make the
  "verbatim passthrough" framing literally true again.
- Or does `booth-core`'s install API need to grow a single-string chart-reference form
  (e.g. resolving `oci://...` and a repo-relative form itself), so translation happens
  in one place instead of every registry-consuming client re-implementing the same
  parsing?
- Either way, this is a decision that touches both `contracts/module-registry-
  protocol.md` and `booth-core`'s own API surface — not something `booth-module-store`
  should settle unilaterally, per this repo's brief.

## Consequences if left as-is

- Any external registry advertising a chart reference in a form other than
  `oci://host/path/chart-name` is currently listed but not installable through this
  repo's UI until either this parser grows a case for it (itself risky to do
  unilaterally, since guessing at chart-location semantics per hostname/scheme is
  exactly the kind of thing this ADR flags as needing a real decision) or the protocol
  is amended.
