# booth-module-store

The Module Store: the catalog of installable Project Booth modules (a bundled,
offline-capable list plus optional external registry connections) and the
browsing/install UI, calling `booth-core`'s existing install/uninstall API. Mandatory
module — see `../booth-architecture`'s ADR 0027 and `agent-briefs/module-store.md`.

## Stack

- **Backend**: Go + `chi`, matching `booth-core`'s own stack (same OIDC verification
  library, same router, same manifest-as-`BoothModule`-CRD pattern) so a reader already
  familiar with `booth-core` doesn't have to context-switch. No Helm SDK or
  `controller-runtime` dependency — this repo never installs/uninstalls modules itself
  or watches CRDs, it only calls `booth-core`'s API and templates one `BoothModule`
  instance for itself (see `charts/booth-module-store/templates/boothmodule.yaml`).
- **Frontend**: React + TypeScript + Vite + Tailwind, matching `booth-design`'s
  declared stack (ADR 0009) even though that repo doesn't exist yet. `web/src/
  ModuleStoreApp.tsx` is the native-mode component `booth-design`'s shell will
  eventually mount at its reserved "Module Store" slot; `web/src/devshell/` is a
  throwaway mock shell for local development in the meantime (see that file's own
  comment), not something that ships.
- **Deployment**: one Helm chart (`charts/booth-module-store`), per ADR 0003.

## Repo layout

```
cmd/module-store/        entrypoint
internal/catalog/        two-tier catalog: bundled seed data, merge logic, external
                          registry client, chartRef translation
internal/coreclient/     HTTP client for booth-core's install/uninstall/list API
internal/auth/           independent JWT re-verification (defense in depth)
internal/api/            this module's own HTTP surface, reached via booth-core's
                          gateway once installed
charts/booth-module-store/  Helm chart, including this module's own BoothModule
                          manifest registration
web/                      native-mode frontend (see web/README-equivalent comments in
                          ModuleStoreApp.tsx and devshell/DevShell.tsx)
docs/decisions/           two flagged, unresolved cross-cutting questions this repo
                          hit while scaffolding (see below) — not decided here
docs/bundled-catalog-sync.md   proposed (not yet built) approach for keeping the
                          bundled catalog in sync with MODULE_REGISTRY.md over time
test/contract/           validates this repo's own BoothModule manifest against
                          contracts/module-manifest.md via `helm template`
test/integration/        real-cluster (kind) smoke test — see its own README for
                          current scope and what's still blocked on booth-core
```

## Flagged open questions

Two genuine contract/architecture gaps surfaced while building this, documented in
`docs/decisions/` rather than resolved unilaterally here:

- **[0001](docs/decisions/0001-chartref-translation-gap.md)** — `contracts/module-
  registry-protocol.md` says an external registry's `chartRef` is passed "verbatim" to
  `booth-core`'s install API, but that API has no field shaped to accept a single
  string — only structured `path`/`repoUrl`/`chartName`/`version`. This repo ships a
  narrow interim parser (`oci://` references only) rather than guessing at a general
  translation.
- **[0002](docs/decisions/0002-install-namespace-convention.md)** — `booth-core`'s
  install API requires a target namespace, but no contract specifies what namespace a
  module should install into. This repo defaults to `booth-<module-id>`, overridable
  per call, pending a real fleet-wide convention.

## Running locally

Backend:

```
go run ./cmd/module-store
# requires BOOTH_OIDC_ISSUER_URL and BOOTH_OIDC_CLIENT_ID at minimum
```

Frontend dev harness (mock shell, not the real one):

```
cd web
npm install
npm run dev
```

## Testing

Per `contracts/testing-strategy.md` / ADR 0024:

- `go test ./...` — unit + contract tests (layers 1-2). The manifest contract test in
  `test/contract` needs a `helm` binary on `PATH`; it skips itself with a clear message
  if one isn't found (CI installs one).
- `cd web && npm test` — frontend unit/component tests.
- `.github/workflows/integration.yml` — real-cluster (`kind`) smoke test, on merge to
  `main` and nightly. See `test/integration/README.md` for current scope.
