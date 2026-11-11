// The sheets: all projects, and within a project its Overview, Unplanned
// and Backlog tabs, and the plan, phase and item sheets. Each project sheet
// reads one or two API pages (subsFor), which app.js loads into the
// project's data.pages.

import { esc, href, agoSpan } from "./html.js";
import { titleBlock, projectHead, liveField } from "./title.js";
import { schedule } from "./needs.js";
import { plansColumn } from "./plans.js";
import { revisionSchedule } from "./revision.js";
import { unplannedBody, backlogBody } from "./lists.js";
import { planSubs, planSheet, planStatus, planDoc, planBacklog } from "./plan.js";
import { phaseSheet } from "./phase.js";
import { itemSheet } from "./item.js";

const enc = encodeURIComponent;

// subsFor: the API pages (paths under /api/projects/{id}) a route reads.
export function subsFor(r) {
  switch (r.kind) {
    case "": return [""];
    case "unplanned": case "backlog": return [r.kind];
    case "plans": return planSubs({ ...r, id: enc(r.id) });
    case "phases": case "items": return [`${r.kind}/${enc(r.id)}`];
  }
  return [];
}

const WHAT = { plans: "plan", phases: "phase", items: "item" };

// need draws an API page once it's loaded, or says it's loading or failed.
// A whole sheet (sheet: true) that isn't found is a Not found sheet.
function need(data, sub, draw, sheet = false) {
  const got = data?.pages?.get(sub);
  if (got?.value) return draw(got.value);
  if (got?.error) {
    if (sheet && got.error.status === 404) return pageNotFound(`${got.error.message}.`);
    const msg = failed(got.error);
    return sheet ? `<div class="page">${msg}</div>` : msg;
  }
  return sheet ? `<div class="page"><p class="empty">Loading…</p></div>` : `<p class="empty">Loading…</p>`;
}

// projectSheet: the sheet a route shows within a readable project.
export function projectSheet(state, pr, r) {
  const data = state.data.get(pr.id), pid = pr.id, ch = data?.changes;
  const [sub, sub2] = subsFor(r);
  switch (r.kind) {
    case "": return pageProject(state, pr);
    case "unplanned":
      return projectHead(pr, data, "unplanned", state.live) + `<div class="page">${need(data, sub, p => unplannedBody(pid, p, ch))}</div>`;
    case "backlog":
      return projectHead(pr, data, "backlog", state.live) + `<div class="page">${need(data, sub, p => backlogBody(pid, p, ch, { key: `project:${pid}` }))}</div>`;
    case "plans":
      return need(data, sub, page => planSheet(pr, r, data, page, tab =>
        tab === "doc" ? planDoc(pid, page)
          : tab === "backlog" ? need(data, sub2, bl => planBacklog(pid, r.id, bl, ch))
            : planStatus(pid, page, ch)), true);
    case "phases": return need(data, sub, page => phaseSheet(pr, data, page), true);
    case "items": return need(data, sub, page => itemSheet(pr, data, page), true);
  }
  return pageNotFound(`There's no ${WHAT[r.kind] || "sheet"} here.`);
}

const crumbHome = `<a href="${href.home()}">All projects</a>`;

export function pageHome(state) {
  const all = state.projects || [];
  const ok = all.filter(p => !p.error), bad = all.filter(p => p.error);
  const sum = f => ok.reduce((n, p) => n + f(p.summary), 0);
  const fields = [
    ["Projects on mull", String(ok.length)],
    ["Blocking", String(sum(s => s.blocking))],
    ["Waiting for review", String(sum(s => s.needs_you.reviews))],
    ["Agents working", String(sum(s => s.agents))],
    ["Status", liveField(state.live)],
    ["Not readable", String(bad.length)],
  ];
  const head = titleBlock({ title: "All projects", fields });
  if (!all.length) {
    return head + `<div class="page"><p class="empty">No projects yet. Run any mull command in a repository with a mull branch to add it.</p></div>`;
  }
  const rows = ok.map(pr => homeRow(pr, state.data.get(pr.id))).join("") + bad.map(errorRow).join("");
  return head + `<div class="page"><section><div class="hd"><h2>Projects</h2><span class="lab">Updates when any repo's mull branch moves</span></div>
    <div class="tbl-wrap"><table class="t"><thead><tr><th>Project</th><th>Active plans</th><th class="n">Blocking</th><th class="n">Reviews</th><th class="n">Questions</th><th class="n">Agents</th><th class="n">Untriaged</th><th class="n">Revision</th><th class="n">Last change</th></tr></thead>
    <tbody>${rows}</tbody></table></div></section></div>`;
}

