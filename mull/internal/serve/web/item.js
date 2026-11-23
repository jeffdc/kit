// An item's sheet, read-only: its title block, what's wrong with it (its
// problems), what's open on it, its done report when it has one
// (review.js), and its notes. Agents write items; the human acts on the
// project's surface (accepting, sending back, answering), which this sheet
// links to.

import { esc, href, tag, itemStage, shortTitle, plural, ago, base, copyId, day } from "./html.js";
import { titleBlock } from "./title.js";
import { markdown } from "./doc.js";
import { reportSection } from "./review.js";
import { problemNotes } from "./problems.js";

export function itemSheet(pr, data, page) {
  const pid = pr.id, it = page.item, ch = data.changes, g = page.git;
  const crumbs = [`<a href="${href.home()}">All projects</a>`, `<a href="${esc(href.project(pid))}">${esc(pid)}</a>`];
  if (it.plan) crumbs.push(`<a href="${esc(href.plan(pid, it.plan))}">${esc(shortTitle(page.plan_title || it.plan))}</a>`);
  else crumbs.push(it.state === "captured" ? `<a href="${esc(href.backlog(pid))}">Queue</a>` : `<a href="${esc(href.unplanned(pid))}">Unplanned</a>`);
  if (it.phase) crumbs.push(`<a href="${esc(href.phase(pid, it.phase))}">${esc(page.phase_title || it.phase)}</a>`);
  const waits = it.blocked_by?.length ? ` <small>waits for ${it.blocked_by.map(id => copyId(id)).join(", ")}</small>` : "";
  const fields = [
    ["Stage", `${tag(itemStage(it))}${waits}${page.archived ? ` <small>in the Archive</small>` : ""}`],
    ["ID", copyId(it.id)],
    ["Kind", it.kind ? `${esc(it.kind)}${it.area ? ` <small><code>${esc(it.area)}</code></small>` : ""}` : `<span class="mut">work item</span>`],
    claimField(it),
    ["From", it.source ? `<small>${esc(it.source)}</small>` : `<span class="mut">${it.state === "captured" ? "not given" : "planning"}</span>`],
    ["Git", gitField(g)],
  ];
  const head = titleBlock({ title: it.title, crumbs, fields, long: true, cloud: ch ? ch.cls(c => c.type === "item" && c.id === it.id) : "" });
  const docs = it.docs?.length ? `<p class="sub">Docs: ${it.docs.map(d => `<code>${esc(d)}</code>`).join(", ")}</p>` : "";
  const notes = `<section><div class="hd"><h2>Notes</h2><span class="lab">The brief an agent works from</span></div>
    <div class="prose notes">${it.notes ? markdown(it.notes, pid) : `<p class="ph-empty">No notes.</p>`}</div>${docs}</section>`;
  return head + `<div class="page">${problemNotes(page.problems)}${openOn(pid, page.needs_you)}${page.review ? reportSection(pid, page) : ""}${notes}</div>`;
}

// openOn: what's open on the item (a question, or its work waiting), as a
// note pointing at the surface, where it's settled.
function openOn(pid, ny) {
  const entries = (ny?.tiers || []).filter(t => t.key !== "decide").flatMap(t => t.entries || []);
  if (!entries.length) return "";
  const list = entries.map(e => `${copyId(e.id)} ${esc(e.kind === "review" ? "Its work waits for you" : e.text)}`).join("; ");
  return `<p class="note"><span class="lab">Needs you · ${plural(entries.length, "call")}</span> ${list}. <a href="${esc(href.project(pid))}" data-needs>Settle it on the surface</a>.</p>`;
}

// claimField: who holds the item while it's active; once the work is in
// review, done or dropped, the claim stays on the record (its done report
// is measured from it) but no one holds it, so it reads as where the work
// was done.
function claimField(it) {
  const c = it.claim;
  if (!c) return ["Claim", `<span class="mut">none</span>`];
  const where = `<code>${esc(base(c.worktree))}</code>`, branch = c.branch ? `${esc(c.branch)} · ` : "";
  if (it.state === "active") return ["Claim", `${where} <small>${branch}since ${esc(ago(c.at))}</small>`];
  return ["Worked in", `${where} <small>${branch}from ${esc(day(c.at))}</small>`];
}

// gitField: what git says about the item's work, measured against its
// plan's target branch: landed there (and when the commit that brought it
// in was made), or still only on other branches.
function gitField(g) {
  const target = `<code>${esc(g.target)}</code>`;
  if (!g.seen_in_git && !g.unmerged_commits) return `<span class="mut">no commits yet</span> <small>lands on ${target}</small>`;
  const parts = [g.seen_in_git ? `${plural(g.landed_commits, "commit")} on ${target}` : `not on ${target} yet`];
  if (g.unmerged_commits) parts.push(`${g.unmerged_commits} unmerged`);
  const notes = [];
  if (g.merged_at) notes.push(`merged ${esc(ago(g.merged_at))}`);
  else if (g.last_activity) notes.push(`last ${esc(ago(g.last_activity))}`);
  if (g.branches?.length) notes.push(g.branches.map(esc).join(", "));
  return `${parts.join(" · ")}${notes.length ? ` <small>${notes.join(" · ")}</small>` : ""}`;
}
