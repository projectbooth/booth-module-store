# Real-cluster integration tests (layer 3)

Per `contracts/testing-strategy.md` and ADR 0024. Runs on merge to `main` and nightly
(`.github/workflows/integration.yml`), not on every push — it's the slower "actually
deployed to a real cluster" layer.

## What this covers today

- Our chart deploys into a real `kind` cluster and the pod becomes `Ready`.
- Our chart's `BoothModule` custom resource (`templates/boothmodule.yaml`) is created
  with the expected `spec` fields — verifying our side of ADR 0019's registration
  contract, using a vendored copy of booth-core's CRD (see `fixtures/`, and that file's
  own comment for why it's vendored rather than pulled from a real `booth-core` build).
- `/healthz` responds through the in-cluster Service.

## What this doesn't cover yet, and why

The full cross-service scenario this layer is meant for — `booth-module-store` calling
a real `booth-core`'s install API and watching a module actually get installed
(`agent-briefs/module-store.md`'s testing note: "actually triggering an install
end-to-end... belongs in the real-cluster tier") — isn't wired up yet. It needs a
`booth-core` build this repo's CI can install alongside its own, and
`contracts/testing-strategy.md` is explicit that `booth-core` owes the fleet "a stable,
referenceable build" for exactly this purpose — not yet published as of this scaffold.
Don't build a workaround (e.g. checking out `booth-core`'s source at an arbitrary
commit) for this; wait for that build to exist, per the contract, then wire this layer
up for real.

## A note on the OIDC issuer used in CI

`.github/workflows/integration.yml` points `oidc.issuerUrl` at a real, publicly
reachable OIDC discovery document (not a fake `example.com` URL) purely so
`auth.NewVerifier`'s startup-time discovery call succeeds and the pod becomes healthy —
no real login happens in this test, and `requireAudience` is left off. This is a CI
convenience, not a statement about which identity provider a real deployment should
use (ADR 0004 leaves that pluggable).
