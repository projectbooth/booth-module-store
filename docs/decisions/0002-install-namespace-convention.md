# booth-module-store decision 0002: what namespace does a module install into?

Status: **flagged back to the coordinator, not resolved here** — this repo ships a
clearly-labeled interim default rather than deciding a fleet-wide convention
unilaterally.

## Context

`booth-core`'s `POST /api/modules/{id}/install` requires a `namespace` field in its
request body (`internal/api/server.go`'s `installRequest.Namespace`, "required" per
that handler's own validation) — but no contract or ADR in `booth-architecture` says
what namespace a module should actually install into. `ARCHITECTURE.md` and ADR 0003
establish Kubernetes/Helm as the deployment model and note `booth-core` itself runs in
`booth-system` by convention (its chart's own default namespace), but nothing extends
that convention to the modules `booth-core` installs on request.

This matters beyond just "what string to send": namespace choice is also where a real
deployment would eventually want to hang RBAC/network-policy boundaries per module —
exactly the kind of decision that shouldn't be made once, silently, inside the one repo
that happens to be the first caller of the install API.

## What this repo does about it (v0 interim, not a fleet-wide answer)

`internal/api/server.go`'s `defaultNamespace(moduleID)` defaults to `"booth-" +
moduleID` (e.g. installing the catalog entry `storage` targets namespace
`booth-storage`) whenever a caller doesn't specify one explicitly. Both the native
UI's install action and a direct API call can override this via an optional
`namespace` field in the request body / query parameter — see
`internal/api/server.go`'s `mutateRequest` and `web/src/api/client.ts`.

## What needs a real decision

- Should every module deployment use a fixed, predictable namespace convention like
  `booth-<module-id>` (this repo's interim guess), or should a deployment operator
  choose per-install (e.g. for multi-tenancy reasons not yet modeled anywhere in this
  architecture)?
- Does this belong in `contracts/core-platform-api.md` (since `booth-core`'s install
  API is where the requirement actually lives) or a new ADR of its own?
- If a convention like `booth-<module-id>` is adopted fleet-wide, should `booth-core`
  itself default to it when `namespace` is omitted, rather than requiring every caller
  (today just this repo, but potentially a future CLI or GitOps tool) to know and
  repeat the same convention independently?

## Consequences if left as-is

- Every future caller of `booth-core`'s install API (this repo today, potentially
  others later) has to independently reinvent or copy this same default, with no
  shared source of truth — exactly the kind of duplicated assumption that's cheap to
  fix now and expensive once more callers exist.