function homeRow(pr, data) {
  const s = pr.summary, pid = pr.id, link = href.project(pid), rev = pr.revision;
  const tree = data?.tree;
  const plans = s.plans.filter(p => p.state === "active").map(p => {
    const phases = tree?.plans.find(t => t.slug === p.slug)?.phases;
    const now = phases?.filter(ph => ph.state === "now").map(ph => ph.title);
    const meta = !phases ? "" : `<span class="meta">${!phases.length ? "no phases" : `now: ${esc(now.join(", ") || "nothing can start")}`}</span>`;
    return `<div><a href="${esc(href.plan(pid, p.slug))}">${esc(p.title)}</a>${meta}</div>`;
  }).join("") || `<span class="mut">none</span>`;
  const branch = `<code>${esc(s.integration)}</code>${s.ahead_of_release ? ` · ${s.ahead_of_release} ahead of ${esc(s.release)}` : ""}`;
  const changed = rev?.changes ? (data?.changes ? data.changes.clsOf(data.changes.list) : " cl") : "";
  return `<tr class="${changed.trim()}" data-href="${esc(link)}"><td><a class="ttl" href="${esc(link)}" style="font-size:15px">${esc(pid)}</a><span class="meta">${branch}</span></td>` +
    `<td>${plans}</td><td class="n">${s.blocking ? `<span class="stamp">${s.blocking}</span>` : "0"}</td><td class="n">${s.needs_you.reviews}</td><td class="n">${s.needs_you.questions}</td>` +
    `<td class="n">${s.agents}</td><td class="n">${s.untriaged}</td><td class="n">${rev ? `Rev ${rev.number}` : ""}</td>` +
    `<td class="n mut">${agoSpan(s.last_change)}</td></tr>`;
}

function errorRow(pr) {
  const why = pr.error.code === "not_initialized" ? "No mull branch yet: run mull init in it to start." : pr.error.error;
  return `<tr class="waits"><td>${esc(pr.name)}<span class="meta">${esc(pr.path)}</span></td><td colspan="8"><code>${esc(pr.error.code)}</code> ${esc(why)}</td></tr>`;
}

// pageProject is a project's Overview: the revision schedule, then Needs
// you beside the Plans column.
export function pageProject(state, pr) {
  const data = state.data.get(pr.id);
  const head = projectHead(pr, data, "overview", state.live);
  const got = data?.pages?.get("");
  const ov = got?.value;
  if (!ov) return head + `<div class="page">${got?.error ? failed(got.error) : `<p class="empty">Loading…</p>`}</div>`;
  const ch = data.changes;
  const ny = ov.needs_you;
  const rows = new Map();
  for (const t of ny.tiers) for (const e of t.entries || t.groups?.flatMap(g => g.entries) || []) rows.set(e.id, e);
  const entryHref = id => {
    const e = rows.get(id);
    if (!e) return "";
    return e.item ? href.item(pr.id, e.item) : e.phase ? href.phase(pr.id, e.phase) : e.plan ? href.plan(pr.id, e.plan) : "";
  };
  const s = ov.summary;
  const migrate = s.migration_required
    ? `<p class="note">This project's mull data is format ${s.format}. mull reads it, but refuses every write until it's migrated: <code>mull migrate --dry-run --text</code> shows what would change.</p>`
    : "";
  return head + `<div class="page">${migrate}${revisionSchedule(pr.id, data.revision, entryHref)}<div class="cols">
    <section><div class="hd"><h2>Needs you</h2><span class="lab">${ny.open} open${ny.parked ? ` · ${ny.parked} parked` : ""} · ranked by what it unblocks</span></div>${schedule(ny, { pid: pr.id, changes: ch })}</section>
    <aside>${plansColumn(pr.id, ov, ch)}</aside>
  </div></div>`;
}

export function pageNotFound(what) {
  return titleBlock({ title: "Not found", crumbs: [crumbHome] }) +
    `<div class="page"><p class="empty">${esc(what)} <a href="${href.home()}">Back to all projects</a>.</p></div>`;
}

export function pageUnreadable(pr) {
  return titleBlock({ title: pr.name, crumbs: [crumbHome] }) +
    `<div class="page"><p class="note"><code>${esc(pr.error.code)}</code> ${esc(pr.error.error)}<br><span class="mut">${esc(pr.path)}</span></p></div>`;
}

export const failed = err => `<p class="note">Couldn't load this from mull serve: ${err.code ? `<code>${esc(err.code)}</code> ` : ""}${esc(err.message)}</p>`;
