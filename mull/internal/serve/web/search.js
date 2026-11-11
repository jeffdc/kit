// Find: s opens a search band at the top of the sheet, in place (nothing
// floats but the toast), that finds records by ID or title in every
// project. Up and Down move, Enter (or a click) opens, Esc closes.

import { esc, href, tag } from "./html.js";
import { api } from "./api.js";

const band = document.getElementById("search");
let hits = [], sel = 0, seq = 0, timer;

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
  const where = [h.project, what, h.type === "entry" && h.item ? `on ${h.item}` : ""].filter(Boolean).map(esc).join(" · ");
  const state = h.state && h.type !== "entry" ? tag(h.state) : h.state && h.state !== "open" ? `<span class="lab">${esc(SETTLED[h.state] || h.state)}</span>` : "";
  return `<li${i === sel ? ' class="on" aria-selected="true"' : ""} role="option"><a href="${esc(target(h))}" data-hit="${i}">` +
    `<span class="id">${esc(h.id)}</span><span class="txt"><b>${esc(h.title)}</b><small>${where}</small></span><span class="st">${state}</span></a></li>`;
}

function draw(q) {
  const list = band.querySelector(".hits");
  if (!q.trim()) { list.innerHTML = ""; return; }
  list.innerHTML = hits.length ? hits.map(row).join("") : `<li class="none">Nothing has that ID or those words in its title.</li>`;
  list.querySelector(".on")?.scrollIntoView({ block: "nearest" });
}

async function find(q) {
  const n = ++seq;
  try {
    const res = await api.search(q);
    if (n !== seq) return; // a later search is on its way
    hits = res.hits;
    sel = 0;
    draw(q);
  } catch (err) {
    if (n === seq) band.querySelector(".hits").innerHTML = `<li class="none">Search failed: ${esc(err.message)}</li>`;
  }
}

export function openSearch() {
  if (!band.hidden) { band.querySelector("input").select(); return; }
  band.hidden = false;
  band.innerHTML = `<div class="find"><label class="lab" for="find-q">Find</label>` +
    `<input id="find-q" type="search" autocomplete="off" spellcheck="false" placeholder="An ID, or words from a title" aria-controls="find-hits">` +
    `<span class="hint">↑↓ move · Enter opens · Esc closes</span></div><ol class="hits" id="find-hits" role="listbox"></ol>`;
  hits = []; sel = 0;
  const input = band.querySelector("input");
  input.addEventListener("input", () => {
    clearTimeout(timer);
    const q = input.value;
    if (!q.trim()) { seq++; hits = []; draw(q); return; }
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
  band.addEventListener("click", ev => {
    const a = ev.target.closest("[data-hit]");
    if (!a || ev.metaKey || ev.ctrlKey) return;
    ev.preventDefault();
    go(hits[Number(a.dataset.hit)]);
  });
  input.focus();
}

function go(h) {
  closeSearch();
  location.hash = target(h);
}

export function closeSearch() {
  clearTimeout(timer);
  seq++;
  band.hidden = true;
  band.innerHTML = "";
}

// s opens Find unless the human is typing somewhere.
document.addEventListener("keydown", ev => {
  if (ev.key !== "s" || ev.metaKey || ev.ctrlKey || ev.altKey) return;
  if (ev.target.closest?.("input, textarea, select, [contenteditable]")) return;
  ev.preventDefault();
  openSearch();
});
