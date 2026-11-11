// A plan's sheet: its title block and three tabs. Status is where it stands
// (the phases in schedule order, what needs you, what's next), Plan doc is
// the shared doc with its status margin, and Backlog is what's captured for
// it.

import { esc, href, tag, cells, plural, itemStage, clock, CHEVRON } from "./html.js";
import { titleBlock } from "./title.js";
import { within } from "./revision.js";
import { schedule } from "./needs.js";
import { renderDoc } from "./doc.js";
import { itemTable, col, backlogBody } from "./lists.js";

const stamp = n => n ? ` <span class="stamp">${n} blocking</span>` : "";
const crumbsFor = pid => [`<a href="${href.home()}">All projects</a>`, `<a href="${esc(href.project(pid))}">${esc(pid)}</a>`];

// planSubs: the API pages a plan route needs.
export const planSubs = r => [`plans/${r.id}`, ...(r.sub === "backlog" ? [`backlog?plan=${r.id}`] : [])];

export function planSheet(pr, r, data, page, body) {
  const pid = pr.id, slug = r.id, p = page.plan, st = page.status, ch = data.changes;
  const tab = r.sub === "doc" || r.sub === "backlog" ? r.sub : "status";
  const docCloud = ch ? ch.cls(c => c.type === "plan_doc" && c.id === slug) : "";
  const since = esc(clock(data.revision?.seen_at));
  const fields = [
    ["State", p.state === "shaping" ? `${tag("shaping")} <small>not agreed yet</small>` : p.state === "active" ? tag("now", "active") : tag(p.state)],
    ["Phases", st.phases ? `${st.built} of ${st.phases} done` : `<span class="mut">none yet</span>`],
    ["Can start now", st.now.length ? plural(st.now.length, "phase") : `<span class="mut">none</span>`],
    ["Go live", st.go_live ? `<a href="${esc(href.phase(pid, st.go_live.id))}">${esc(st.go_live.title)}</a>` : `<span class="mut">not set</span>`],
    ["Open questions", `${st.questions}${stamp(st.blocking)}`],
    ["Plan doc", !p.doc ? `<span class="mut">not written</span>`
      : docCloud ? `<a href="${esc(href.plan(pid, slug))}/doc">changed</a> <small>since ${since}</small>` : `<small>no changes since ${since}</small>`],
  ];
  const head = titleBlock({ title: p.title, crumbs: crumbsFor(pid), fields, long: true, cloud: ch ? ch.cls(c => c.type === "plan" && c.id === slug) : "" });
  const base = href.plan(pid, slug);
  const t = (key, label, link, count, cloud = "") => {
    const on = tab === key;
    return `<a class="${on ? "on" : ""}${cloud}" href="${esc(link)}"${on ? ' aria-current="page"' : ""}>${label}${count !== undefined ? ` <em>${esc(count)}</em>` : ""}</a>`;
  };
  const changes = data.revision?.changes.length || 0;
  const nav = `<nav class="tabs" aria-label="Plan sections">` +
    t("status", "Status", base, st.questions ? plural(st.questions, "question") : undefined) +
    t("doc", "Plan doc", `${base}/doc`, undefined, docCloud) +
    t("backlog", "Backlog", `${base}/backlog`, `${page.backlog}${page.untriaged ? ` · ${page.untriaged} untriaged` : ""}`) +
    `<div class="acts">${p.state === "shaping" ? `<button class="btn p" data-cmd="mull plan agree ${esc(slug)}" data-say="Agreeing a plan is yours to do.">Agree this plan</button>` : ""}` +
    `<button class="btn" data-seen="${esc(pid)}"${changes ? "" : " disabled"}>Mark revision seen</button></div></nav>`;
  return head + nav + `<div class="page">${body(tab)}</div>`;
}

export function planStatus(pid, page, ch) {
  const p = page.plan, slug = p.slug;
  const intro = p.state === "shaping" ? `<p class="sub" style="margin:0">A shaping plan has no phases until you agree it. Read and shape it in the Plan doc tab.</p>` : "";
  const table = phaseTable(pid, slug, page.schedule, ch);
  const unfiled = page.unfiled.length
    ? `<section><div class="hd"><h2>Not in a phase</h2><span class="lab">Committed to this plan without a phase</span></div>${itemTable(pid, page.unfiled, [col.id(), col.title(pid), col.stage()], { changes: ch })}</section>`
    : "";
  return `${intro}${table}<div class="cols">
    <section><div class="hd"><h2>Needs you</h2><span class="lab">This plan · ranked</span></div>${schedule(page.needs_you, { pid, changes: ch, noPlan: true })}</section>
    <aside><div class="hd"><h2>What's next</h2><span class="lab">Phases that can start now</span></div>${nextBlock(pid, page, ch)}</aside>
  </div>${unfiled}`;
}

