# booth-module-store decision 0004: accessToken joins the native-module props contract

Status: **agreed with booth-design's agent, flagged to the coordinator for
`contracts/ui-integration.md`** — same "propose, don't silently decide alone" pattern
as `docs/decisions/0003-native-module-props-contract.md`, which ADR 0031 has since
pinned into the contract without this field.

## Context

`booth-design` implemented `booth-core`'s actual auth model for real (ADR 0032):
`booth-core` has no cookie/session support at all, only `Authorization: Bearer
<token>`, obtained via a client-side OIDC PKCE flow and held in memory. ADR 0032's own
consequences section left "the exact mechanism for how a mounted native component
accesses the token `booth-design` obtained... an implementation detail between
`booth-design` and native modules, escalating only if it turns out to need
standardizing."

Wiring `@projectbooth/module-store-ui@0.1.0` into `booth-design` for the first time
surfaced that it does need standardizing right away: this package's own
`web/src/api/client.ts` was sending `credentials: "include"` and no `Authorization`
header at all — the same wrong cookie-based assumption `booth-design`'s own client had
until ADR 0032, mirrored here rather than independently re-derived (see
`docs/decisions/0003`'s own note about mirroring `booth-design`'s convention "for
consistency," which turned out to be consistent with a mistake, not a working pattern).
Every API call from this component would have 401'd against a real `booth-core`.

## What was agreed

Extend `NativeModuleProps` (ADR 0031) with a fourth field, following the same "props
not context" reasoning already established:

```ts
export interface ModuleStoreAppProps {
  workspace: string;
  role: "owner" | "editor" | "viewer";
  theme: "dark" | "light";
  accessToken: string; // new
}
```

`booth-design` passes the in-memory token from its own `src/lib/auth/tokenStore.ts`
(`getAccessToken()`). This package's `web/src/api/client.ts` attaches it as
`Authorization: Bearer ${accessToken}` on every request, replacing the removed
`credentials: "include"` cookie assumption entirely — `booth-core` has no cookie
mechanism to send credentials to.

Note this module's own backend (`internal/auth/middleware.go`) needed no change: it
already read `Authorization: Bearer` correctly, since that's the only shape
`booth-core`'s own middleware has ever accepted. The bug was entirely in this
package's browser-side client not sending one.

## What needs a coordinator decision

Whether `accessToken` should be added to `contracts/ui-integration.md`'s
`NativeModuleProps` shape alongside `workspace`/`role`/`theme` (ADR 0031), the same way
that ADR formalized the first three fields after they were agreed ad hoc between these
two repos. Recommendation: yes — every other native module (`booth-catalog`,
`booth-api`, etc.) will hit the exact same 401 this repo just fixed if the token isn't
part of the documented standard contract from the start.
