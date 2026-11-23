// The model: a help sheet on how mull works, for the human. It's static
// (the redesign, as docs/redesign.md and mull guide describe it), so keep
// it in step when the model changes. No status here: that's every other
// sheet.

import { esc, tag, href } from "./html.js";
import { titleBlock } from "./title.js";

const arrow = `<span class="faint" aria-hidden="true">→</span>`;

// What the surface shows, top to bottom.
const SURFACE = [
  ["Needs you", "High-risk work waiting, questions, landed work to look at", "Each settled on its row: accept or send back, take the default or decide differently, looks fine or send back."],
  ["Trending worse", "Rules, clusters, problems", "Checked rules failing or with more violations than at your last look, rules with no check, three or more queued items in one area, and what's wrong with work."],
  ["Plans", "Each plan's phases as a hill", "Uphill is still being worked out, downhill is executing; waiting and done are shown quietly."],
  ["Flow", "Folded", "What landed, with its one-line summary and risk; what slowed it; what came back; the median from start to done."],
];

// How done scores risk, from the diff alone.
const RISK = [
  ["High", "Waits for you", "It touches a high-risk path (the rulebook, checks, CI, migrations, auth…) or a rule. It's in Needs you until you accept it or send it back."],
  ["Medium", "Lands, and is shown to you", "Checks pass, but the diff is large. It's in Needs you as landed work to look at."],
  ["Low", "Lands, sampled", "Everything else that passes. A fixed share is drawn for you to look at."],
];

// What the human runs. Agents add --on-request when they run these for you.
const COMMANDS = [
  ["mull status", "The surface in a terminal (--text): what changed since you last looked here."],
  ["mull accept <id…>", "Accept high-risk work so it lands, or say landed work looked fine (Accept and Looks fine on the surface do this for you)."],
  ["mull do <id> --reason \"…\"", "Send work back: waiting work reopens for another session, landed work gets a follow-up item (Send back does this for you)."],
  ["mull decide <id> \"…\"", "Answer a question; it joins the decision log (Take the default and Decide differently do this for you)."],
  ["mull rules", "The rulebook, RULES.md in the repository: each rule, its scope and check. mull rules check runs them."],
  ["mull plan agree <slug>", "Agree a shaping plan, so it can have phases and work (Agree this plan, on its sheet, does this for you)."],
  ["mull config risk", "Change the high-risk paths, the size that makes work medium, and the share of low-risk work sampled."],
];

export function pageModel() {
  const fields = [
    ["Records", "plan · phase · item"],
    ["Item stages", "captured to done"],
    ["Needs you", "reviews · questions · looks"],
    ["Queue", "the agents' own"],
    ["Kept on", `the <code>mull</code> branch`],
    ["Rules", `<code>RULES.md</code> in the repo`],
  ];
  const head = titleBlock({ title: "The model", crumbs: [`<a href="${href.home()}">All projects</a>`], fields });
  const stages = ["captured", "committed", "active", "in-review", "done"].map(s => tag(s)).join(arrow);
  const table = (rows, a, b) => `<div class="tbl-wrap"><table class="t"><thead><tr><th>${esc(a)}</th><th class="w">${esc(b)}</th></tr></thead><tbody>
    ${rows.map(([title, hint, what]) => `<tr><td><span class="ttl">${esc(title)}</span><span class="meta">${esc(hint)}</span></td><td class="mut w">${esc(what)}</td></tr>`).join("")}
    </tbody></table></div>`;
  return head + `<div class="page model">
  <section><div class="hd"><h2>How it fits together</h2><span class="lab">One project is one git repository</span></div>
<pre>Project            <i>a repo, with a mull branch</i>
├─ <b>Rules</b>           RULES.md: each rule with its check, or advisory
├─ <b>Plan</b>            shaping → active → done      <i>agreed by you, explicitly</i>
│  │  doc             the goal, the appetite, the phases' done-criteria
│  └─ <b>Phase</b>        cut into items when it starts
│     └─ <b>Item</b>      one agent session
├─ <b>Unplanned</b>       items started with no plan
└─ <b>Queue</b>           the agents' bugs and ideas, each with its area</pre></section>

  <section><div class="hd"><h2>The surface</h2><span class="lab">A project's first sheet: what changed since you last looked</span></div>
    ${table(SURFACE, "Part", "What's in it")}
    <p class="sub measure">Mark seen, by the tabs, moves your last look to now. The last look is kept on this machine, never on the mull branch.</p></section>

  <section><div class="hd"><h2>Risk, from the diff</h2><span class="lab">Never from the agent's account</span></div>
    ${table(RISK, "Band", "What happens")}
    <div class="stages">${stages}</div>
    <p class="sub measure">Agents decide locally and note it in their report; only a rule conflict or something hard to undo becomes a question, with the agent's default and when it stands.</p></section>

  <section><div class="hd"><h2>What you run</h2><span class="lab">The sheets do most of these as you</span></div><dl class="rules cmds">
    ${COMMANDS.map(([cmd, what]) => `<dt><code>${esc(cmd)}</code></dt><dd>${esc(what)}</dd>`).join("")}
  </dl><p class="sub measure">The rest is in <code>mull guide</code>: <code>agents</code>, <code>supervisor</code>, <code>intake</code> and <code>json</code>.</p></section>
</div>`;
}
