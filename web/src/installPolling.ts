// Tuning for the install-polling fix (ARCHITECTURE.md item 70): booth-core's install API
// accepts an install request and returns 202 before the module is actually registered —
// the BoothModule CR core's own registry reads from appears a few seconds later, not
// synchronously with the response. Core has no durable "installing" status to query
// (its own decision 0005: a synchronous install call's own success/failure IS the
// result), so the page itself polls its own /api/catalog with backoff until the module
// shows up installed, instead of trusting the very next catalog reload — which is what
// used to flip the Install button back from "Installing" before the install had
// actually finished, and let a second click send a second install that Helm applies as
// an upgrade (the double-install bug seen on Database, Logging and Lakehouse).
export const INSTALL_POLL_INITIAL_DELAY_MS = 2_000;
export const INSTALL_POLL_MAX_DELAY_MS = 20_000;
export const INSTALL_POLL_BACKOFF_FACTOR = 2;
export const INSTALL_POLL_TIMEOUT_MS = 3 * 60 * 1000; // "up to a few minutes" per the bug report

export function nextInstallPollDelay(current: number): number {
  return Math.min(current * INSTALL_POLL_BACKOFF_FACTOR, INSTALL_POLL_MAX_DELAY_MS);
}
