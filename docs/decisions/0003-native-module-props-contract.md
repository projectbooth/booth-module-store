# booth-module-store decision 0003: the native-module mount props contract

Status: **agreed with booth-design's agent, flagged to the coordinator for
`contracts/ui-integration.md`** — not something these two repos should keep as a
private, undocumented handshake, per ADR 0030's own instruction to report back
anything that looks like it needs the contract to say more.

## Context

ADR 0030 confirmed native-mode modules ship their own UI as an npm package that
`booth-design` mounts via `src/lib/nativeModules.ts`, but explicitly left "what props
or shared context (workspace slug, active role, theme) the package should expect to
receive" as something `booth-design` and `booth-module-store` (the first real
consumer) work out together — the same "propose, don't silently decide alone" pattern
used elsewhere in this project.

## What was agreed

Plain React props, not a shared context object — a context would mean this package
importing something `booth-design` exports, inverting the dependency direction ADR
0030 exists to fix (the shell depends on the module's package, never the reverse).

```ts
export interface ModuleStoreAppProps {
  workspace: string; // active workspace slug (ADR 0025)
  role: "owner" | "editor" | "viewer"; // caller's role in that workspace (ADR 0025)
  theme: "dark" | "light";
}
```

- **`workspace`** — required. Every API call this component makes needs it to set the
  `X-Workspace` header booth-core's own auth middleware requires on every
  authenticated request (ADR 0025 §6) — see the consequence below, this wasn't
  optional polish.
- **`role`** — required. Used only to hide install/uninstall actions in this
  component's own UI for non-owners; `booth-core` still independently enforces
  owner-only on the actual install/uninstall calls (ADR 0023), so this is a UX nicety,
  not a security boundary this package is responsible for.
- **`theme`** — included even though this component's CSS-only styling already follows
  the ambient `data-theme` attribute on a document ancestor (the same convention
  `booth-design`'s own `useTheme` hook documents) — kept for any JS-driven decision a
  native module might need. This component also applies it directly to its own root
  element, so it renders correctly even mounted somewhere that hasn't already set that
  attribute on an ancestor.

`NativeModulePane`'s mount call becomes `<Component workspace={...} role={...}
theme={...} />` instead of the zero-prop `<Component />` `booth-design` shipped
first (its own `src/lib/nativeModules.ts` predates this agreement and flagged the gap
itself rather than guessing).

## A real bug this surfaced, not just a formalization

Working this out against the actual code (not just in the abstract) surfaced that
`web/src/api/client.ts` didn't attach an `X-Workspace` header at all — every request
from this component would have been rejected at `booth-core`'s own gateway (400,
missing header) before ever reaching this module's backend, regardless of any prop
contract. Fixed alongside this decision — see `internal/api/server.go`'s and
`web/src/api/client.ts`'s current state. Caught because a peer session working on
`booth-design`'s side of this same contract read this repo's actual client code, not
because either side had tested the two together (there is no integration test covering
this path — worth a `booth-e2e` scenario once that repo exercises native-module
mounting for real, per `contracts/testing-strategy.md`'s layer-4 description).

## What needs a coordinator decision

Whether this exact shape — the three fields above, "props not context" — should move
into `contracts/ui-integration.md` as the standard every future native-mode module
follows, rather than staying an ad hoc agreement between these two repos that a third
native module (`booth-catalog`, `booth-api`, etc.) would have to independently
rediscover or copy. Recommendation: yes, pin it — this is exactly the kind of
cross-cutting shape ADR 0030 itself warned would "compound" if left for each module to
reinvent.
