// Find: s opens a command palette floating mid-screen over a scrim that
// finds records by ID or title. Up and Down move, Enter (or a click) opens,
// Esc or a click outside closes. On a project's sheet it searches that
// project (⌥P switches to every project and back); elsewhere, every
// project. Tab (Shift+Tab back) cycles the status filter: Open (live
// work), Finished (done, dropped, settled or archived) and All. The foot
// says how many matches the filter left out, so nothing seems missing.

import { esc, href, tag, copyId } from "./html.js";
import { api } from "./api.js";

const band = document.getElementById("search");
const STATUSES = [["open", "Open"], ["finished", "Finished"], ["all", "All"]];
let hits = [], sel = 0, seq = 0, timer, before = null;
let status = "open", home = null, scoped = false, counts = { open: 0, finished: 0 };

// scopeOf names the project whose sheet is showing (null off a project);
// app.js sets it, as it knows which projects can be read.
let scopeOf = () => null;
export function findScope(fn) { scopeOf = fn; }

// where a hit opens: its sheet, or for an inbox entry the sheet it's on.
function target(h) {
  switch (h.type) {
    case "plan": return href.plan(h.project, h.id);
    case "phase": return href.phase(h.project, h.id);
    case "item": return href.item(h.project, h.id);
  }
  if (h.item) return href.item(h.project, h.item);
  if (h.plan) return href.plan(h.project, h.plan);
  return href.project(h.project);
}

const WHAT = { plan: "plan", phase: "phase", item: "item", entry: "" };
// SETTLED: how a settled entry reads, by the action that settled it.
const SETTLED = { accept: "accepted", decide: "decided", drop: "dropped", park: "parked", do: "made work" };

function row(h, i) {
  const what = h.type === "entry" ? h.kind : h.kind && h.state === "captured" ? h.kind : WHAT[h.type];
  // Searching one project, its name on every hit would be noise.
  const where = [scoped ? "" : h.project, what, h.type === "entry" && h.item ? `on ${h.item}` : "", h.archived ? "archive" : ""].filter(Boolean).map(esc).join(" · ");
  const state = h.state && h.type !== "entry" ? tag(h.state) : h.state && h.state !== "open" ? `<span class="lab">${esc(SETTLED[h.state] || h.state)}</span>` : "";
  // The ID is its own link (out of the tab order: the row's link is the
  // keyboard target), so its copy button can sit beside it, outside both.
  const link = esc(target(h));
  return `<li${i === sel ? ' class="on" aria-selected="true"' : ""} role="option">${copyId(h.id, `<a class="id" href="${link}" data-hit="${i}" tabindex="-1">${esc(h.id)}</a>`)}` +
    `<a class="hit" href="${link}" data-hit="${i}"><span class="txt"><b>${esc(h.title)}</b><small>${where}</small></span><span class="st">${state}</span></a></li>`;
}

// opts draws the filters in the head: the status, and the scope when Find
// opened on a project's sheet. Their buttons stay out of the tab order:
// Tab cycles the status and ⌥P the scope, and focus stays in the input.
function opts() {
  const btn = (attr, v, label, on) => `<button type="button" ${attr}="${esc(v)}" aria-pressed="${on}" tabindex="-1">${esc(label)}</button>`;
  const st = `<span class="seg" role="group" aria-label="Status (Tab)" title="Tab cycles, Shift+Tab goes back">${STATUSES.map(([v, label]) => btn("data-find-status", v, label, status === v)).join("")}</span>`;
  const sc = home ? `<span class="seg" role="group" aria-label="Scope (⌥P)" title="⌥P switches">${btn("data-find-scope", "project", home, scoped)}${btn("data-find-scope", "every", "Every project", !scoped)}</span>` : "";
  band.querySelector(".opts").innerHTML = st + sc;
}

function draw(q) {
  const list = band.querySelector(".hits");
  opts();
  foot(q);
  if (!q.trim()) { list.innerHTML = ""; return; }
  const which = status === "all" ? "" : `${status} `, where = scoped ? `in ${esc(home)} ` : "";
  list.innerHTML = hits.length ? hits.map(row).join("")
    : `<li class="none">Nothing ${which}${where}has that ID or those words in its title.</li>`;
  list.querySelector(".on")?.scrollIntoView({ block: "nearest" });
}

// foot gives the keys, and how many matches the status filter left out.
function foot(q) {
  const keys = ["↑↓ move", "Enter opens", "Tab status", home ? "⌥P scope" : "every project", "Esc closes"].join(" · ");
  const left = !q.trim() ? 0 : status === "open" ? counts.finished : status === "finished" ? counts.open : 0;
  const more = left ? `<span class="more">${left} more ${status === "open" ? "finished" : "open"}</span>` : "";
  band.querySelector(".keys").innerHTML = `<span>${keys}</span>${more}`;
}

