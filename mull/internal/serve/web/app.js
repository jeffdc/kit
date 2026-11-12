// mull serve's page: state, loading, routing and actions. The project list
// comes from GET /api/projects; each readable project's tree and revision
// are loaded for the sheet index, and the API pages the current sheet reads
// with them. GET /api/events says which project moved, and only that one is
// fetched again.

import { api, listen } from "./api.js";
import { esc, ago, href } from "./html.js";
import { Changes } from "./revision.js";
import { renderTree, expandFor, toggle, reveal } from "./tree.js";
import { pageHome, projectSheet, subsFor, pageNotFound, pageUnreadable, notOnMull } from "./pages.js";
import { filterOf, clearFilter } from "./lists.js";
import { pageModel } from "./model.js";
import { writeState, setWrite, disarm, toggleMore } from "./write.js";
import { openSearch } from "./search.js";

const state = {
  projects: null,    // GET /api/projects, once loaded
  listError: null,
  unmulled: null,    // GET /api/unmulled: repos beside these not on mull yet
  data: new Map(),   // project ID → {tree, revision, changes, pages: Map(API sub-path → {value} | {error})}
  live: false,       // the event stream is open
  focusNeeds: "",    // a stamp was clicked: the sheet (hash) to open at its Needs you
  focusReview: "",   // a Review link was clicked: the item sheet (hash) to open at its review packet
};
// What the human opened or closed (tiers, groups, the revision schedule),
// by data-open key, so live re-renders keep it.
const opened = new Map();

const $ = id => document.getElementById(id);
const main = $("main"), treeEl = $("tree");

// ---- routes: #/<project>[/<kind>[/<id>[/<tab>]]], mirroring the API's paths

