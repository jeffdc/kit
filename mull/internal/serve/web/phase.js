// A phase's sheet: where it stands, its items and what's queued for it,
// what needs you about it, and its doc and rules, which are edited in
// place. (Phase integration reviews are retired.)

import { esc, href, tag, cells, shortTitle, copyId } from "./html.js";
import { titleBlock } from "./title.js";
import { schedule } from "./needs.js";
import { renderDoc } from "./doc.js";
import { itemTable, rankTable, col } from "./lists.js";
import { editWrite } from "./write.js";
import { rulesSection } from "./rules.js";

// titleOf finds a phase's title in the project's tree (for needs, which the
// API gives as IDs).
function titleOf(data, id) {
  for (const pl of data.tree?.plans || []) for (const ph of pl.phases) if (ph.id === id) return ph.title;
  return id;
}

export function phaseSheet(pr, data, page) {
  const pid = pr.id, ph = page.phase, st = page.status, ch = data.changes;
  const p = st.progress, total = p.total - p.dropped;
  const link = id => `<a href="${esc(href.phase(pid, id))}">${esc(titleOf(data, id))}</a>`;
  const fields = [
    ["State", `${tag(st.state)}${st.go_live ? ` <span class="tag">Go live</span>` : ""}${st.blocking ? ` <span class="stamp">${st.blocking} blocking</span>` : ""}`],
    ["Items", total ? `${cells(p)} <span class="mono">${p.done}/${total}</span>` : `<span class="mut">none yet</span>`],
    ["ID", copyId(ph.id)],
    ["Needs", ph.needs?.length ? `<span>${ph.needs.map(link).join(", ")}</span>` : `<span class="mut">nothing</span>`],
    ["Needed by", page.needed_by.length ? `<span>${page.needed_by.map(n => `<a href="${esc(href.phase(pid, n.id))}">${esc(n.title)}</a>`).join(", ")}</span>` : `<span class="mut">nothing</span>`],
    ["Open questions", String(st.questions)],
  ];
  const crumbs = [`<a href="${href.home()}">All projects</a>`, `<a href="${esc(href.project(pid))}">${esc(pid)}</a>`, `<a href="${esc(href.plan(pid, ph.plan))}">${esc(shortTitle(page.plan_title))}</a>`];
  const head = titleBlock({ title: ph.title, crumbs, fields, long: true, cloud: ch ? ch.cls(c => c.type === "phase" && c.id === ph.id) : "" });
  const waits = st.state === "waits" && st.waits_for.length
    ? `<p class="sub" style="margin:0">Waits for ${st.waits_for.map(w => `<a href="${esc(href.phase(pid, w.id))}">${esc(w.title)}</a>`).join(", ")} to be done.</p>`
    : "";
  const captured = page.captured.length
    ? `<div class="hd" style="margin-top:24px"><h2>Queued for this phase</h2><span class="lab">The agents' queue · work here pulls it in</span></div>${itemTable(pid, page.captured, [col.id(), col.title(pid, "Queued"), col.kind(), col.area()], { changes: ch })}`
    : "";
  const docCloud = ch ? ch.cls(c => c.type === "phase_doc" && c.id === ph.id) : "";
  const doc = ph.doc
    ? renderDoc(ph.doc, pid, page.doc_refs)
    : `<div class="prose"><p class="ph-empty">Not written yet. The agent drafts it with you when this phase comes up.</p></div>`;
  const edit = editWrite({ pid, key: `edit-phasedoc:${pid}:${ph.id}`, ops: [{ op: "phase doc", id: ph.id }], field: "doc", long: true,
    value: ph.doc, label: ph.doc ? "Edit the doc" : "Write the doc", save: "Save the doc", done: `Saved the phase doc of ${ph.title}.`,
    placeholder: "Markdown. Its design and done-when: intent only, as status comes from the records." });
  return head + `<div class="page">
    ${ph.summary ? `<p class="lede">${esc(ph.summary)}</p>` : ""}${waits}
    <div class="cols">
      <section><div class="hd"><h2>Items</h2><span class="lab">One session each · in rank order</span></div>${rankTable(pid, page.items, ph.id, [col.id(), col.title(pid), col.stage()], { changes: ch, empty: "No items yet. They're written when this phase comes up." })}${captured}</section>
      <aside id="needs"><div class="hd"><h2>Needs you</h2><span class="lab">This phase</span></div>${schedule(page.needs_you, { pid, changes: ch, noPlan: true, noPhase: true })}</aside>
    </div>
    <section><div class="hd ed"><h2><span class="${docCloud.trim()}">The phase doc</span></h2><span class="lab">Its design, shared with the agents</span>${edit.button}</div><div class="doc-wrap">${edit.form || doc}</div></section>
    ${rulesSection({ pid, kind: "phase", key: `edit-phaserules:${pid}:${ph.id}`, op: { op: "phase rules", id: ph.id }, rules: ph.rules, title: ph.title,
      refs: page.doc_refs, cloud: ch ? ch.cls(c => c.type === "phase_rules" && c.id === ph.id) : "" })}
  </div>`;
}