// phaseTable: every phase, done ones folded into one group, then each step
// of the schedule.
function phaseTable(pid, slug, steps, ch) {
  if (!steps.some(s => s.phases.length)) return "";
  const rows = steps.map(s => {
    if (s.key === "done") {
      const group = `built-${slug}`;
      return `<tr class="grp-h"><td colspan="7"><details data-open="built:${esc(pid)}:${esc(slug)}" data-rows="${esc(group)}"><summary>${CHEVRON}${esc(s.title)}<em>${s.phases.length}</em></summary></details></td></tr>` +
        s.phases.map(ph => phaseRow(pid, ph, ch, group)).join("");
    }
    return `<tr class="grp-h${s.key === "now" ? " now" : ""}"><td colspan="7">${esc(s.title)}<em>${s.phases.length}</em></td></tr>` + s.phases.map(ph => phaseRow(pid, ph, ch)).join("");
  }).join("");
  return `<section><div class="hd"><h2>Where we are</h2><span class="lab">Phases in the order their needs allow</span></div>
    <div class="tbl-wrap"><table class="t"><thead><tr><th style="width:70px">ID</th><th>Phase</th><th>State</th><th>Items</th><th class="n">Done</th><th class="n">Questions</th><th>Needs</th></tr></thead><tbody>${rows}</tbody></table></div></section>`;
}

function phaseRow(pid, ph, ch, group) {
  const link = href.phase(pid, ph.id), p = ph.progress, total = p.total - p.dropped;
  const cls = [ph.state !== "now" ? "waits" : "", ch ? ch.cls(within.phase(ph.id)).trim() : ""].filter(Boolean).join(" ");
  return `<tr${cls ? ` class="${cls}"` : ""} data-href="${esc(link)}"${group ? ` data-group="${esc(group)}" hidden` : ""}><td><code>${esc(ph.id)}</code></td>` +
    `<td><a class="ttl" href="${esc(link)}">${esc(ph.title)}</a>${ph.go_live ? ` <span class="tag">Go live</span>` : ""}${ph.summary ? `<span class="meta">${esc(ph.summary)}</span>` : ""}</td>` +
    `<td>${tag(ph.state)}${stamp(ph.blocking)}</td><td>${cells(p)}</td>` +
    `<td class="n">${total ? `<span class="mono">${p.done}/${total}</span>` : `<span class="mut">no items</span>`}</td><td class="n">${ph.questions || ""}</td>` +
    `<td class="mono">${ph.needs.map(n => `<a href="${esc(href.phase(pid, n))}">${esc(n)}</a>`).join(" ")}</td></tr>`;
}

// nextBlock: each phase that can start now, with its work in flight and
// the first of what's ready.
function nextBlock(pid, page, ch) {
  if (!page.next.length) {
    return `<p class="empty">${page.plan.state === "shaping" ? "No phases yet. When you agree the plan, its phases appear here." : "Nothing can start."}</p>`;
  }
  return page.next.map(ph => {
    const open = ph.items.filter(i => i.state !== "done" && i.state !== "dropped");
    const show = open.filter(i => i.state !== "committed").concat(open.filter(i => i.state === "committed").slice(0, 2));
    const more = open.length - show.length, p = ph.progress, total = p.total - p.dropped;
    const rows = show.map(i => {
      const link = href.item(pid, i.id);
      return `<div class="ag nx${ch ? ch.cls(within.item(i.id)) : ""}" data-href="${esc(link)}"><a href="${esc(link)}"><code>${esc(i.id)}</code></a><a href="${esc(link)}" class="clip">${esc(i.title)}</a>${tag(itemStage(i))}</div>`;
    }).join("");
    return `<div class="plan"><div class="ph"><h3><a href="${esc(href.phase(pid, ph.id))}">${esc(ph.title)}</a>${stamp(ph.blocking)}</h3><span class="mono mut">${total ? `${p.done}/${total}` : ""}</span></div>
      <div class="nx-list">${rows || `<div class="sub">No open items.</div>`}${more ? `<div class="sub">and ${more} more ready</div>` : ""}</div></div>`;
  }).join("");
}

export function planDoc(pid, page) {
  const p = page.plan;
  const body = p.doc
    ? renderDoc(p.doc, pid, page.doc_refs)
    : `<div class="prose"><p class="ph-empty">No plan doc yet.${p.summary ? ` The summary: ${esc(p.summary)}` : ""}</p></div>`;
  return `<section><div class="hd"><h2>The plan</h2><span class="lab">Shared by you and the agents · intent only; status is in the margin</span></div><div class="doc-wrap">${body}</div></section>`;
}

export const planBacklog = (pid, slug, bl, ch) => backlogBody(pid, bl, ch, { key: `plan:${pid}:${slug}`, inPlan: slug });