function route() {
  let parts;
  try {
    parts = location.hash.replace(/^#\/?/, "").split("/").filter(Boolean).map(decodeURIComponent);
  } catch {
    parts = ["?"];
  }
  const [pid = null, kind = "", id = "", sub = ""] = parts;
  return { pid, kind, id, sub };
}
const KINDS = new Set(["", "plans", "phases", "items", "unplanned", "backlog"]);

// ---- loading

// coalesce runs fn at most once at a time per key. A call while it runs
// queues one more run after it, so a burst of events is at most two fetches.
const running = new Map();
function coalesce(key, fn) {
  const r = running.get(key);
  if (r) { r.again = true; return r.done; }
  const entry = { again: false };
  entry.done = fn().catch(err => console.error(err)).finally(() => {
    running.delete(key);
    if (entry.again) coalesce(key, fn);
  });
  running.set(key, entry);
  return entry.done;
}

const readable = id => (state.projects || []).some(p => p.id === id && !p.error);

function loadList() {
  return coalesce("list", async () => {
    const [list, unmulled] = await Promise.allSettled([api.projects(), api.unmulled()]);
    if (list.status === "fulfilled") {
      state.projects = list.value.projects;
      state.listError = null;
    } else {
      state.listError = list.reason;
    }
    if (unmulled.status === "fulfilled") state.unmulled = unmulled.value;
    for (const id of [...state.data.keys()]) if (!readable(id)) state.data.delete(id);
    for (const pr of state.projects || []) if (!pr.error && !state.data.has(pr.id)) loadProject(pr.id);
    render();
  });
}

function loadProject(id) {
  return coalesce("p:" + id, async () => {
    const d = state.data.get(id) || {};
    state.data.set(id, d);
    const r = route(), current = r.pid === id;
    const subs = current ? subsFor(r) : [];
    const [tree, rev, ...pages] = await Promise.allSettled([api.tree(id), api.revision(id), ...subs.map(s => api.page(id, s))]);
    if (!readable(id)) return;
    const first = !d.tree;
    if (tree.status === "fulfilled") { d.tree = tree.value; d.treeError = null; } else d.treeError = tree.reason;
    if (first && d.tree && current) expandFor(route(), state); // now the tree knows where the route's row is
    if (rev.status === "fulfilled") useRevision(id, rev.value);
    // Only the pages just read are kept: any other is fetched again when
    // the human opens it, so nothing stale is shown after a change.
    d.pages = new Map(subs.map((s, i) => [s, pages[i].status === "fulfilled" ? { value: pages[i].value } : { error: pages[i].reason }]));
    render();
  });
}

// useRevision keeps a project's revision, its changes (fresh ones are those
// not drawn before), and the list's count, in step.
function useRevision(id, rev) {
  const d = state.data.get(id);
  d.revision = rev;
  d.changes = new Changes(rev, d.changes);
  const pr = (state.projects || []).find(p => p.id === id);
  if (pr) pr.revision = { number: rev.tip.number, changes: rev.changes.length, seen_at: rev.seen_at };
}

function loadAll() {
  loadList();
  for (const id of state.data.keys()) loadProject(id);
}

// ---- rendering

function page(r) {
  if (!r.pid) return pageHome(state);
  if (r.pid === "-") return r.kind === "model" && !r.id ? pageModel() : pageNotFound("That isn't a sheet in this set.");
  if (!state.projects) return state.listError ? pageNotFound(`Couldn't load the project list: ${state.listError.message}.`) : `<div class="page"><p class="empty">Loading…</p></div>`;
  const pr = state.projects.find(p => p.id === r.pid);
  if (!pr) return pageNotFound(`No project “${r.pid}” is registered with mull serve.`);
  if (pr.error) return pageUnreadable(pr);
  if (!KINDS.has(r.kind) || (r.kind !== "" && r.kind !== "unplanned" && r.kind !== "backlog" && !r.id) ||
    (r.kind === "unplanned" && r.id && r.id !== "all")) return pageNotFound("That isn't a sheet in this set.");
  return projectSheet(state, pr, r);
}

function title(r) {
  if (!r.pid) return "mull";
  if (r.pid === "-") return "The model · mull";
  return r.kind ? `${r.id || r.kind} · ${r.pid} · mull` : `${r.pid} · mull`;
}

// focusKey names the focused control, so a live re-render can give focus
// back to its replacement.
function focusKey() {
  const el = document.activeElement;
  if (!el || el === document.body) return null;
  if (el.tagName === "SUMMARY" && el.parentElement.dataset.open) return `[data-open="${CSS.escape(el.parentElement.dataset.open)}"] > summary`;
  for (const attr of ["data-tg", "data-seen", "data-write", "data-draft", "data-form-open", "data-more", "data-copy", "data-copy-id", "data-cmd", "href"]) {
    const v = el.getAttribute(attr);
    if (v !== null) return `[${attr}="${CSS.escape(v)}"]`;
  }
  return null;
}

function render() {
  const r = route();
  const focus = focusKey(), inTree = treeEl.contains(document.activeElement);
  const caret = document.activeElement?.matches?.("textarea") ? [document.activeElement.selectionStart, document.activeElement.selectionEnd] : null;
  main.innerHTML = page(r);
  document.title = title(r);
  for (const el of main.querySelectorAll("[data-open]")) {
    if (opened.has(el.dataset.open)) el.open = opened.get(el.dataset.open);
    if (el.dataset.rows) showRows(el);
  }
  joinRowClouds(main);
  renderTree(treeEl, state, r);
  renderFoot(r);
  renderLive();
  for (const d of state.data.values()) d.changes?.settle();
  if (focus) {
    const el = (inTree ? treeEl : main).querySelector(focus);
    el?.focus({ preventScroll: true });
    if (caret && el?.matches("textarea")) el.setSelectionRange(...caret); // typing survives a live re-render
  }
  if (state.focusNeeds === location.hash) showNeeds();
  if (state.focusReview === location.hash) showReview();
}

// showReview scrolls an item's sheet to its review packet, once the sheet
// has drawn it.
function showReview() {
  const sec = main.querySelector("section.review");
  if (!sec) return; // not loaded yet: the next render tries again
  state.focusReview = "";
  sec.scrollIntoView({ block: "start" });
}

// showNeeds opens every tier and group in the sheet's Needs you and scrolls
// to it, once the sheet has drawn it.
function showNeeds() {
  const sec = main.querySelector("#needs");
  if (!sec) return; // not loaded yet: the next render tries again
  state.focusNeeds = "";
  for (const d of sec.querySelectorAll("details[data-open]")) {
    if (d.classList.contains("tier")) { d.open = true; opened.set(d.dataset.open, true); }
  }
  sec.scrollIntoView({ block: "start" });
}

// Consecutive clouded table rows share one cloud.
function joinRowClouds(root) {
  for (const tr of root.querySelectorAll("tr.cl")) {
    if (tr.previousElementSibling?.classList.contains("cl")) tr.classList.add("cl-cont");
    if (tr.nextElementSibling?.classList.contains("cl")) tr.classList.add("cl-open");
  }
}

// renderFoot: what isn't on mull yet (old mull data to convert, then
// recent repos with no mull branch), registered repos that can't be read,
// and the link to the model.
function renderFoot(r) {
  const foot = $("railfoot");
  foot.hidden = false;
  const { convert, notYet, broken } = notOnMull(state);
  const names = list => list.map(u => `<span title="${esc(u.path)}">${esc(u.name)}</span>`).join(" · ");
  const here = r.pid === "-" && r.kind === "model";
  foot.innerHTML =
    (convert.length ? `<div><span class="lab">Needs converting</span><p>${names(convert)}</p></div>` : "") +
    (notYet.length ? `<div><span class="lab">Not on mull yet</span><p>${names(notYet)}</p></div>` : "") +
    (broken.length ? `<div><span class="lab">Can't read</span><p>${broken.map(p => `<a href="#/${encodeURIComponent(p.id)}">${esc(p.name)}</a>`).join(" · ")}</p></div>` : "") +
    `<div><a class="btn s${here ? " on" : ""}" href="${href.model()}"${here ? ' aria-current="page"' : ""}>The model: how mull works</a></div>`;
}

function renderLive() {
  const el = $("live");
  el.classList.toggle("off", !state.live);
  el.lastChild.textContent = state.live ? "Live" : "Offline";
  el.title = state.live ? "Following every repo's mull branch" : "Lost mull serve; reconnecting";
}

// Relative times ("4 min ago") are kept current without a re-render.
setInterval(() => {
  for (const el of document.querySelectorAll("[data-ago]")) el.textContent = ago(el.dataset.ago);
}, 30000);

// ---- actions

let toastTimer;
function toast(html) {
  const t = $("toast");
  t.innerHTML = html;
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (t.hidden = true), 5000);
}

