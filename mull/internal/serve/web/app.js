// mull serve's page: state, loading, routing and actions. The project list
// comes from GET /api/projects; each readable project's tree and revision
// are loaded for the sheet index, and the current project's overview with
// them. GET /api/events says which project moved, and only that one is
// fetched again.

import { api, listen } from "./api.js";
import { esc, ago } from "./html.js";
import { Changes } from "./revision.js";
import { renderTree, expandFor, toggle, reveal } from "./tree.js";
import { pageHome, pageProject, pageUndrawn, pageNotFound, pageUnreadable } from "./pages.js";

const state = {
  projects: null,    // GET /api/projects, once loaded
  listError: null,
  data: new Map(),   // project ID → {tree, revision, changes, overview, stale, …}
  live: false,       // the event stream is open
};
// What the human opened or closed (tiers, groups, the revision schedule),
// by data-open key, so live re-renders keep it.
const opened = new Map();

const $ = id => document.getElementById(id);
const main = $("main"), treeEl = $("tree");

// ---- routes: #/<project>[/<kind>[/<id>]], mirroring the API's paths

function route() {
  let parts;
  try {
    parts = location.hash.replace(/^#\/?/, "").split("/").filter(Boolean).map(decodeURIComponent);
  } catch {
    parts = ["?"];
  }
  const [pid = null, kind = "", id = ""] = parts;
  return { pid, kind, id };
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
    try {
      state.projects = (await api.projects()).projects;
      state.listError = null;
    } catch (err) {
      state.listError = err;
    }
    for (const id of [...state.data.keys()]) if (!readable(id)) state.data.delete(id);
    for (const pr of state.projects || []) if (!pr.error && !state.data.has(pr.id)) loadProject(pr.id);
    render();
  });
}

function loadProject(id) {
  return coalesce("p:" + id, async () => {
    const d = state.data.get(id) || {};
    state.data.set(id, d);
    const current = route().pid === id;
    const [tree, rev, ov] = await Promise.allSettled([api.tree(id), api.revision(id), current ? api.overview(id) : null]);
    if (!readable(id)) return;
    const first = !d.tree;
    if (tree.status === "fulfilled") { d.tree = tree.value; d.treeError = null; } else d.treeError = tree.reason;
    if (first && d.tree && current) expandFor(route(), state); // now the tree knows where the route's row is
    if (rev.status === "fulfilled") useRevision(id, rev.value);
    if (current) {
      if (ov.status === "fulfilled") { d.overview = ov.value; d.overviewError = null; d.stale = false; } else d.overviewError = ov.reason;
    } else {
      d.stale = true; // fetched again when the human opens it
    }
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
  if (!state.projects) return state.listError ? pageNotFound(`Couldn't load the project list: ${state.listError.message}.`) : `<div class="page"><p class="empty">Loading…</p></div>`;
  const pr = state.projects.find(p => p.id === r.pid);
  if (!pr) return pageNotFound(`No project “${r.pid}” is registered with mull serve.`);
  if (pr.error) return pageUnreadable(pr);
  if (!KINDS.has(r.kind) || (r.kind !== "" && r.kind !== "unplanned" && r.kind !== "backlog" && !r.id)) return pageNotFound("That isn't a sheet in this set.");
  if (r.kind === "") return pageProject(state, pr);
  return pageUndrawn(state, pr, r);
}

function title(r) {
  if (!r.pid) return "mull";
  return r.kind ? `${r.id || r.kind} · ${r.pid} · mull` : `${r.pid} · mull`;
}

// focusKey names the focused control, so a live re-render can give focus
// back to its replacement.
function focusKey() {
  const el = document.activeElement;
  if (!el || el === document.body) return null;
  if (el.tagName === "SUMMARY" && el.parentElement.dataset.open) return `[data-open="${CSS.escape(el.parentElement.dataset.open)}"] > summary`;
  for (const attr of ["data-tg", "data-seen", "data-copy", "data-cmd", "href"]) {
    const v = el.getAttribute(attr);
    if (v !== null) return `[${attr}="${CSS.escape(v)}"]`;
  }
  return null;
}

function render() {
  const r = route();
  const focus = focusKey(), inTree = treeEl.contains(document.activeElement);
  main.innerHTML = page(r);
  document.title = title(r);
  for (const el of main.querySelectorAll("[data-open]")) {
    if (opened.has(el.dataset.open)) el.open = opened.get(el.dataset.open);
  }
  joinRowClouds(main);
  renderTree(treeEl, state, r);
  renderFoot();
  renderLive();
  for (const d of state.data.values()) d.changes?.settle();
  if (focus) (inTree ? treeEl : main).querySelector(focus)?.focus({ preventScroll: true });
}

// Consecutive clouded table rows share one cloud.
function joinRowClouds(root) {
  for (const tr of root.querySelectorAll("tr.cl")) {
    if (tr.previousElementSibling?.classList.contains("cl")) tr.classList.add("cl-cont");
    if (tr.nextElementSibling?.classList.contains("cl")) tr.classList.add("cl-open");
  }
}

function renderFoot() {
  const bad = (state.projects || []).filter(p => p.error);
  const foot = $("railfoot");
  foot.hidden = !bad.length;
  if (!bad.length) return;
  const fresh = bad.filter(p => p.error.code === "not_initialized"), other = bad.filter(p => p.error.code !== "not_initialized");
  foot.innerHTML =
    (fresh.length ? `<div><span class="lab">Not on mull yet</span><p>${fresh.map(p => esc(p.name)).join(" · ")}</p></div>` : "") +
    (other.length ? `<div><span class="lab">Can't read</span><p>${other.map(p => `<a href="#/${encodeURIComponent(p.id)}">${esc(p.name)}</a> ${esc(p.error.code)}`).join(" · ")}</p></div>` : "");
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
  } catch {
    toast(fallback);
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

document.addEventListener("toggle", ev => {
  const k = ev.target.dataset?.open;
  if (k) opened.set(k, ev.target.open);
}, true);

document.addEventListener("click", ev => {
  const t = ev.target;
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
    if (ev.metaKey || ev.ctrlKey) window.open(row.dataset.href, "_blank");
    else location.hash = row.dataset.href;
  }
});

window.addEventListener("hashchange", () => {
  document.querySelector(".rail").classList.remove("open");
  $("idx").setAttribute("aria-expanded", "false");
  const r = route();
  expandFor(r, state);
  const d = r.pid && state.data.get(r.pid);
  if (d && (d.stale || !d.overview)) loadProject(r.pid);
  render();
  reveal(treeEl);
  window.scrollTo(0, 0);
});

// ---- start

listen({
  project: id => {
    loadList();
    if (state.data.has(id)) loadProject(id);
  },
  live: up => { state.live = up; render(); },
  reopened: loadAll,
});

loadList().then(() => {
  expandFor(route(), state);
  render();
  reveal(treeEl);
});
render();
