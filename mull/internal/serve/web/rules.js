// Rules: a plan's or phase's standing rules for the agents working in it,
// the human's, in markdown. mull context --item hands a worker its plan's
// rules, then its phase's, in full. They show under the doc and are edited
// in place, as the doc is.

import { esc } from "./html.js";
import { renderDoc } from "./doc.js";
import { editWrite } from "./write.js";

// rulesSection: the rules of a plan or phase (kind) with their edit write.
// op is the write without its rules field; cloud is the changes class.
export function rulesSection({ pid, kind, key, op, rules, title, refs, cloud = "" }) {
  const edit = editWrite({ pid, key, ops: [op], field: "rules", long: true,
    value: rules, label: rules ? "Edit the rules" : "Write the rules", save: "Save the rules", done: `Saved the ${kind} rules of ${title}.`,
    placeholder: `Markdown. What every agent working in this ${kind} follows: how to set up, what to use, what not to touch.` });
  const body = rules
    ? renderDoc(rules, pid, refs)
    : `<div class="prose"><p class="ph-empty">No rules for this ${kind}.${kind === "phase" ? " Its workers follow the plan's." : ""}</p></div>`;
  const lab = kind === "plan" ? "Yours, for every agent working in this plan · each gets them in full with its item"
    : "Yours, for this phase's workers · after the plan's, in full with each item";
  return `<section><div class="hd ed"><h2><span class="${cloud.trim()}">The rules</span></h2><span class="lab">${esc(lab)}</span>${edit.button}</div><div class="doc-wrap">${edit.form || body}</div></section>`;
}
