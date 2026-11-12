// Writes: the actions that change mull data, as against the ones that copy
// a command to run. A write button looks different (a heavier box and a
// tick), asks once more before it writes (its first click arms it), is
// pending while the write runs, and a failure stays shown where it happened
// until it's tried again. A write that takes text (an answer, a decision,
// a reason), or edits text already there (a title, notes, a doc), opens a
// form in place instead of arming: opening it is the asking once more, and
// its Save writes. The state, drafts included, is kept here by key, so
// live re-renders draw it again.

import { esc } from "./html.js";

const states = new Map(); // key → {armed} | {pending} | {error}, or for a text write {open, draft, pending?, error?}, and for an edit also {base, value, required}

export const writeState = key => states.get(key) || {};
export const setWrite = (key, s) => (s ? states.set(key, s) : states.delete(key));

// disarm drops every armed write but keep, and closes every open form
// that isn't saving (its draft is kept for next time); it reports whether
// anything changed.
export function disarm(keep = "") {
  let any = false;
  for (const [k, s] of states) {
    if (k === keep) continue;
    if (s.armed) { states.delete(k); any = true; }
    if (s.open && !s.pending) { s.open = false; s.error = ""; any = true; }
  }
  return any;
}

export const TICK = `<svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true"><path d="M1.5 5.2 4 7.6 8.6 2.4" fill="none" stroke="currentColor" stroke-width="1.6"/></svg>`;

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

// textWrite draws a write that takes text: the button that opens its form,
// and the form (empty while closed), for the caller to place. ops are the
// batch lines it sends, with the text set as field on each; label opens
// the form, save is its submit, placeholder says what to write, and done
// is what the toast says once written.
export function textWrite({ pid, key, ops, field, label, save, placeholder, done, small = true }) {
  const s = writeState(key);
  const button = `<button class="btn wr${small ? " s" : ""}${s.open ? " armed" : ""}" data-form-open="${esc(key)}" aria-expanded="${s.open ? "true" : "false"}" title="Saves to mull: write it, then save.">${TICK}${esc(label)}</button>`;
  if (!s.open) return { button, form: "" };
  const draft = s.draft || "";
  const form = `<form class="wform" data-form="${esc(key)}" data-pid="${esc(pid)}" data-ops="${esc(JSON.stringify(ops))}" data-field="${esc(field)}" data-done="${esc(done)}">` +
    `<textarea data-draft="${esc(key)}" rows="2" placeholder="${esc(placeholder)}"${s.pending ? " readonly" : ""}>${esc(draft)}</textarea>` +
    `<div class="wform-acts"><button type="submit" class="btn wr s${s.pending ? " pending" : ""}"${s.pending || !draft.trim() ? " disabled" : ""}>${TICK}${esc(s.pending ? "Saving…" : save)}</button>` +
    `<button type="button" class="btn s" data-form-close="${esc(key)}"${s.pending ? " disabled" : ""}>Cancel</button><span class="sub">⌘↵ saves · Esc closes, keeping what you wrote</span></div>` +
    (s.error ? `<p class="wfail" role="alert"><b>Not saved</b>${esc(s.error)}</p>` : "") + `</form>`;
  return { button, form };
}

// editWrite draws a write that edits text already there (a title, notes,
// a doc): textWrite's button and form, the form opening on the text as it
// is. value is that text as the page shows it now; long gives a roomy
// editor that grows with what's in it (notes and docs, which are
// markdown), else it's one line, which Enter saves (a title). required
// keeps Save off while it's empty; Save is off while nothing's changed.
// The draft is kept, by key, until it's saved or discarded: while closed,
// the button says there's one; a draft no one has touched follows the
// text as it changes; and if the text changes under a touched draft, the
// form says so before Save replaces it.
export function editWrite({ pid, key, ops, field, value, label, save, placeholder, done, long = false, required = false, small = true }) {
  const s = writeState(key);
  value = value || "";
  if (states.has(key)) {
    s.value = value;
    s.required = required;
    if (s.draft === undefined || (s.draft === s.base && !s.pending)) s.draft = s.base = value;
  }
  const dirty = s.draft !== undefined && s.draft.trim() !== value.trim();
  const draftNote = !s.open && dirty ? `<span class="wdraft">draft</span>` : "";
  const button = `<button class="btn wr${small ? " s" : ""}${s.open ? " armed" : ""}" data-form-open="${esc(key)}" aria-expanded="${s.open ? "true" : "false"}" title="${dirty ? "Saves to mull: your unsaved draft is kept." : "Saves to mull: edit it, then save."}">${TICK}${esc(label)}${draftNote}</button>`;
  if (!s.open) return { button, form: "" };
  const draft = s.draft;
  const ro = s.pending ? " readonly" : "";
  const input = long
    ? `<textarea data-draft="${esc(key)}" data-grow rows="10" placeholder="${esc(placeholder)}"${ro}>${esc(draft)}</textarea>`
    : `<input type="text" data-draft="${esc(key)}" value="${esc(draft)}" placeholder="${esc(placeholder)}" autocomplete="off"${ro}>`;
  const moved = dirty && s.base !== value
    ? `<p class="wnote">Changed since you began this draft. Saving replaces what's there now with your draft; Discard changes starts again from it.</p>` : "";
  const form = `<form class="wform edit ${long ? "long" : "line"}" data-form="${esc(key)}" data-pid="${esc(pid)}" data-ops="${esc(JSON.stringify(ops))}" data-field="${esc(field)}" data-done="${esc(done)}">` +
    moved + input +
    `<div class="wform-acts"><button type="submit" class="btn wr s${s.pending ? " pending" : ""}"${s.pending || !canSave(key, draft) ? " disabled" : ""}>${TICK}${esc(s.pending ? "Saving…" : save)}</button>` +
    `<button type="button" class="btn s" data-form-close="${esc(key)}"${s.pending ? " disabled" : ""}>Cancel</button>` +
    (dirty && !s.pending ? `<button type="button" class="btn s" data-form-discard="${esc(key)}">Discard changes</button>` : "") +
    `<span class="sub">${long ? "Markdown · ⌘↵ saves" : "↵ saves"} · Esc closes, keeping your draft</span></div>` +
    (s.error ? `<p class="wfail" role="alert"><b>Not saved</b>${esc(s.error)}</p>` : "") + `</form>`;
  return { button, form };
}

// canSave says whether a form's text can be saved: a text write needs
// something written, and an edit a change (and text, if it's required).
export function canSave(key, text) {
  const s = writeState(key);
  if (s.value === undefined) return !!text.trim();
  return text.trim() !== s.value.trim() && (!s.required || !!text.trim());
}

// discard drops an edit's draft, so its form starts again from the text
// as it is now.
export function discard(key) {
  states.set(key, { open: true });
}

// More: a row's rarer writes (park, drop) stay behind a toggle that opens
// in place, as nothing floats. Open ones are kept here, by key, so live
// re-renders keep them open.
const more = new Set();
export const moreOpen = key => more.has(key);
export const toggleMore = key => (more.has(key) ? more.delete(key) : more.add(key));
export const moreButton = (key, open) =>
  `<button class="btn s more${open ? " on" : ""}" data-more="${esc(key)}" aria-expanded="${open ? "true" : "false"}">${open ? "Less" : "More"}</button>`;

// failure is the line saying why a write didn't save, where it was tried.
export function failure(key, what) {
  const s = writeState(key);
  return s.error ? `<p class="wfail" role="alert"><b>Not saved</b>${esc(what)}: ${esc(s.error)}</p>` : "";
}
