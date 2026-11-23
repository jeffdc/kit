// The review packet: what was built (does it work?) beside how (was it
// built well?), with Accept and Send it back while the review is open. An
// item's sheet shows its review's; a phase's sheet shows its integration
// review's, which covers the phase's done items as one combined diff on
// the target, and carries no decisions. Accepting and sending back write
// (write.js), as does settling each decision a review carries.

import { esc, href, tag, plural, clock, copyId, itemStage } from "./html.js";
import { decideDifferently } from "./needs.js";
import { markdown } from "./doc.js";
import { writeButton, writeState, textWrite, failure } from "./write.js";

const short = sha => esc(String(sha || "").slice(0, 7));

// How a settled review or decision reads: the action as the human took it.
const SETTLED = { accept: "accepted", do: "sent back", decide: "decided", drop: "dropped", park: "parked" };

function duration(ms) {
  const s = Math.round(ms / 1000);
  return s < 60 ? `${s} s` : `${Math.floor(s / 60)} m ${String(s % 60).padStart(2, "0")} s`;
}

// tour: the report's steps, each path in them a link to the plan's
// preview, which serves the target the work lands on (the server splits
// the steps: review.tour). Opened in a new tab, so the packet stays put
// while Jeff walks it. Done's warnings about the tour (a local URL that
// isn't the preview) follow it, in ink. onTarget: whether the work is on
// the target yet (a phase's combined work always is: it's filed there).
function tour(r, rv, target, onTarget) {
  const warns = (r.warnings || []).map(w => `<p class="rv-note tour-warn"><b>Warning:</b> ${esc(w)}</p>`).join("");
  if (!r.tour?.length || (r.tour.length === 1 && r.tour[0] === "none"))
    return `<div><span class="lab">Try it</span><p class="sub" style="margin:6px 0 0">No UI tour: nothing visible changed.</p>${warns}</div>`;
  const steps = rv.tour || r.tour.map(text => [{ text }]);
  const step = parts => parts.map(p => p.url ? `<a href="${esc(p.url)}" target="_blank" rel="noopener">${esc(p.text)}</a>` : esc(p.text)).join("");
  const linked = steps.some(parts => parts.some(p => p.url));
  const where = rv.preview
    ? `<p class="sub" style="margin:6px 0 0">${linked ? "Paths open on" : "On"} the preview, <a class="pv" href="${esc(rv.preview)}" target="_blank" rel="noopener">${esc(rv.preview)}</a>, which serves <code>${esc(target)}</code>${onTarget ? "" : `: this work isn't on it yet`}.</p>`
    : "";
  return `<div><span class="lab">Try it</span><ol class="tour">${steps.map(parts => `<li>${step(parts)}</li>`).join("")}</ol>${where}${warns}</div>`;
}

