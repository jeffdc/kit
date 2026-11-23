// An item's sheet: its title block, what's wrong with it (its problems),
// what to do with a captured one, what needs you about it, the review
// packet when it has a done report (review.js), and its notes. Editing its
// title and notes, dropping a captured item, accepting or sending back a
// review, and deciding a decision differently, write (write.js); starting
// an item, which hands it to an agent, copies the mull command.

import { esc, href, tag, itemStage, shortTitle, plural, ago, base, copyId, day } from "./html.js";
import { titleBlock } from "./title.js";
import { schedule } from "./needs.js";
import { markdown } from "./doc.js";
import { textWrite, editWrite } from "./write.js";
import { reviewPacket } from "./review.js";
import { problemNotes } from "./problems.js";

export function itemSheet(pr, data, page) {
  const pid = pr.id, it = page.item, ch = data.changes, g = page.git;
  const crumbs = [`<a href="${href.home()}">All projects</a>`, `<a href="${esc(href.project(pid))}">${esc(pid)}</a>`];
  if (it.plan) crumbs.push(`<a href="${esc(href.plan(pid, it.plan))}">${esc(shortTitle(page.plan_title || it.plan))}</a>`);
  else crumbs.push(it.state === "captured" ? `<a href="${esc(href.backlog(pid))}">Backlog</a>` : `<a href="${esc(href.unplanned(pid))}">Unplanned</a>`);
  if (it.phase) crumbs.push(`<a href="${esc(href.phase(pid, it.phase))}">${esc(page.phase_title || it.phase)}</a>`);
  const waits = it.blocked_by?.length ? ` <small>waits for ${it.blocked_by.map(id => copyId(id)).join(", ")}</small>` : "";
  const fields = [
    ["Stage", `${tag(itemStage(it))}${waits}${page.archived ? ` <small>in the Archive</small>` : ""}`],
    ["ID", copyId(it.id)],
    ["Kind", it.kind ? `${esc(it.kind)}${it.category ? ` <small>${esc(it.category)}</small>` : ""}` : `<span class="mut">work item</span>`],
    claimField(it),
    ["From", it.source ? `<small>${esc(it.source)}</small>` : `<span class="mut">${it.state === "captured" ? "not given" : "planning"}</span>`],
    ["Git", gitField(g)],
  ];
  // Its title and notes are edited in place, captured or committed: the
  // backlog is where Jeff writes things down before an agent is involved.
  const editTitle = editWrite({ pid, key: `edit-title:${pid}:${it.id}`, ops: [{ op: "set", id: it.id, field: "title" }], field: "value",
    value: it.title, required: true, label: "Edit title", save: "Save title", placeholder: "What it is, in a line.", done: `Saved the title of ${it.id}.` });
  const head = titleBlock({ title: it.title, crumbs, fields, long: true, edit: editTitle, cloud: ch ? ch.cls(c => c.type === "item" && c.id === it.id) : "" });
  // A captured item: start it (a copy, as starting hands it to an agent) or
  // drop it (a write). Committing it and triage happen with an agent.
  const drop = it.state === "captured" ? textWrite({ pid, key: `dropitem:${pid}:${it.id}`, ops: [{ op: "drop", ids: [it.id] }], field: "reason", small: false,
    label: "Drop", save: "Drop it for good", done: `Dropped ${it.id}.`,
    placeholder: "Why it isn't needed. It goes into the item's notes; dropping can't be undone." }) : null;
  const acts = drop
    ? `<div><div class="acts-row"><button class="btn p" data-cmd="mull start ${esc(it.id)}">Start it now</button>${drop.button}<span class="sub">Starting it skips planning; it keeps this ID through review and done. Committing it to a plan, and triage, happen with an agent.</span></div>${drop.form}</div>`
    : "";
  const ny = page.needs_you.tiers.length
    ? `<section id="needs"><div class="hd"><h2>Needs you</h2><span class="lab">On this item</span></div>${schedule(page.needs_you, { pid, changes: ch, noPlan: true, noPhase: true })}</section>`
    : "";
  const docs = it.docs?.length ? `<p class="sub">Docs: ${it.docs.map(d => `<code>${esc(d)}</code>`).join(", ")}</p>` : "";
  const editNotes = editWrite({ pid, key: `edit-notes:${pid}:${it.id}`, ops: [{ op: "set", id: it.id, field: "notes" }], field: "value", long: true,
    value: it.notes, label: it.notes ? "Edit notes" : "Write notes", save: "Save notes", done: `Saved the notes of ${it.id}.`,
    placeholder: "Markdown. What it is and why, and anything the agent who takes it on should know." });
  const notes = `<section><div class="hd ed"><h2>Notes</h2><span class="lab">The brief an agent works from</span>${editNotes.button}</div>
    ${editNotes.form || `<div class="prose notes">${it.notes ? markdown(it.notes, pid) : `<p class="ph-empty">${it.state === "captured" ? "One line is enough to capture. Write more here whenever you like, or leave it to the agent who starts it." : "No notes."}</p>`}</div>`}${docs}</section>`;
  return head + `<div class="page">${problemNotes(page.problems)}${acts}${ny}${page.review ? reviewPacket(pid, page, { it }) : ""}${notes}</div>`;
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
