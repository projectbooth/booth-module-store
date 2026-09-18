// Public entry point for @projectbooth/module-store-ui (ADR 0030). Anything
// booth-design (or any future consumer) needs is re-exported here — internal
// components/helpers under src/components, src/api, etc. are not part of the public
// API and can change freely.
//
// Consumers must also import this package's stylesheet once
// (`@projectbooth/module-store-ui/dist/style.css`) — see this repo's README for why
// it's a separate import rather than auto-injected.
import "./library.css";

export { ModuleStoreApp } from "./ModuleStoreApp";
export type { ModuleStoreAppProps } from "./ModuleStoreApp";
export type { WorkspaceRole } from "./types";
