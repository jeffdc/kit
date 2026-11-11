// Writes: the actions that change mull data, as against the ones that copy
// a command to run. A write button looks different (a heavier box and a
// tick), asks once more before it writes (its first click arms it), is
// pending while the write runs, and a failure stays shown where it happened
// until it's tried again. The state is kept here, by key, so live
// re-renders draw it again.

import { esc } from "./html.js";

const states = new Map(); // key → {armed} | {pending} | {error}

export const writeState = key => states.get(key) || {};
export const setWrite = (key, s) => (s ? states.set(key, s) : states.delete(key));

// disarm drops every armed write but keep; it reports whether any was.
export function disarm(keep = "") {
  let any = false;
  for (const [k, s] of states) if (s.armed && k !== keep) { states.delete(k); any = true; }
  return any;
}

const TICK = `<svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true"><path d="M1.5 5.2 4 7.6 8.6 2.4" fill="none" stroke="currentColor" stroke-width="1.6"/></svg>`;

// writeButton draws a write action. key names it across renders; ops are
// the batch lines it sends; label is what it says, ask what it says once
// armed, and done what the toast says once written.
export function writeButton({ pid, key, ops, label, ask, done, small = true }) {
  const s = writeState(key);
  const cls = `btn wr${small ? " s" : ""}${s.armed ? " armed" : ""}${s.pending ? " pending" : ""}`;
  const text = s.pending ? "Saving…" : s.armed ? ask : label;
  const title = s.armed ? "Click again to save it: it can't be undone. Esc to cancel." : "Saves to mull: asks once more first.";
  return `<button class="${cls}" data-write="${esc(key)}" data-pid="${esc(pid)}" data-ops="${esc(JSON.stringify(ops))}" data-done="${esc(done)}" title="${esc(title)}"${s.pending ? " disabled" : ""}>${TICK}${esc(text)}</button>`;
}

// failure is the line saying why a write didn't save, where it was tried.
export function failure(key, what) {
  const s = writeState(key);
  return s.error ? `<p class="wfail" role="alert"><b>Not saved</b>${esc(what)}: ${esc(s.error)}</p>` : "";
}
