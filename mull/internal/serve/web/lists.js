// Item lists: the shared item table, a project's Unplanned and Queue
// sheets, and a plan's queue. Whole rows are links; the title stays the
// keyboard target. Changed items are clouded.

import { esc, href, tag, itemStage, shortTitle, day, base, ago, plural, copyId } from "./html.js";
import { within } from "./revision.js";
import { schedule } from "./needs.js";
import { moveButton, writeState, failure } from "./write.js";

// Columns: each is [heading, cell(item), class]. Tables pick what they need.
export const col = {
  id: () => ["ID", it => copyId(it.id), "", "width:70px"],
  title: (pid, label = "Item") => [label, it => {
    const link = href.item(pid, it.id);
    // Only a live claim: once the work is in review or finished, no one holds it.
    const claim = it.claim && it.state === "active" ? `<span class="meta">${esc(base(it.claim.worktree))}${it.claim.branch ? ` · ${esc(it.claim.branch)}` : ""} · since ${esc(ago(it.claim.at))}</span>` : "";
    const qs = it.questions ? ` <span class="mut">· ${it.questions} open question${it.questions > 1 ? "s" : ""}</span>` : "";
    return `<a class="ttl" href="${esc(link)}">${esc(it.title)}</a>${qs}${claim}`;
  }],
  kind: () => ["Kind", it => esc(it.kind || "")],
  area: () => ["Area", it => it.area ? `<code>${esc(it.area)}</code>` : `<span class="mut">none</span>`],
  stage: () => ["Stage", it => tag(itemStage(it))],
  category: () => ["Category", it => esc(it.category || "")],
  plan: (pid, plans) => ["Plan", it => it.plan
    ? `<a href="${esc(href.plan(pid, it.plan))}">${esc(shortTitle(plans.get(it.plan) || it.plan))}</a>${it.phase ? `<span class="meta">${esc(it.phase_title || it.phase)}</span>` : ""}`
    : `<span class="mut">none</span>`],
  phase: () => ["Phase", it => it.phase ? esc(it.phase_title || it.phase) : `<span class="mut">none</span>`],
  from: () => ["From", it => it.source ? esc(it.source) : `<span class="mut">not given</span>`],
  date: () => ["Date", it => `<span class="mono">${esc(day(it.created))}</span>`, "n"],
};

// wide marks a column to drop at phone width. Never the last column: its
// cell closes a clouded row's cloud.
export const wide = ([th, td, cls = "", style]) => [th, td, `${cls} w`.trim(), style];

const OPEN = new Set(["committed", "active", "in-review"]);
const moveKey = (pid, id, dir) => `move:${pid}:${id}:${dir}`;

// rankTable is the item table of one group (group: a phase ID, a plan
// slug, or none for Unplanned), in rank order. Its first column is an open
// item's position (1, 2, 3; equal ranks share one; finished and unranked
// items have none) with the item's moves: up and down a place among the
// group's open work, each sending the whole new order as a rank write.
// A move that fails says so under the table.
export function rankTable(pid, items, group, cols, opts) {
  const ids = items.filter(it => OPEN.has(it.state)).map(it => it.id);
  const busy = ids.some(id => writeState(moveKey(pid, id, "up")).pending || writeState(moveKey(pid, id, "down")).pending);
  const move = (it, k, dir, to) => {
    const order = ids.filter(id => id !== it.id);
    order.splice(to, 0, it.id);
    return moveButton({ pid, key: moveKey(pid, it.id, dir), ops: [{ op: "rank", group, ids: order }], dir, busy,
      label: `Move ${it.id} ${dir}`, done: `Moved ${it.id} ${dir}: it's ${to + 1} of ${ids.length} now.` });
  };
  const pos = ["#", it => {
    const n = `<span class="mono">${it.position || ""}</span>`;
    const k = ids.indexOf(it.id);
    if (k < 0 || ids.length < 2) return `<span class="pos">${n}</span>`;
    const gap = `<span class="mv-gap"></span>`;
    return `<span class="pos">${n}<span class="mvs">${k > 0 ? move(it, k, "up", k - 1) : gap}${k < ids.length - 1 ? move(it, k, "down", k + 1) : gap}</span></span>`;
  }, "", "width:84px"];
  const failed = items.flatMap(it => ["up", "down"].map(dir => failure(moveKey(pid, it.id, dir), `Moving ${it.id} ${dir}`))).join("");
  return itemTable(pid, items, [pos, ...cols], opts) + failed;
}

