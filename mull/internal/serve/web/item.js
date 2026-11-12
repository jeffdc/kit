// An item's sheet: its title block, what to do with a captured one, what
// needs you about it, the review packet when it has a done report, and its
// notes. Editing its title and notes, dropping a captured item, accepting
// or sending back a review, and deciding a decision differently, write
// (write.js); starting an item, which hands it to an agent, copies the mull
// command.

import { esc, href, tag, itemStage, shortTitle, plural, clock, ago, base } from "./html.js";
import { titleBlock } from "./title.js";
import { schedule, decideDifferently } from "./needs.js";
import { markdown } from "./doc.js";
import { writeButton, writeState, textWrite, editWrite, failure } from "./write.js";

const short = sha => esc(String(sha || "").slice(0, 7));

// How a settled review or decision reads: the action as the human took it.
const SETTLED = { accept: "accepted", do: "sent back", decide: "decided", drop: "dropped", park: "parked" };

function duration(ms) {
  const s = Math.round(ms / 1000);
  return s < 60 ? `${s} s` : `${Math.floor(s / 60)} m ${String(s % 60).padStart(2, "0")} s`;
}

export function itemSheet(pr, data, page) {
  const pid = pr.id, it = page.item, ch = data.changes, g = page.git;
  const crumbs = [`<a href="${href.home()}">All projects</a>`, `<a href="${esc(href.project(pid))}">${esc(pid)}</a>`];
  if (it.plan) crumbs.push(`<a href="${esc(href.plan(pid, it.plan))}">${esc(shortTitle(page.plan_title || it.plan))}</a>`);
  else crumbs.push(it.state === "captured" ? `<a href="${esc(href.backlog(pid))}">Backlog</a>` : `<a href="${esc(href.unplanned(pid))}">Unplanned</a>`);
  if (it.phase) crumbs.push(`<a href="${esc(href.phase(pid, it.phase))}">${esc(page.phase_title || it.phase)}</a>`);
  const waits = it.blocked_by?.length ? ` <small>waits for ${esc(it.blocked_by.join(", "))}</small>` : "";
  const fields = [
    ["Stage", `${tag(itemStage(it))}${waits}`],
    ["ID", `<code>${esc(it.id)}</code>`],
    ["Kind", it.kind ? `${esc(it.kind)}${it.category ? ` <small>${esc(it.category)}</small>` : ""}` : `<span class="mut">work item</span>`],
    ["Claim", it.claim ? `<code>${esc(base(it.claim.worktree))}</code> <small>${it.claim.branch ? `${esc(it.claim.branch)} · ` : ""}since ${esc(ago(it.claim.at))}</small>` : `<span class="mut">none</span>`],
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
  return head + `<div class="page">${acts}${ny}${page.review ? review(pid, it, page) : ""}${notes}</div>`;
}

function gitField(g) {
  if (!g.seen_in_git) return `<span class="mut">no commits yet</span>`;
  const parts = [plural(g.landed_commits, "landed commit")];
  if (g.unmerged_commits) parts.push(`${g.unmerged_commits} unmerged`);
  const branches = g.branches?.length ? ` · ${g.branches.map(esc).join(", ")}` : "";
  return `${parts.join(" · ")} <small>${g.last_activity ? `last ${esc(ago(g.last_activity))}` : ""}${branches}</small>`;
}

// review: the packet, what was built (does it work?) beside how (was it
// built well?).
function review(pid, it, page) {
  const { entry, report: r, decisions } = page.review;
  const open = decisions.filter(d => !d.settled);
  let state, below = "";
  if (entry && !entry.settled) {
    // Accepting ratifies the open decisions it carries: armed, it lists them.
    const key = `accept:${pid}:${it.id}`;
    const accept = writeButton({ pid, key, ops: [{ op: "accept", ids: [it.id] }], small: false,
      label: `Accept${open.length ? ` with its ${plural(open.length, "decision")}` : ""}`, ask: open.length ? `Accept and ratify ${open.length}?` : "Accept?",
      done: `Accepted ${it.id}${open.length ? ` and ratified ${plural(open.length, "decision")}` : ""}. It's done.` });
    const back = textWrite({ pid, key: `sendback:${pid}:${it.id}`, ops: [{ op: "do", id: entry.id }], field: "reason", small: false,
      label: "Send it back", save: "Send it back", done: `Sent ${it.id} back. It reopens for another session.`,
      placeholder: "What's wrong. It goes into the item's notes for the next session; its decisions stay open." });
    state = back.button + accept;
    const ratifies = writeState(key).armed
      ? `<p class="rv-note">${open.length ? `Accepting also ratifies ${open.map(d => `<code>${esc(d.id)}</code> ${esc(d.text)}`).join("; ")}. If one isn't right, decide it differently instead: the work goes back to an agent for the change.` : "Accepting marks it done; it carries no open decisions."}</p>`
      : "";
    below = ratifies + failure(key, `Accepting ${it.id}`) + back.form;
  } else if (entry?.settled) {
    const s = entry.settled;
    state = `${tag(s.action === "accept" ? "done" : "committed", SETTLED[s.action] || s.action)} <span class="sub">${esc(clock(s.at))}${s.note ? ` · ${esc(s.note)}` : ""}</span>`;
  } else {
    state = `<span class="sub">Report ${r.n} ${r.passed ? "isn't filed for review" : "failed its checks, so it isn't filed for review"}.</span>`;
  }
  const earlier = page.reports.length > 1 ? `<span class="sub">${page.reports.map(x => `report ${x.n} ${x.passed ? "passed" : "failed"} ${esc(clock(x.at))}`).join(" · ")}</span>` : "";
  const checks = (r.checks?.results || []).map(c => `<div class="check${c.exit ? " bad" : ""}">${tag(c.exit ? "committed" : "pass", c.exit ? `exit ${c.exit}` : "passed")}<code>${esc(c.command)}</code><span class="sub">${esc(duration(c.duration_ms))}</span></div>`).join("")
    || `<div class="check">${tag(r.passed ? "pass" : "committed", r.passed ? "passed" : "failed")}<span class="sub">no checks configured</span></div>`;
  const tour = r.tour?.length && !(r.tour.length === 1 && r.tour[0] === "none")
    ? `<div><span class="lab">Try it</span><ol class="tour">${r.tour.map(t => `<li>${esc(t)}</li>`).join("")}</ol></div>`
    : `<div><span class="lab">Try it</span><p class="sub" style="margin:6px 0 0">No UI tour: nothing visible changed.</p></div>`;
  // Each open decision is approved on its own, apart from accepting the
  // work; deciding one differently while the work waits sends it back.
  const waiting = !!(entry && !entry.settled);
  const decs = decisions.length ? decisions.map(d => {
    const s = d.settled;
    const dd = s ? null : decideDifferently(pid, d.id, "packet", waiting);
    const ak = `approve:packet:${pid}:${d.id}`;
    const act = s ? tag(s.action === "accept" ? "done" : "committed", s.action === "accept" ? "approved" : SETTLED[s.action] || s.action)
      : `<span class="decn-acts">${writeButton({ pid, key: ak, ops: [{ op: "accept", ids: [d.id] }], label: "Approve", ask: "Approve?", done: `Approved decision ${d.id}.` })}${dd.button}</span>`;
    return `<div class="decn"><code>${esc(d.id)}</code><span>${esc(d.text)}</span>${act}${d.detail ? `<span class="why">${esc(d.detail)}</span>` : ""}${s?.note && s.action !== "accept" ? `<span class="why">${esc(s.note)}</span>` : ""}${s ? "" : failure(ak, `Approving ${d.id}`)}${dd ? dd.form : ""}</div>`;
  }).join("") : `<p class="sub" style="margin:6px 0 0">None.</p>`;
  const files = (r.changes || []).map(l => {
    const add = l.files.reduce((n, f) => n + f.added, 0), del = l.files.reduce((n, f) => n + f.removed, 0);
    return `<tr class="grp-h"><td><b>${esc(l.layer)}</b></td><td class="n">${plural(l.files.length, "file")}</td><td class="n mono">+${add}</td><td class="n mono">−${del}</td></tr>` +
      l.files.map(f => `<tr><td><code class="mut">${esc(f.status)}</code> <code>${esc(f.path)}</code></td><td></td><td class="n mono">+${f.added}</td><td class="n mono">−${f.removed}</td></tr>`).join("");
  }).join("");
  return `<section class="review">
    <div class="rv-head"><h2>Review: does it work, and was it built well?</h2>${state}</div>${below}
    <div class="pk">
      <section><div class="hd"><h2>What was built</h2><span class="lab">Does it work?</span></div>
        <div class="prose">${markdown(r.summary, pid)}</div>
        ${tour}
        ${checks}
        <span class="sub">Report ${r.n} · <code>${short(r.commit)}</code> from <code>${short(r.base)}</code> · ${esc(clock(r.at))}</span>${earlier}</section>
      <section><div class="hd"><h2>How it was built</h2><span class="lab">Sound design?</span></div>
        <div class="prose">${markdown(r.design, pid)}</div>
        <div><span class="lab">Decisions made in this session · ${decisions.length || "none"}</span>${decs}</div>
        ${files ? `<div><span class="lab">Files by layer</span><div class="tbl-wrap"><table class="t files"><tbody>${files}</tbody></table></div></div>` : ""}
      </section>
    </div></section>`;
}
