// The Plans column of a project's overview: for each plan not done, what's
// built, what can start now, what's next, the go-live phase and the agents
// at work, as a labelled definition list.

import { esc, href, tag, cells, plural, ago } from "./html.js";
import { within } from "./revision.js";

const stamp = n => n ? `<span class="stamp">${n} blocking</span>` : "";

export function plansColumn(pid, overview, changes) {
  const plans = overview.plans || [];
  const loose = (overview.agents || []).filter(a => !a.plan);
  const blocks = plans.map(ps => plan(pid, ps, changes));
  if (loose.length) {
    blocks.push(`<div class="plan"><div class="ph"><h3><a href="${esc(href.unplanned(pid))}">Unplanned work</a></h3></div>
      <div class="dl"><span class="lab">Agents</span><div>${loose.map(a => agent(pid, a, changes)).join("")}</div></div></div>`);
  }
  const body = blocks.length ? blocks.join("") : `<p class="empty">No plans in progress.</p>`;
  return `<div class="hd"><h2>Plans</h2><span class="lab">${plural(plans.length, "plan")} in progress</span></div>${body}`;
}

function plan(pid, ps, ch) {
  const rows = [];
  const cl = pred => ch ? ch.cls(pred) : "";
  if (ps.state === "shaping") {
    rows.push(["State", `<div>Plan doc in progress<div class="sub">${plural(ps.questions, "open question")} · ${ps.phases ? plural(ps.phases, "phase") : "no phases yet"}</div></div>`]);
  } else {
    if (ps.built) rows.push(["Built", `<div class="sub">${ps.built} of ${plural(ps.phases, "phase")} done</div>`]);
    if (ps.now.length) {
      rows.push([`Now · ${ps.now.length}`, `<div>${ps.now.map(ph => {
        const p = ph.progress, total = p.total - p.dropped, link = href.phase(pid, ph.id);
        return `<div class="pr${cl(within.phase(ph.id))}" data-href="${esc(link)}"><a href="${esc(link)}"><span>${esc(ph.title)}</span>${stamp(ph.blocking)}</a>${cells(p)}<em>${total ? `${p.done}/${total}` : ""}</em></div>`;
      }).join("")}</div>`]);
    }
    if (ps.next.length) {
      rows.push(["Next", `<div>${ps.next.map(ph => `<div><a class="${cl(within.phase(ph.id)).trim()}" href="${esc(href.phase(pid, ph.id))}">${esc(ph.title)}</a>` +
        `<div class="sub">waits for ${ph.waits_for.map(w => esc(w.title)).join(", ")}</div></div>`).join("")}</div>`]);
    }
    const gl = ps.go_live;
    if (gl) {
      const w = gl.waits_for.length;
      const state = gl.state === "done" ? "done" : w ? `waits for ${plural(w, "phase")}` : "can start";
      rows.push(["Go live", `<div><a class="${cl(within.phase(gl.id)).trim()}" href="${esc(href.phase(pid, gl.id))}">${esc(gl.title)}</a><div class="sub">${state}</div></div>`]);
    }
    if (!ps.phases) rows.push(["Phases", `<div class="sub">none yet</div>`]);
    else if (!ps.now.length && !ps.next.length && !gl && ps.built < ps.phases) rows.push(["Now", `<div class="sub">nothing can start</div>`]);
  }
  if (ps.agents.length) rows.push(["Agents", `<div>${ps.agents.map(a => agent(pid, a, ch)).join("")}</div>`]);
  const state = ps.state === "active" ? tag("now", "active") : tag(ps.state);
  return `<div class="plan"><div class="ph"><h3><a class="${cl(within.planRecord(ps.slug)).trim()}" href="${esc(href.plan(pid, ps.slug))}">${esc(ps.title)}</a></h3>${state}</div>
    <div class="dl">${rows.map(([l, v]) => `<span class="lab">${esc(l)}</span>${v}`).join("")}</div></div>`;
}

function agent(pid, a, ch) {
  const link = href.item(pid, a.id);
  const since = a.claim?.at ? ago(a.claim.at).replace(/ ago$/, "") : "";
  const sub = [since, plural(a.commits, "commit")].filter(Boolean).join(" · ");
  return `<div class="ag${ch ? ch.cls(within.item(a.id)) : ""}" data-href="${esc(link)}"><a href="${esc(link)}"><code>${esc(a.id)}</code></a>` +
    `<span title="${esc(a.phase_title ? `${a.phase_title}: ${a.title}` : a.title)}">${esc(a.title)}</span><span class="sub">${esc(sub)}</span></div>`;
}
