// Problems: what's wrong with work, as mull view lists it under PROBLEMS,
// found from the record and git (overlaps aside: they're pairs, shown in
// the Plans column and a plan's title block). A problem needs a look, not an
// answer, so it isn't in Needs you, and it takes no hue: ink and grey only.
// It's derived each time, never a record, so it's never clouded.

import { esc, href, tag, itemStage, shortTitle, copyId } from "./html.js";

// What each kind is called, and what usually puts it right.
const KIND = {
  "done-not-in-git": ["Not on the target", "Merge its work into the target, or find out why it isn't there."],
  "work-after-accept": ["Work after accept", "Later work needs its own item, a follow-up of this one."],
  "failed-done": ["Failed done", "The agent fixes the checks and runs done again, or releases the item."],
  "idle-claim": ["Idle claim", "Check on the agent, or have the item released."],
  "unclaimed-work": ["Unclaimed work", "Start it, or check the commits' Mull: trailer."],
};
const kindLabel = k => KIND[k]?.[0] || k;

// problemsSection: a schedule of problems on the project overview (every
// plan's and unplanned work's) or a plan's Status tab (its own, so inPlan
// leaves the plan out of each row). Nothing when there are none.
export function problemsSection(pid, rows, { inPlan = false } = {}) {
  if (!rows?.length) return "";
  const body = rows.map(p => {
    const link = href.item(pid, p.item);
    const where = inPlan ? (p.phase_title ? esc(p.phase_title) : "")
      : [p.plan ? `<a href="${esc(href.plan(pid, p.plan))}">${esc(shortTitle(p.plan_title || p.plan))}</a>` : `<a href="${esc(href.unplanned(pid))}">Unplanned</a>`,
        p.phase_title ? `<a href="${esc(href.phase(pid, p.phase))}">${esc(p.phase_title)}</a>` : ""].filter(Boolean).join(" · ");
    return `<tr data-href="${esc(link)}"><td>${copyId(p.item, `<a href="${esc(link)}"><code>${esc(p.item)}</code></a>`)}</td>` +
      `<td><a class="ttl" href="${esc(link)}">${esc(p.title)}</a>${where ? `<span class="meta">${where}</span>` : ""}</td>` +
      `<td class="w">${tag(itemStage(p))}</td>` +
      `<td><span class="lab">${esc(kindLabel(p.kind))}</span><span class="meta pb">${esc(p.text)}</span></td></tr>`;
  }).join("");
  return `<section id="problems"><div class="hd"><h2>Problems</h2><span class="lab">Found in the record and git · ${rows.length}</span></div>
    <div class="tbl-wrap"><table class="t probs"><thead><tr><th style="width:70px">ID</th><th>Item</th><th class="w">Stage</th><th>What's wrong</th></tr></thead><tbody>${body}</tbody></table></div></section>`;
}

// problemNotes: an item's own problems, each an ink note at the top of its
// sheet, with what usually puts it right.
export function problemNotes(rows) {
  return (rows || []).map(p => `<p class="note pbn"><span class="lab">Problem · ${esc(kindLabel(p.kind))}</span> ${esc(p.text)}.` +
    `${KIND[p.kind] ? ` <span class="mut">${esc(KIND[p.kind][1])}</span>` : ""}</p>`).join("");
}