export function itemTable(pid, items, cols, { changes, empty = "None." } = {}) {
  if (!items.length) return `<p class="empty">${esc(empty)}</p>`;
  const head = cols.map(([th, , cls, style]) => `<th${cls ? ` class="${cls}"` : ""}${style ? ` style="${style}"` : ""}>${th}</th>`).join("");
  const rows = items.map(it => {
    const cls = [it.state === "done" || it.state === "dropped" ? "waits" : "", changes ? changes.cls(within.item(it.id)).trim() : ""].filter(Boolean).join(" ");
    return `<tr${cls ? ` class="${cls}"` : ""} data-href="${esc(href.item(pid, it.id))}">${cols.map(([, td, c]) => `<td${c ? ` class="${c}"` : ""}>${td(it)}</td>`).join("")}</tr>`;
  }).join("");
  return `<div class="tbl-wrap"><table class="t"><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div>`;
}

// unplannedBody: the unplanned work. Finished work is archived and left
// out (it's under the sheet index's Archive, and on each item's sheet);
// all shows it too.
export function unplannedBody(pid, page, changes, all = false) {
  const n = page.archived;
  const table = rankTable(pid, page.items, "none", [col.id(), col.title(pid), col.kind(), col.stage(), col.category()],
    { changes, empty: n ? "Nothing unplanned is open." : "Nothing unplanned is committed." }) +
    (n
      ? `<p class="empty">${plural(n, "done or dropped item")} ${n === 1 ? "is" : "are"} in the Archive, at the foot of the sheet index. <a href="${esc(href.unplanned(pid, true))}">Show ${n === 1 ? "it" : "them"} here too</a></p>`
      : all ? `<p class="empty">Showing the archived work too. <a href="${esc(href.unplanned(pid))}">Hide it</a></p>` : "");
  const ny = page.needs_you.tiers.length
    ? `<section id="needs"><div class="hd"><h2>Needs you</h2><span class="lab">On unplanned work</span></div>${schedule(page.needs_you, { pid, changes, noPlan: true })}</section>`
    : "";
  return `<section><div class="hd"><h2>Unplanned work</h2><span class="lab">Committed, no plan, in rank order · queued work started with no plan lands here</span></div>${table}</section>${ny}`;
}

// backlogBody: the queue, the agents' captured items, newest first, each
// with its area (where in the code it is) if it has one. It's the agents'
// to file and work, so it's a plain list: no filters, nothing to triage.
// inPlan is the plan's slug on a plan's Queue tab, where the plan column
// gives way to the phase.
export function backlogBody(pid, page, changes, { inPlan = "" } = {}) {
  const plans = new Map(page.plans.map(p => [p.slug, p.title]));
  const all = page.items;
  const cols = [col.id(), col.title(pid, "Queued"), col.kind(), col.area(), wide(inPlan ? col.phase() : col.plan(pid, plans)), wide(col.from()), col.date()];
  const table = all.length
    ? itemTable(pid, all, cols, { changes })
    : `<p class="empty">Nothing queued${inPlan ? " for this plan" : ""}.</p>`;
  const counts = all.length ? `${plural(page.bugs, "bug")} · ${plural(page.ideas, "idea")} · ` : "";
  const other = inPlan ? ` · <a href="${esc(href.backlog(pid))}">the whole project's queue</a>` : "";
  return `<section><div class="hd"><h2>${inPlan ? "This plan's queue" : "Queue"}</h2><span class="lab">${counts}the agents' own: they file it and work it, and work in an area pulls in what's queued there${other}</span></div>
    ${table}</section>`;
}