async function find(q) {
  const n = ++seq;
  try {
    const res = await api.search(q, status, scoped ? home : "");
    if (n !== seq) return; // a later search is on its way
    hits = res.hits;
    counts = { open: res.open, finished: res.finished };
    sel = 0;
    draw(q);
  } catch (err) {
    if (n === seq) band.querySelector(".hits").innerHTML = `<li class="none">Search failed: ${esc(err.message)}</li>`;
  }
}

// refilter searches again after the status or scope changed.
function refilter() {
  const input = band.querySelector("input");
  clearTimeout(timer);
  if (input.value.trim()) find(input.value);
  else draw(input.value);
}

function cycle(back) {
  const i = STATUSES.findIndex(([v]) => v === status);
  status = STATUSES[(i + (back ? STATUSES.length - 1 : 1)) % STATUSES.length][0];
  refilter();
}

// Focus stays in the palette while it's open (it's a modal dialog).
function trap(ev) {
  if (!band.hidden && !band.contains(ev.target)) band.querySelector("input")?.focus();
}

export function openSearch() {
  if (!band.hidden) { band.querySelector("input").select(); return; }
  before = document.activeElement;
  home = scopeOf();
  hits = []; sel = 0; status = "open"; scoped = !!home; counts = { open: 0, finished: 0 };
  band.hidden = false;
  band.innerHTML = `<div class="scrim" data-find-close></div>` +
    `<div class="palette" role="dialog" aria-modal="true" aria-label="Find"><div class="q"><label class="lab" for="find-q">Find</label>` +
    `<input id="find-q" type="search" autocomplete="off" spellcheck="false" placeholder="An ID, or words from a title" aria-controls="find-hits">` +
    `<div class="opts"></div></div>` +
    `<ol class="hits" id="find-hits" role="listbox"></ol><div class="keys"></div></div>`;
  draw("");
  const input = band.querySelector("input"), palette = band.querySelector(".palette");
  input.addEventListener("input", () => {
    clearTimeout(timer);
    const q = input.value;
    if (!q.trim()) { seq++; hits = []; counts = { open: 0, finished: 0 }; draw(q); return; }
    timer = setTimeout(() => find(q), 60);
  });
  palette.addEventListener("keydown", ev => {
    if (ev.isComposing) return;
    if (ev.key === "Tab") {
      // Tab cycles the status from the input; from anywhere else in the
      // palette it brings focus back to the input.
      ev.preventDefault();
      if (ev.target === input) cycle(ev.shiftKey);
      else input.focus();
    } else if (ev.altKey && !ev.metaKey && !ev.ctrlKey && ev.code === "KeyP") {
      ev.preventDefault(); // not π
      if (home) { scoped = !scoped; refilter(); }
    } else if (ev.target !== input) {
      if (ev.key === "Escape") { ev.preventDefault(); closeSearch(); }
    } else if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      ev.preventDefault();
      if (!hits.length) return;
      sel = (sel + (ev.key === "ArrowDown" ? 1 : hits.length - 1)) % hits.length;
      draw(input.value);
    } else if (ev.key === "Enter" && hits[sel]) {
      ev.preventDefault();
      go(hits[sel]);
    } else if (ev.key === "Escape") {
      ev.preventDefault();
      closeSearch();
    }
  });
  // A filter's button takes the click but not the focus.
  palette.addEventListener("mousedown", ev => {
    if (ev.target.closest("[data-find-status], [data-find-scope]")) ev.preventDefault();
  });
  band.querySelector(".scrim").addEventListener("click", () => closeSearch());
  palette.addEventListener("click", ev => {
    const st = ev.target.closest("[data-find-status]");
    if (st) { status = st.dataset.findStatus; input.focus(); refilter(); return; }
    const sc = ev.target.closest("[data-find-scope]");
    if (sc) { scoped = sc.dataset.findScope === "project"; input.focus(); refilter(); return; }
    const a = ev.target.closest("[data-hit]");
    if (!a || ev.metaKey || ev.ctrlKey) return;
    ev.preventDefault();
    go(hits[Number(a.dataset.hit)]);
  });
  document.addEventListener("focusin", trap);
  input.focus();
}

function go(h) {
  before = null; // the sheet changes: focus starts afresh there
  closeSearch();
  location.hash = target(h);
}

export function closeSearch() {
  clearTimeout(timer);
  seq++;
  document.removeEventListener("focusin", trap);
  band.hidden = true;
  band.innerHTML = "";
  if (before?.isConnected) before.focus();
  before = null;
}

// s opens Find unless the human is typing somewhere.
document.addEventListener("keydown", ev => {
  if (ev.key !== "s" || ev.metaKey || ev.ctrlKey || ev.altKey) return;
  if (ev.target.closest?.("input, textarea, select, [contenteditable]")) return;
  ev.preventDefault();
  openSearch();
});