// reviewPacket draws page.review for an item (it: the item; the page's
// git says where its work is) or a phase (phase: the phase; page.items its
// items as they stand now). Either way page.reports lists every attempt.
export function reviewPacket(pid, page, { it, phase }) {
  const rv = page.review, { entry, report: r, decisions } = rv;
  const id = it ? it.id : phase.id;
  const open = decisions.filter(d => !d.settled);
  let state, below = "";
  if (entry && !entry.settled) {
    // Accepting an item's review ratifies the open decisions it carries:
    // armed, it lists them. A phase's carries none; accepting it completes
    // the phase.
    const key = `accept:${pid}:${id}`;
    const accept = writeButton({ pid, key, ops: [{ op: "accept", ids: [it ? it.id : entry.id] }], small: false,
      label: `Accept${open.length ? ` with its ${plural(open.length, "decision")}` : ""}`, ask: open.length ? `Accept and ratify ${open.length}?` : "Accept?",
      done: it ? `Accepted ${it.id}${open.length ? ` and ratified ${plural(open.length, "decision")}` : ""}. It's done.`
        : `Accepted the integration review of ${phase.title}. The phase is complete.` });
    const back = textWrite({ pid, key: `sendback:${pid}:${id}`, ops: [{ op: "do", id: entry.id }], field: "reason", small: false,
      label: "Send it back", save: "Send it back",
      done: it ? `Sent ${it.id} back. It reopens for another session.` : `Sent the integration review of ${phase.title} back. Its fix-up item is in the phase.`,
      placeholder: it ? "What's wrong. It goes into the item's notes for the next session; its decisions stay open."
        : "What to fix. It becomes a fix-up item in this phase (its first line the title), and the phase reopens until that's done and reviewed again." });
    state = back.button + accept;
    const note = it
      ? open.length ? `Accepting also ratifies ${open.map(d => `${copyId(d.id)} ${esc(d.text)}`).join("; ")}. If one isn't right, decide it differently instead: the work goes back to an agent for the change.` : "Accepting marks it done; it carries no open decisions."
      : "Accepting completes the phase, so the phases that need it can start.";
    below = (writeState(key).armed ? `<p class="rv-note">${note}</p>` : "") + failure(key, `Accepting ${it ? it.id : "the integration review"}`) + back.form;
  } else if (entry?.settled) {
    const s = entry.settled;
    const fix = s.action === "do" && s.item && phase ? ` · fix-up ${copyId(s.item, `<a href="${esc(href.item(pid, s.item))}"><code>${esc(s.item)}</code></a>`)}` : "";
    state = `${tag(s.action === "accept" ? "done" : "committed", SETTLED[s.action] || s.action)} <span class="sub">${esc(clock(s.at))}${s.note ? ` · ${esc(s.note)}` : ""}${fix}</span>`;
  } else {
    state = `<span class="sub">Report ${r.n} ${r.passed ? "isn't filed for review" : "failed its checks, so it isn't filed for review"}.</span>`;
  }
  const earlier = page.reports.length > 1 ? `<span class="sub">${page.reports.map(x => `report ${x.n} ${x.passed ? "passed" : "failed"} ${esc(clock(x.at))}`).join(" · ")}</span>` : "";
  const checks = (r.checks?.results || []).map(c => `<div class="check${c.exit ? " bad" : ""}">${tag(c.exit ? "committed" : "pass", c.exit ? `exit ${c.exit}` : "passed")}<code>${esc(c.command)}</code><span class="sub">${esc(duration(c.duration_ms))}</span></div>`).join("")
    || `<div class="check">${tag(r.passed ? "pass" : "committed", r.passed ? "passed" : "failed")}<span class="sub">no checks configured</span></div>`;
  // An item's review: each open decision is approved on its own, apart
  // from accepting the work; deciding one differently while the work waits
  // sends it back.
  const waiting = !!(entry && !entry.settled);
  const decs = !it ? "" : decisions.length ? decisions.map(d => {
    const s = d.settled;
    const dd = s ? null : decideDifferently(pid, d.id, "packet", waiting);
    const ak = `approve:packet:${pid}:${d.id}`;
    const act = s ? tag(s.action === "accept" ? "done" : "committed", s.action === "accept" ? "approved" : SETTLED[s.action] || s.action)
      : `<span class="decn-acts">${writeButton({ pid, key: ak, ops: [{ op: "accept", ids: [d.id] }], label: "Approve", ask: "Approve?", done: `Approved decision ${d.id}.`, confirm: false })}${dd.button}</span>`;
    return `<div class="decn">${copyId(d.id)}<span>${esc(d.text)}</span>${act}${d.detail ? `<span class="why">${esc(d.detail)}</span>` : ""}${s?.note && s.action !== "accept" ? `<span class="why">${esc(s.note)}</span>` : ""}${s ? "" : failure(ak, `Approving ${d.id}`)}${dd ? dd.form : ""}</div>`;
  }).join("") : `<p class="sub" style="margin:6px 0 0">None.</p>`;
  // A phase's review: the done items it covers, each with its stage now.
  const now = new Map((page.items || []).map(x => [x.id, x]));
  const covers = it ? "" : `<div><span class="lab">Items it covers · ${r.items?.length || "none"}</span>${(r.items || []).map(x => {
    const cur = now.get(x.id);
    return `<div class="decn cov">${copyId(x.id, `<a href="${esc(href.item(pid, x.id))}"><code>${esc(x.id)}</code></a>`)}<span>${esc(cur?.title || x.title)}</span>${cur ? tag(itemStage(cur)) : ""}</div>`;
  }).join("")}</div>`;
  // Changes to the plan's shared files (its shared globs, as done found
  // them) are flagged: a box in ink above the decisions, and a word in the
  // files table. Ink only: they need a look, not an answer.
  const sharedPaths = new Set((r.shared || []).map(f => f.path));
  const shared = r.shared?.length
    ? `<div class="shared"><span class="lab">Shared files changed · ${r.shared.length}</span>
        <p class="sub">The plan lists these as shared by its parallel work. Check the change suits everything that uses them.</p>
        ${r.shared.map(f => `<div class="sh"><code class="mut">${esc(f.status)}</code><code>${esc(f.path)}</code><span class="sub">shared: <code>${esc(f.glob)}</code></span></div>`).join("")}</div>`
    : "";
  const files = (r.changes || []).map(l => {
    const add = l.files.reduce((n, f) => n + f.added, 0), del = l.files.reduce((n, f) => n + f.removed, 0);
    return `<tr class="grp-h"><td><b>${esc(l.layer)}</b></td><td class="n">${plural(l.files.length, "file")}</td><td class="n mono">+${add}</td><td class="n mono">−${del}</td></tr>` +
      l.files.map(f => `<tr><td><code class="mut">${esc(f.status)}</code> <code>${esc(f.path)}</code>${sharedPaths.has(f.path) ? ` <span class="sub">shared</span>` : ""}</td><td></td><td class="n mono">+${f.added}</td><td class="n mono">−${f.removed}</td></tr>`).join("");
  }).join("");
  const filesLabel = it ? `<span class="lab">Files by layer</span>`
    : `<span class="lab">Combined diff by layer</span><p class="sub" style="margin:4px 0 0">On <code>${esc(rv.target)}</code>, from <code>${short(r.base)}</code> to <code>${short(r.commit)}</code>, in the files its items changed.</p>`;
  const title = it ? "Review: does it work, and was it built well?" : "Integration review: does the phase hold together?";
  const target = it ? page.git.target : rv.target, onTarget = it ? page.git.seen_in_git : true;
  return `<section class="review">
    <div class="rv-head"><h2>${title}</h2>${state}</div>${below}
    <div class="pk">
      <section><div class="hd"><h2>What was built</h2><span class="lab">Does it work?</span></div>
        <div class="prose">${markdown(r.summary, pid)}</div>
        ${tour(r, rv, target, onTarget)}
        ${checks}
        ${covers}
        <span class="sub">Report ${r.n} · <code>${short(r.commit)}</code>${r.base ? ` from <code>${short(r.base)}</code>` : ""} · ${esc(clock(r.at))}</span>${earlier}</section>
      <section><div class="hd"><h2>How it was built</h2><span class="lab">Sound design?</span></div>
        <div class="prose">${markdown(r.design, pid)}</div>
        ${shared}
        ${it ? `<div><span class="lab">Decisions made in this session · ${decisions.length || "none"}</span>${decs}</div>` : ""}
        ${files ? `<div>${filesLabel}<div class="tbl-wrap"><table class="t files"><tbody>${files}</tbody></table></div></div>` : ""}
      </section>
    </div></section>`;
}
