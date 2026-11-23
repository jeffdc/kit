// A project's surface: what changed since the human last looked (GET
// …/surface), in the order it's acted on. Needs you (the calls, settled on
// their rows), then what's trending worse (rules, clusters of queued work,
// problems), then each plan's hill, then the flow of work, folded. Every
// number comes from mull's records and git. Mark seen, by the tabs, moves
// the last look, and the surface starts again from there.

import { esc, href, clock, plural, copyId, CHEVRON } from "./html.js";
import { calls } from "./needs.js";
import { problemsTable } from "./problems.js";
import { plansSection } from "./plans.js";

export function surfaceBody(pid, sf, changes) {
  const n = sf.calls.length;
  const needs = `<section id="needs"><div class="hd"><h2>Needs you</h2><span class="lab">${n ? `${n} · ` : ""}since you looked, ${esc(clock(sf.since))}</span></div>${calls(pid, sf.calls, sf.flow.landed)}</section>`;
  return needs + trending(pid, sf) + plansSection(pid, sf.plans, changes) + flow(pid, sf.flow);
}

// trending: the rules failing or worse since the last look, the rules with
// no check (one line), where queued work clusters, and the problems.
function trending(pid, sf) {
  const rules = sf.rules.filter(r => r.failing || r.trend === "worse");
  const parts = [];
  if (rules.length) {
    const rows = rules.map(r => {
      const link = href.rules(pid);
      return `<tr data-href="${esc(link)}"><td>${copyId(r.id, `<a href="${esc(link)}"><code>${esc(r.id)}</code></a>`)}</td><td class="ttl">${esc(r.title)}</td>` +
        `<td class="n mono">${r.trend === "new" ? "" : r.then}</td><td class="n mono">${r.now}</td><td>${r.failing ? "<b>failing</b>" : "worse"}</td></tr>`;
    }).join("");
    parts.push(`<div class="part"><span class="lab">Rules failing or worse · ${rules.length}</span><div class="tbl-wrap"><table class="t"><thead><tr><th style="width:70px">Rule</th><th>What it says</th><th class="n">Then</th><th class="n">Now</th><th>Trend</th></tr></thead><tbody>${rows}</tbody></table></div></div>`);
  }
  if (sf.clusters.length) {
    const rows = sf.clusters.map(c => `<tr><td><code>${esc(c.area)}</code><span class="meta">the design there may be wrong</span></td><td class="n mono">${c.count}</td>` +
      `<td>${c.items.map(id => copyId(id, `<a href="${esc(href.item(pid, id))}"><code>${esc(id)}</code></a>`)).join(" ")}</td></tr>`).join("");
    parts.push(`<div class="part"><span class="lab">Queued work clustering · ${sf.clusters.length}</span><div class="tbl-wrap"><table class="t"><thead><tr><th>Area</th><th class="n">Queued</th><th>Items</th></tr></thead><tbody>${rows}</tbody></table></div></div>`);
  }
  if (sf.problems.length) {
    parts.push(`<div class="part"><span class="lab">Problems with work · ${sf.problems.length}</span>${problemsTable(pid, sf.problems)}</div>`);
  }
  const checked = sf.rules.length - rules.length;
  const lines = [];
  if (!parts.length) lines.push(`<p class="empty">Nothing is trending worse.${checked ? ` ${plural(checked, "checked rule")} holding.` : ""}</p>`);
  else if (checked) lines.push(`<p class="sub">${plural(checked, "other checked rule")} holding.</p>`);
  if (sf.advisory.length) {
    const ids = sf.advisory.map(a => { const [id, ...t] = a.split(": "); return `<span title="${esc(t.join(": "))}">${esc(id)}</span>`; });
    lines.push(`<p class="sub">${sf.advisory.length === 1 ? "1 rule has" : `${sf.advisory.length} rules have`} no check, so nothing guards ${sf.advisory.length === 1 ? "it" : "them"}: ${ids.join(", ")}. <a href="${esc(href.rules(pid))}">The rulebook</a></p>`);
  }
  return `<section id="trend"><div class="hd"><h2>Trending worse</h2><span class="lab">From the checks and the records</span></div>${parts.join("")}${lines.join("")}</section>`;
}

const minutes = m => m < 60 ? `${m} min` : `${Math.floor(m / 60)} h${m % 60 ? ` ${m % 60} min` : ""}`;

// risk is how a landed item's band reads: shown to the human (medium), or
// drawn as a sample (low).
const risk = l => l.sampled ? `${l.risk}, sampled` : l.risk === "medium" ? "medium, shown" : l.risk;

// flow: how work moved since the last look, folded: what landed with its
// summary and risk, what slowed it, and what came back.
function flow(pid, f) {
  const facts = [`${f.landed.length} landed`,`${f.active} active of ${f.parallel} at once`,
    f.median_minutes ? `median ${minutes(f.median_minutes)} start to done` : "", f.rework ? `${f.rework} came back` : ""].filter(Boolean).join(" · ");
  const landed = f.landed.length
    ? `<div class="tbl-wrap"><table class="t"><thead><tr><th style="width:70px">ID</th><th>Landed</th><th>Risk</th><th class="n w">At</th></tr></thead><tbody>${f.landed.map(l => {
      const link = href.item(pid, l.item);
      return `<tr data-href="${esc(link)}"><td>${copyId(l.item)}</td><td><a class="ttl" href="${esc(link)}">${esc(l.title)}</a><span class="meta">${esc(l.summary)}</span></td><td class="mut">${esc(risk(l))}</td><td class="n mono mut w">${esc(clock(l.at))}</td></tr>`;
    }).join("")}</tbody></table></div>`
    : `<p class="empty">Nothing landed since you looked.</p>`;
  const slowed = f.slowed.length
    ? `<div class="part"><span class="lab">What slowed it · ${f.slowed.length}</span><div class="tbl-wrap"><table class="t"><tbody>${f.slowed.map(s =>
      `<tr data-href="${esc(href.item(pid, s.item))}"><td style="width:70px">${copyId(s.item)}</td><td>${esc(s.text)}</td></tr>`).join("")}</tbody></table></div></div>`
    : "";
  return `<section><details class="fold" data-open="flow:${esc(pid)}"><summary class="hd"><h2>${CHEVRON} Flow</h2><span class="lab">${esc(facts)}</span></summary>${landed}${slowed}</details></section>`;
}