async function copy(text, done, fallback) {
  try {
    await navigator.clipboard.writeText(text);
    toast(done);
    return true;
  } catch {
    toast(fallback);
    return false;
  }
}

// copyID copies a bare ID from its copy-ID button (copyId in html.js). The
// button turns to a tick for a moment, where it was clicked; if the
// clipboard can't be reached, the ID is selected for ⌘C instead.
async function copyID(btn) {
  const id = btn.dataset.copyId;
  if (await copy(id, `Copied the ID <code>${esc(id)}</code>.`, `Couldn't reach the clipboard. The ID <code>${esc(id)}</code> is selected: press ⌘C to copy it.`)) {
    btn.classList.add("ok");
    clearTimeout(btn.okTimer);
    btn.okTimer = setTimeout(() => btn.classList.remove("ok"), 1600);
  } else {
    const el = btn.previousElementSibling, code = el?.matches("code, .id") ? el : el?.querySelector("code");
    if (code) getSelection().selectAllChildren(code);
  }
}

const wait = ms => new Promise(r => setTimeout(r, ms));

// markSeen marks the revision this page showed as seen. Its clouds fade
// first, the tree's with them; anything newer stays clouded.
async function markSeen(pid, btn) {
  const d = state.data.get(pid);
  const commit = d?.revision?.tip.commit;
  btn.disabled = true;
  const clouds = [...main.querySelectorAll(".cl"), ...treeEl.querySelectorAll(`.tcl[data-pid="${CSS.escape(pid)}"]`)];
  clouds.forEach(el => el.classList.add("fading"));
  try {
    const [rev] = await Promise.all([api.seen(pid, commit), wait(360)]);
    useRevision(pid, rev);
    render();
    toast(`Revision ${rev.since.number} marked seen. New changes will be clouded.`);
  } catch (err) {
    clouds.forEach(el => el.classList.remove("fading"));
    btn.disabled = false;
    toast(`Couldn't mark it seen: ${esc(err.message)}`);
  }
}

// write runs a write button: the first click arms it, the second saves. It
// sends the revision the page shows, so a page that's behind is refused
// (a conflict) and redrawn rather than acting on what it no longer shows.
let disarmTimer;
async function write(btn) {
  const key = btn.dataset.write, pid = btn.dataset.pid, s = writeState(key);
  if (s.pending) return;
  disarm(key);
  clearTimeout(disarmTimer);
  if (!s.armed) {
    setWrite(key, { armed: true });
    render();
    disarmTimer = setTimeout(() => { if (disarm()) render(); }, 4000);
    return;
  }
  await save(key, pid, JSON.parse(btn.dataset.ops), btn.dataset.done, {});
}

