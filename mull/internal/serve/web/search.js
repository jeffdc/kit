// Find: s opens a command palette floating mid-screen over a scrim that
// finds records by ID or title in every project. Up and Down move, Enter
// (or a click) opens, Esc or a click outside closes. It searches open
// records first; the archive (closed plans and their records) only when
// asked, from the foot, which says how many matches it holds.

import { esc, href, tag } from "./html.js";
import { api } from "./api.js";

const band = document.getElementById("search");
let hits = [], sel = 0, seq = 0, timer, before = null, archive = false, archived = 0;

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
  const where = [h.project, what, h.type === "entry" && h.item ? `on ${h.item}` : "", h.archived ? "archive" : ""].filter(Boolean).map(esc).join(" · ");
  const state = h.state && h.type !== "entry" ? tag(h.state) : h.state && h.state !== "open" ? `<span class="lab">${esc(SETTLED[h.state] || h.state)}</span>` : "";
  return `<li${i === sel ? ' class="on" aria-selected="true"' : ""} role="option"><a href="${esc(target(h))}" data-hit="${i}">` +
    `<span class="id">${esc(h.id)}</span><span class="txt"><b>${esc(h.title)}</b><small>${where}</small></span><span class="st">${state}</span></a></li>`;
}

function draw(q) {
  const list = band.querySelector(".hits");
  foot(q);
  if (!q.trim()) { list.innerHTML = ""; return; }
  list.innerHTML = hits.length ? hits.map(row).join("")
    : `<li class="none">${archived ? "Nothing open has" : "Nothing has"} that ID or those words in its title.</li>`;
  list.querySelector(".on")?.scrollIntoView({ block: "nearest" });
}

// foot gives the keys, and offers the archive when it holds more matches.
function foot(q) {
  const more = q.trim() && !archive && archived
    ? ` · <button type="button" class="btn s" data-find-archive>Search the archive too: ${archived} more</button>` : "";
  band.querySelector(".keys").innerHTML = `↑↓ move · Enter opens · Esc closes · ${archive ? "every project, archive too" : "every project"}${more}`;
}

async function find(q) {
  const n = ++seq;
  try {
    const res = await api.search(q, archive);
    if (n !== seq) return; // a later search is on its way
    hits = res.hits;
    archived = res.archived;
    sel = 0;
    draw(q);
  } catch (err) {
    if (n === seq) band.querySelector(".hits").innerHTML = `<li class="none">Search failed: ${esc(err.message)}</li>`;
  }
}

export function openSearch() {
  if (!band.hidden) { band.querySelector("input").select(); return; }
  before = document.activeElement;
  band.hidden = false;
  band.innerHTML = `<div class="scrim" data-find-close></div>` +
    `<div class="palette" role="dialog" aria-modal="true" aria-label="Find"><div class="q"><label class="lab" for="find-q">Find</label>` +
    `<input id="find-q" type="search" autocomplete="off" spellcheck="false" placeholder="An ID, or words from a title" aria-controls="find-hits"></div>` +
    `<ol class="hits" id="find-hits" role="listbox"></ol><div class="keys">↑↓ move · Enter opens · Esc closes · every project</div></div>`;
  hits = []; sel = 0; archive = false; archived = 0;
  const input = band.querySelector("input");
  input.addEventListener("input", () => {
    clearTimeout(timer);
    const q = input.value;
    if (!q.trim()) { seq++; hits = []; archived = 0; draw(q); return; }
    timer = setTimeout(() => find(q), 60);
  });
  input.addEventListener("keydown", ev => {
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
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
  band.querySelector(".scrim").addEventListener("click", () => closeSearch());
  band.querySelector(".palette").addEventListener("click", ev => {
    if (ev.target.closest("[data-find-archive]")) {
      archive = true;
      input.focus();
      find(input.value);
      return;
    }
    const a = ev.target.closest("[data-hit]");
    if (!a || ev.metaKey || ev.ctrlKey) return;
    ev.preventDefault();
    go(hits[Number(a.dataset.hit)]);
  });
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
