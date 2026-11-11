// An item's sheet: its title block, what to do with a captured one, what
// needs you about it, the review packet when it has a done report, and its
// notes. Read-first: actions copy the mull command.

import { esc, href, tag, itemStage, shortTitle, plural, clock, ago, base } from "./html.js";
import { titleBlock } from "./title.js";
import { schedule } from "./needs.js";
import { markdown } from "./doc.js";

const short = sha => esc(String(sha || "").slice(0, 7));

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
  const head = titleBlock({ title: it.title, crumbs, fields, long: true, cloud: ch ? ch.cls(c => c.type === "item" && c.id === it.id) : "" });
  const acts = it.state === "captured"
    ? `<div class="acts-row"><button class="btn p" data-cmd="mull start ${esc(it.id)}">Start it now</button><button class="btn" data-cmd="mull commit ${esc(it.id)} --plan …">Commit to a plan</button><button class="btn" data-cmd="mull drop ${esc(it.id)} --reason &quot;…&quot;">Drop</button><span class="sub">Starting it skips planning; it keeps this ID through review and done.</span></div>`
    : "";
  const ny = page.needs_you.tiers.length
    ? `<section><div class="hd"><h2>Needs you</h2><span class="lab">On this item</span></div>${schedule(page.needs_you, { pid, changes: ch, noPlan: true, noPhase: true })}</section>`
    : "";
  const docs = it.docs?.length ? `<p class="sub">Docs: ${it.docs.map(d => `<code>${esc(d)}</code>`).join(", ")}</p>` : "";
  const notes = `<section><div class="hd"><h2>Notes</h2><span class="lab">The brief an agent works from</span></div>
    <div class="prose notes">${it.notes ? markdown(it.notes, pid) : `<p class="ph-empty">${it.state === "captured" ? "One line is enough to capture. The agent who starts it writes what it needs here." : "No notes."}</p>`}</div>${docs}</section>`;
  return head + `<div class="page">${acts}${page.review ? review(pid, it, page) : ""}${ny}${notes}</div>`;
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
  let state;
  if (entry && !entry.settled) {
    state = `<button class="btn" data-cmd="mull do ${esc(it.id)} --reason &quot;…&quot;">Send it back</button>` +
      `<button class="btn p" data-cmd="mull accept ${esc(it.id)}" data-say="Accepting also ratifies ${esc(plural(open.length, "open decision"))}.">Accept${open.length ? ` with its ${plural(open.length, "decision")}` : ""}</button>`;
  } else if (entry?.settled) {
    const s = entry.settled;
    state = `${tag(s.action === "accept" ? "done" : "committed", s.action === "accept" ? "accepted" : s.action)} <span class="sub">${esc(clock(s.at))}${s.note ? ` · ${esc(s.note)}` : ""}</span>`;
  } else {
    state = `<span class="sub">Report ${r.n} ${r.passed ? "isn't filed for review" : "failed its checks, so it isn't filed for review"}.</span>`;
  }
  const earlier = page.reports.length > 1 ? `<span class="sub">${page.reports.map(x => `report ${x.n} ${x.passed ? "passed" : "failed"} ${esc(clock(x.at))}`).join(" · ")}</span>` : "";
  const checks = (r.checks?.results || []).map(c => `<div class="check${c.exit ? " bad" : ""}">${tag(c.exit ? "committed" : "pass", c.exit ? `exit ${c.exit}` : "passed")}<code>${esc(c.command)}</code><span class="sub">${esc(duration(c.duration_ms))}</span></div>`).join("")
    || `<div class="check">${tag(r.passed ? "pass" : "committed", r.passed ? "passed" : "failed")}<span class="sub">no checks configured</span></div>`;
  const tour = r.tour?.length && !(r.tour.length === 1 && r.tour[0] === "none")
    ? `<div><span class="lab">Try it</span><ol class="tour">${r.tour.map(t => `<li>${esc(t)}</li>`).join("")}</ol></div>`
    : `<div><span class="lab">Try it</span><p class="sub" style="margin:6px 0 0">No UI tour: nothing visible changed.</p></div>`;
  const decs = decisions.length ? decisions.map(d => {
    const s = d.settled;
    const act = s ? tag(s.action === "accept" ? "done" : "committed", s.action === "accept" ? "approved" : s.action) : `<button class="btn s" data-cmd="mull decide ${esc(d.id)} &quot;…&quot;">Decide differently</button>`;
    return `<div class="decn"><code>${esc(d.id)}</code><span>${esc(d.text)}</span>${act}${d.detail ? `<span class="why">${esc(d.detail)}</span>` : ""}${s?.note && s.action !== "accept" ? `<span class="why">${esc(s.note)}</span>` : ""}</div>`;
  }).join("") : `<p class="sub" style="margin:6px 0 0">None.</p>`;
  const files = (r.changes || []).map(l => {
    const add = l.files.reduce((n, f) => n + f.added, 0), del = l.files.reduce((n, f) => n + f.removed, 0);
    return `<tr class="grp-h"><td><b>${esc(l.layer)}</b></td><td class="n">${plural(l.files.length, "file")}</td><td class="n mono">+${add}</td><td class="n mono">−${del}</td></tr>` +
      l.files.map(f => `<tr><td><code class="mut">${esc(f.status)}</code> <code>${esc(f.path)}</code></td><td></td><td class="n mono">+${f.added}</td><td class="n mono">−${f.removed}</td></tr>`).join("");
  }).join("");
  return `<section class="review">
    <div class="rv-head"><h2>Review: does it work, and was it built well?</h2>${state}</div>
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
