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
  declared stack (ADR 0009). `web/src/ModuleStoreApp.tsx` is the native-mode component
  `booth-design`'s shell mounts at its reserved "Module Store" slot, published as the
  npm package `@projectbooth/module-store-ui` per ADR 0030 (see "Publishing the UI
  package" below). `web/src/devshell/` is a throwaway mock shell for local development
  only (see that file's own comment), not something that ships.
- **Deployment**: one Helm chart (`charts/booth-module-store`), per ADR 0003.

## The native-module mount contract (ADR 0030/0031/0033)

`ModuleStoreApp` takes four required props, all pinned into `contracts/ui-integration.md`:

```ts
interface ModuleStoreAppProps {
  workspace: string; // active workspace slug (ADR 0025) — required for every API call
  role: "owner" | "editor" | "viewer"; // gates install/uninstall actions in this UI only
  theme: "dark" | "light";
  getAccessToken: () => string | null; // booth-design's current bearer token (ADR 0032's
                        // OIDC PKCE flow), or null if not yet authenticated / logged out.
                        // Called fresh immediately before every request, never cached —
                        // a plain value prop would go stale across a silent token
                        // refresh with no render guaranteed to catch it (ADR 0033).
}
```

Consumers must also import `@projectbooth/module-store-ui/dist/style.css` once — this
package's own stylesheet ships without Tailwind's base/reset layer deliberately (a
second global reset from a dependency would fight with the host shell's own), so it's
components/utilities-only and additive.

## Repo layout

```
cmd/module-store/        entrypoint
internal/catalog/        two-tier catalog: bundled seed data, merge logic, external
                          registry client (chartRef passed through unparsed, ADR 0028)
internal/coreclient/     HTTP client for booth-core's install/uninstall/list API
internal/auth/           independent JWT re-verification (defense in depth)
internal/api/            this module's own HTTP surface, reached via booth-core's
                          gateway once installed
charts/booth-module-store/  Helm chart, including this module's own BoothModule
                          manifest registration
web/                      native-mode frontend (see web/README-equivalent comments in
                          ModuleStoreApp.tsx and devshell/DevShell.tsx)
docs/decisions/           flagged cross-cutting questions this repo hit while
                          building (see below) — all now resolved by an
                          architecture-level ADR
docs/bundled-catalog-sync.md   proposed (not yet built) approach for keeping the
                          bundled catalog in sync with MODULE_REGISTRY.md over time
test/contract/           validates this repo's own BoothModule manifest against
                          contracts/module-manifest.md via `helm template`
test/integration/        real-cluster (kind) smoke test — see its own README for
                          current scope and what's still blocked on booth-core
```

## Flagged questions and their resolution

- **[0001](docs/decisions/0001-chartref-translation-gap.md)** — the `chartRef`
  string-vs-structured-object mismatch between `contracts/module-registry-protocol.md`
  and `booth-core`'s real install API. **Resolved** by ADR 0028: `booth-core` now
  accepts a raw `chartRef` string directly. This repo's own interim parser
  (`internal/catalog/chartref.go`) has been retired — registry entries pass
  `chartRef`/`chartVersion` straight through unparsed (`internal/coreclient`).
- **[0002](docs/decisions/0002-install-namespace-convention.md)** — no contract
  specified what namespace a module installs into. **Resolved** by ADR 0029: there is
  deliberately no fleet-wide default, ever — every caller must supply one explicitly.
  This repo's own silent `booth-<module-id>` fallback has been removed accordingly
  (see "Namespace confirmation" below).
- **[0003](docs/decisions/0003-native-module-props-contract.md)** — the concrete
  native-module mount props contract (workspace/role/theme), agreed with
  `booth-design`'s agent per ADR 0030. **Resolved** by ADR 0031: pinned into
  `contracts/ui-integration.md` as the standard every native module follows.
- **[0004](docs/decisions/0004-native-module-access-token-prop.md)** — a first (wrong)
  proposal to add `accessToken: string`. **Resolved, differently than proposed**, by
  ADR 0033: after wiring this package into `booth-design` against a real `booth-core`
  (which has no cookie/session support at all, ADR 0032) hit a 401, the fix that
  landed is `getAccessToken: () => string | null` — a callback, not a value, so it
  can't go stale across a silently-refreshed token.

## Role derivation from the token (ADR 0041)

`internal/auth` re-verifies the bearer token itself and derives the caller's workspace
role from the token's own groups claim (`role.go`) rather than trusting the
gateway-forwarded `X-Booth-Role` header, which anyone reaching the pod directly could
forge alongside a valid low-privilege token. If the header claims a role stronger than
the token grants for that workspace (or isn't a role at all), the request is rejected
with a 403; a weaker header is honored (a gateway may narrow, never widen), and an
absent one falls back to the token's role. A token granting no role in the requested
workspace is a 403 too.

The claim name is deployment config — `BOOTH_OIDC_GROUPS_CLAIM` / the chart's
`oidc.groupsClaim`, default `groups` — and must match `booth-core`'s own setting.

For accuracy about scope: this module's routes don't currently make any decision *from*
that role — install/uninstall forward the caller's own token to `booth-core`, which
enforces owner-only itself. So no route here was exploitable through a forged header
before this change; the derivation makes the identity's `Role` trustworthy for any
future use and satisfies the contract's requirement regardless.

## Namespace confirmation (ADR 0029/0060)

There is no fleet-wide default install namespace, by design — `internal/api/server.go`
requires `namespace` explicitly on every install/uninstall call (400 if omitted), and
the native UI never calls either without the user first seeing and confirming (or
changing) a value: clicking "Install"/"Uninstall" opens an inline confirmation step
pre-filled from the catalog response, not a silent default.

Two different pre-fill sources, for two different reasons (ADR 0060): **install**
pre-fills `suggestedNamespace` (a guess, `booth-<module-id>`) — nothing real exists yet
to know a better answer. **Uninstall** prefers `namespace` (a fact, from booth-core's
registry) when the module is actually installed, falling back to `suggestedNamespace`
only if booth-core hasn't supplied it. Pre-filling the guess for an already-installed
module was a real bug, found live: booth-core's uninstall treats "not found in that
namespace" as success (correct idempotency on its own), so a wrong guess uninstalled
nothing and reported success with no error anywhere.

## Publishing the UI package

`web/` is both the publishable package (`@projectbooth/module-store-ui`) and its own
dev harness — `npm run dev` serves the harness (`src/main.tsx` → `src/devshell/`),
`npm run build` builds the library (`src/index.ts` as the entry, React/ReactDOM
externalized as peer dependencies so `booth-design`'s own copies are used instead of
bundling a second one). `.github/workflows/publish.yml` publishes to GitHub Packages
(`npm.pkg.github.com`) on a `module-store-ui-v*` tag or manual dispatch.

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
