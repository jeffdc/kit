// The model: a help sheet on how mull works, for the human. It's static
// (Model v2, as docs/serve/README.md and mull guide describe it), so keep it
// in step when the model changes. No status here: that's every other sheet.

import { esc, tag, href } from "./html.js";
import { titleBlock } from "./title.js";

const arrow = `<span class="faint" aria-hidden="true">→</span>`;

// The Needs you tiers, in rank order, as GET …/needs_you names them.
const TIERS = [
  ["Blocking work", "Committed work that can't finish without your answer", "A question on an active item, or on committed work that's ready to start."],
  ["Waiting for your review", "Not done until you accept it", "An item an agent finished with mull done: what was built, and how."],
  ["Questions for the current phases", "Will block soon", "A question on an item in a phase that can start now."],
  ["Questions for later phases", "Can wait for their phase", "A question on an item in a phase still waiting for its needs."],
  ["Questions about the whole plan", "Shape the plan, not one phase", "A question with no item, or on an item in no phase."],
  ["Decisions to approve", "Choices agents made; approve, or decide differently", "Grouped by where they were made. Accepting a review ratifies the decisions it carries."],
];

// What the human runs. Agents add --on-request when they run these for you.
const COMMANDS = [
  ["mull accept <id…>", "Accept reviewed work (an item or its review), or approve decisions."],
  ["mull decide <id> \"…\"", "Settle a decision differently, or answer a question; it joins the decision log."],
  ["mull do <id> --reason \"…\"", "Send reviewed work back; or turn a question or idea into an item."],
  ["mull commit <id>", "Make a captured bug or idea into committed work (--plan, --phase, or --plan none)."],
  ["mull plan agree <slug>", "Agree a shaping plan, so it can have phases and work."],
  ["mull triage", "Stamp the backlog as gone through, with an agent (mull guide triage)."],
  ["mull park <id> / mull drop <id>", "Set an entry aside, or drop an entry or item for good (both recorded)."],
];

export function pageModel() {
  const fields = [
    ["Records", "plan · phase · item"],
    ["Item stages", "captured to done"],
    ["Needs you", "questions · reviews · decisions"],
    ["Backlog", "bugs · ideas"],
    ["Kept on", `the <code>mull</code> branch`],
    ["Written by", "agents, through the CLI"],
  ];
  const head = titleBlock({ title: "The model", crumbs: [`<a href="${href.home()}">All projects</a>`], fields });
  const stages = ["captured", "committed", "active", "in-review", "done"].map(s => tag(s)).join(arrow);
  return head + `<div class="page model">
  <section><div class="hd"><h2>How it fits together</h2><span class="lab">One project is one git repository</span></div>
<pre>Project            <i>a repo, with a mull branch</i>
├─ <b>Needs you</b>       questions · reviews · decisions <i>(attention only, ranked)</i>
├─ <b>Plan</b>            shaping → active → done      <i>agreed by you, explicitly</i>
│  │  doc             the picture, done-when, why the order; intent only
│  └─ <b>Phase</b>        doc: its design and done-when
│     │  needs        other phases only  <i>(go live needs X, Y, Z)</i>
│     └─ <b>Item</b>      one agent session; no needs, no order
├─ <b>Unplanned</b>       items committed without a plan
└─ <b>Backlog</b>         captured items: bug or idea, a free-text category,
                   optionally tagged to a plan or phase</pre></section>

  <section><div class="hd"><h2>One item, stages instead of steps</h2><span class="lab">Dropped from anywhere</span></div>
    <div class="stages">${stages}</div>
    <p class="sub measure">Anyone captures; only you commit. An agent can start an item straight from captured or from committed, so a small bug can go from the backlog to review in one session. Every item is reviewed: done means you accepted it.</p></section>

  <section><div class="hd"><h2>Needs you</h2><span class="lab">Ranked by what an answer unblocks</span></div>
    <div class="tbl-wrap"><table class="t"><thead><tr><th style="width:40px">Rank</th><th>Tier</th><th class="w">What lands here</th></tr></thead><tbody>
    ${TIERS.map(([title, hint, what], i) => `<tr><td class="mono">${String(i + 1).padStart(2, "0")}</td><td><span class="ttl">${esc(title)}</span><span class="meta">${esc(hint)}</span></td><td class="mut w">${esc(what)}</td></tr>`).join("")}
    </tbody></table></div>
    <p class="sub measure">Bugs and ideas never land here: they wait in the backlog for triage. Parked entries are counted, not listed.</p></section>

  <section><div class="hd"><h2>Rules</h2></div><dl class="rules">
    <dt>No status in prose</dt><dd>Docs hold intent and reasons. Status comes from the records and shows beside the doc, in its margin.</dd>
    <dt>Order only where it matters</dt><dd>Needs exist between phases only. What can start now is derived: a phase whose needs are done.</dd>
    <dt>Review is always required</dt><dd>What was built (summary, tour, checks) and how (design notes, the session's decisions, files by layer), settled together.</dd>
    <dt>Needs you is attention only</dt><dd>Questions, reviews and decisions. Nothing else nags.</dd>
    <dt>Triage is a process</dt><dd>Run with an agent (<code>mull guide triage</code>). mull stamps when it happened; the UI shows only how much is untriaged.</dd>
    <dt>Anyone captures, you commit</dt><dd>Agents file bugs and ideas with where they came from. You decide what becomes work, and what becomes a plan.</dd>
    <dt>Revisions</dt><dd>Whatever changed since you last looked is clouded in red. Mark revision seen clears the clouds; the last look is kept on this machine, never on the mull branch.</dd>
  </dl></section>

  <section><div class="hd"><h2>What you run</h2><span class="lab">This page is read-first: its buttons copy these</span></div><dl class="rules cmds">
    ${COMMANDS.map(([cmd, what]) => `<dt><code>${esc(cmd)}</code></dt><dd>${esc(what)}</dd>`).join("")}
  </dl><p class="sub measure">The rest is in <code>mull guide</code>: <code>agents</code>, <code>supervisor</code>, <code>intake</code>, <code>triage</code> and <code>json</code>.</p></section>
</div>`;
}