// save sends a write's ops from the revision the page shows. keep is what
// stays of its state while it runs and if it fails (a form's draft).
async function save(key, pid, ops, done, keep) {
  const commit = state.data.get(pid)?.revision?.tip.commit;
  if (!commit) {
    setWrite(key, { ...keep, error: "the page hasn't loaded this project's revision yet; try again in a moment" });
    render();
    return;
  }
  setWrite(key, { ...keep, pending: true });
  render();
  try {
    const rep = await api.ops(pid, commit, ops);
    setWrite(key, null);
    toast(rep.ack_error ? `${esc(done)} It may still be clouded: ${esc(rep.ack_error.error)}` : esc(done));
  } catch (err) {
    if (err.code === "conflict") {
      setWrite(key, keep.open ? { ...keep } : null);
      toast("Nothing was saved: this changed since the page was drawn. It's redrawn now, so look again.");
    } else {
      setWrite(key, { ...keep, error: err.message });
    }
  }
  await loadProject(pid);
  expandFor(route(), state); // a write can move the sheet's row (closing a plan archives it)
  render();
}

// A write that takes text: its button opens the form in place (closing any
// other), Save sends the text as the ops' field, Cancel or Esc closes it
// and keeps the draft.
function openForm(key) {
  disarm(key);
  setWrite(key, { ...writeState(key), open: true, error: "" });
  render();
  const ta = main.querySelector(`textarea[data-draft="${CSS.escape(key)}"]`);
  ta?.focus();
  ta?.setSelectionRange(ta.value.length, ta.value.length);
}

function submitForm(form) {
  const key = form.dataset.form, s = writeState(key);
  const text = form.querySelector("textarea").value.trim();
  if (s.pending || !text) return;
  const ops = JSON.parse(form.dataset.ops).map(op => ({ ...op, [form.dataset.field]: text }));
  save(key, form.dataset.pid, ops, form.dataset.done, { open: true, draft: text });
}

document.addEventListener("submit", ev => {
  const form = ev.target.closest("form[data-form]");
  if (form) { ev.preventDefault(); submitForm(form); }
});

document.addEventListener("input", ev => {
  const ta = ev.target.closest("textarea[data-draft]");
  if (!ta) return;
  const s = writeState(ta.dataset.draft);
  setWrite(ta.dataset.draft, { ...s, draft: ta.value });
  const btn = ta.form?.querySelector('button[type="submit"]');
  if (btn && !s.pending) btn.disabled = !ta.value.trim();
});

// showRows shows or hides the table rows a group's <details> folds.
function showRows(det) {
  for (const tr of main.querySelectorAll(`tr[data-group="${CSS.escape(det.dataset.rows)}"]`)) tr.hidden = !det.open;
}

document.addEventListener("toggle", ev => {
  const k = ev.target.dataset?.open;
  if (k) opened.set(k, ev.target.open);
  if (ev.target.dataset?.rows) showRows(ev.target);
}, true);

// Backlog filters: the kind buttons, the checkbox and the selects.
function setFilter(el) {
  const f = filterOf(el.dataset.filter), k = el.dataset.k;
  f[k] = el.type === "checkbox" ? el.checked : el.dataset.v ?? el.value;
  render();
}
document.addEventListener("change", ev => {
  if (ev.target.matches("select[data-filter], input[data-filter]")) setFilter(ev.target);
});

