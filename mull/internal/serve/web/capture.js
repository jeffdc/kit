// Capture: the human's quick way to get a bug or idea down, before an
// agent is involved. A write that takes text, like the others in write.js,
// but with a title, an optional detail and a kind: the button opens its form
// under the tabs row (or c, anywhere but a text field), Enter in the title or
// ⌘↵ saves, and Esc or Cancel closes it keeping the draft. The draft is kept
// per project in this browser too, so a reload (a new build) doesn't lose a
// thought half written.

import { esc } from "./html.js";
import { writeState, setWrite, TICK } from "./write.js";

export const captureKey = pid => `capture:${pid}`;

// The draft's fields, as kept in the browser.
const FIELDS = ["title", "detail", "kind", "untag"];
const stored = pid => `mull.capture:${pid}`;

function load(pid) {
  try { return JSON.parse(localStorage.getItem(stored(pid)) || "{}"); } catch { return {}; }
}

// keepDraft saves the draft in the browser (or forgets it once it's empty).
export function keepDraft(pid) {
  const s = writeState(captureKey(pid));
  try {
    if (!s.title && !s.detail) localStorage.removeItem(stored(pid));
    else localStorage.setItem(stored(pid), JSON.stringify(Object.fromEntries(FIELDS.map(f => [f, s[f] || ""]))));
  } catch { /* storage off: the draft lives only while the page is open */ }
}

// draft is the capture state for a project, from the browser the first
// time it's asked for.
export function draft(pid) {
  const key = captureKey(pid);
  let s = writeState(key);
  if (!s.loaded) {
    s = { ...load(pid), ...s, loaded: true };
    setWrite(key, s);
  }
  return s;
}

// setDraft sets one field of a project's draft.
export function setDraft(pid, field, value) {
  setWrite(captureKey(pid), { ...draft(pid), [field]: value });
  keepDraft(pid);
}

// captureButton opens (or closes) the form.
export function captureButton(pid) {
  const s = draft(pid);
  return `<button class="btn wr${s.open ? " armed" : ""}" data-capture-open="${esc(pid)}" aria-expanded="${s.open ? "true" : "false"}" title="Capture a bug or idea into the backlog (c)">${TICK}Capture</button>`;
}

// captureForm is the form, empty while closed. On a plan's sheet (plan is
// {slug, title}) what's captured is tagged to that plan unless the box is
// cleared; elsewhere it goes to the project's backlog untagged.
export function captureForm(pid, plan = null) {
  const s = draft(pid);
  if (!s.open) return "";
  const kind = s.kind === "bug" ? "bug" : "idea";
  const ro = s.pending ? " readonly" : "";
  const tagged = plan && s.untag !== plan.slug;
  const seg = (v, label) => `<button type="button" data-cap-kind="${v}" aria-pressed="${kind === v}"${s.pending ? " disabled" : ""}>${label}</button>`;
  const where = plan
    ? `<label class="cap-plan"><input type="checkbox" data-cap-plan="${esc(plan.slug)}"${tagged ? " checked" : ""}${s.pending ? " disabled" : ""}> For this plan</label>`
    : `<span class="sub">To the backlog, untagged.</span>`;
  return `<form class="wform cap" data-capture="${esc(pid)}"${tagged ? ` data-plan="${esc(plan.slug)}"` : ""}>` +
    `<div class="cap-line"><span class="seg" role="group" aria-label="Kind">${seg("bug", "Bug")}${seg("idea", "Idea")}</span>` +
    `<input type="text" data-cap="title" value="${esc(s.title || "")}" placeholder="${kind === "bug" ? "What's wrong, in a line" : "The idea, in a line"}" aria-label="Title" autocomplete="off"${ro}></div>` +
    `<textarea data-cap="detail" rows="2" placeholder="More, if there is any (optional): it becomes the item's notes, and markdown works." aria-label="Detail"${ro}>${esc(s.detail || "")}</textarea>` +
    `<div class="wform-acts"><button type="submit" class="btn wr s${s.pending ? " pending" : ""}"${s.pending || !(s.title || "").trim() ? " disabled" : ""}>${TICK}${esc(s.pending ? "Saving…" : `Capture ${kind}`)}</button>` +
    `<button type="button" class="btn s" data-capture-close${s.pending ? " disabled" : ""}>Cancel</button>${where}<span class="sub">↵ in the title or ⌘↵ saves · Esc closes, keeping what you wrote</span></div>` +
    (s.error ? `<p class="wfail" role="alert"><b>Not saved</b>${esc(s.error)}</p>` : "") + `</form>`;
}

// captureBar places the form under a sheet's tabs.
export const captureBar = (pid, plan = null) => {
  const form = captureForm(pid, plan);
  return form ? `<div class="capbar">${form}</div>` : "";
};

// captureOps is the write a form sends, or null if it has no title.
export function captureOps(form) {
  const s = draft(form.dataset.capture);
  const title = (s.title || "").trim();
  if (!title) return null;
  const op = { op: s.kind === "bug" ? "bug" : "idea", title };
  if ((s.detail || "").trim()) op.detail = s.detail.trim();
  if (form.dataset.plan) op.plan = form.dataset.plan;
  return [op];
}