document.addEventListener("click", ev => {
  const t = ev.target;
  // Copying an ID leaves the rest of the page as it is: an open form keeps
  // its draft, ready for the ID to be pasted into it.
  const ci = t.closest("[data-copy-id]");
  if (ci) { ev.preventDefault(); copyID(ci); return; }
  const wb = t.closest("[data-write]");
  if (wb) { ev.preventDefault(); write(wb); return; }
  const mo = t.closest("[data-more]");
  if (mo) {
    ev.preventDefault();
    disarm();
    toggleMore(mo.dataset.more);
    render();
    return;
  }
  const fo = t.closest("[data-form-open]");
  if (fo) {
    ev.preventDefault();
    if (writeState(fo.dataset.formOpen).open) { disarm(); render(); } else openForm(fo.dataset.formOpen);
    return;
  }
  const fc = t.closest("[data-form-close]");
  if (fc) { disarm(); render(); return; }
  if (t.closest("form[data-form]")) return; // typing in a form, or its Save
  if (disarm()) render(); // a click anywhere else cancels an armed write or closes a form
  if (t.closest("#find")) { openSearch(); return; }
  if (t.closest("#idx")) {
    const open = document.querySelector(".rail").classList.toggle("open");
    $("idx").setAttribute("aria-expanded", open);
    return;
  }
  const tg = t.closest("[data-tg]");
  if (tg) {
    toggle(tg.dataset.tg);
    renderTree(treeEl, state, route());
    treeEl.querySelector(`[data-tg="${CSS.escape(tg.dataset.tg)}"]`)?.focus();
    return;
  }
  const mk = t.closest("a[data-needs]");
  if (mk) {
    state.focusNeeds = mk.getAttribute("href");
    if (state.focusNeeds === location.hash) { ev.preventDefault(); showNeeds(); }
    return; // otherwise the hash change renders the sheet, which shows its Needs you
  }
  const rv = t.closest("a[data-review]");
  if (rv) {
    state.focusReview = rv.getAttribute("href");
    if (state.focusReview === location.hash) { ev.preventDefault(); showReview(); }
    return; // otherwise the hash change renders the item, which shows its packet
  }
  const fb = t.closest("button[data-filter]");
  if (fb) { setFilter(fb); return; }
  const clear = t.closest("[data-clear]");
  if (clear) { clearFilter(clear.dataset.clear); render(); return; }
  const seen = t.closest("[data-seen]");
  if (seen) { markSeen(seen.dataset.seen, seen); return; }
  const cmd = t.closest("[data-cmd]");
  if (cmd) {
    ev.preventDefault();
    const say = cmd.dataset.say ? `${esc(cmd.dataset.say)} ` : "";
    copy(cmd.dataset.cmd, `${say}Copied <code>${esc(cmd.dataset.cmd)}</code>: run it in a terminal.`, `${say}Run <code>${esc(cmd.dataset.cmd)}</code> in a terminal.`);
    return;
  }
  const cp = t.closest("[data-copy]");
  if (cp) {
    ev.preventDefault();
    const text = `Let's work through these open mull questions together: ${cp.dataset.copy}. Run mull context for each first.`;
    copy(text, "Copied a prompt for an agent. Paste it into your agent session.", `Copy this for an agent: <code>${esc(text)}</code>`);
    return;
  }
  const row = t.closest("[data-href]");
  if (row && !t.closest("a, button, summary, input, select, label")) {
    if (ev.metaKey || ev.ctrlKey) { window.open(row.dataset.href, "_blank"); return; }
    if ("review" in row.dataset) {
      state.focusReview = row.dataset.href;
      if (state.focusReview === location.hash) { showReview(); return; }
    }
    location.hash = row.dataset.href;
  }
});

document.addEventListener("keydown", ev => {
  if (ev.key === "Escape" && disarm()) render();
  if (ev.key === "Enter" && (ev.metaKey || ev.ctrlKey) && ev.target.matches("textarea[data-draft]")) {
    ev.preventDefault();
    ev.target.form?.requestSubmit();
  }
});

window.addEventListener("hashchange", () => {
  document.querySelector(".rail").classList.remove("open");
  $("idx").setAttribute("aria-expanded", "false");
  const r = route();
  expandFor(r, state);
  const d = r.pid && state.data.get(r.pid);
  if (d && subsFor(r).some(s => !d.pages?.has(s))) loadProject(r.pid);
  // A stamp or a Review link opens the sheet somewhere: render scrolls there.
  const placed = state.focusNeeds === location.hash || state.focusReview === location.hash;
  if (!placed) { state.focusNeeds = state.focusReview = ""; window.scrollTo(0, 0); }
  render();
  reveal(treeEl);
});

// ---- start

// The build this page was loaded from. When the stream comes back from a
// server running a different build (mull was upgraded and mull serve
// restarted), the page reloads itself onto the new UI rather than run old
// code against a new server.
let build = null;
async function checkBuild() {
  try {
    const now = (await api.build()).build;
    if (build && now !== build) { location.reload(); return true; }
    build = now;
  } catch { /* an older server without /api/build: keep going */ }
  return false;
}
checkBuild();

listen({
  project: id => {
    loadList();
    if (state.data.has(id)) loadProject(id);
  },
  live: up => { state.live = up; render(); },
  reopened: async () => { if (!(await checkBuild())) loadAll(); },
});

loadList().then(() => {
  expandFor(route(), state);
  render();
  reveal(treeEl);
});
render();
