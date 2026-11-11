// Docs: plan, phase and item docs are markdown, rendered with marked. Raw
// HTML in a doc is shown as text and only web links stay links, since docs
// are written by agents. A phase or item ID the doc names becomes a link
// carrying its status, and each section gets a margin with the status of
// what it mentions: docs hold intent, status shows beside them.

import { Marked } from "./vendor/marked.esm.js";
import { esc, href, tag, cells, stageLabel, plural } from "./html.js";

const WEB = /^(https?:|mailto:|#)/i;
const ID = /(^|[^0-9A-Za-z_])([0-9a-f]{4})(?![0-9A-Za-z_])/g;

function parser(pid, byId) {
  const link = r => r.type === "phase" ? href.phase(pid, r.id) : href.item(pid, r.id);
  const m = new Marked({ gfm: true });
  m.use({
    renderer: {
      html: ({ text, block }) => block ? `<p>${esc(text)}</p>` : esc(text),
      link({ href: to, title, tokens }) {
        const text = this.parser.parseInline(tokens);
        if (!WEB.test(to)) return `<span class="xref" title="${esc(to)}">${text}</span>`;
        return `<a href="${esc(to)}"${title ? ` title="${esc(title)}"` : ""}>${text}</a>`;
      },
      image: ({ text }) => text ? `<span class="xref">[${esc(text)}]</span>` : "",
    },
    extensions: [{
      name: "ref",
      level: "inline",
      start(src) {
        for (const x of src.matchAll(ID)) if (byId.has(x[2])) return x.index + x[1].length;
        return undefined;
      },
      tokenizer(src) {
        const x = /^([0-9a-f]{4})(?![0-9A-Za-z_])/.exec(src);
        if (x && byId.has(x[1])) return { type: "ref", raw: x[1], id: x[1] };
        return undefined;
      },
      renderer({ id }) {
        const r = byId.get(id);
        return `<a class="ref" href="${esc(link(r))}" title="${esc(r.title)}"><code>${esc(id)}</code><sup>${esc(stageLabel(r.state))}</sup></a>`;
      },
    }],
  });
  return m;
}

// markdown renders a short text (a summary, notes, design notes) without
// the margin.
export function markdown(src, pid, refs = []) {
  return parser(pid, new Map(refs.map(r => [r.id, r]))).parse(src || "");
}

// renderDoc: the doc in sections (at each heading down to ###), each beside
// a margin listing the phases and items it names.
export function renderDoc(src, pid, refs = []) {
  const byId = new Map(refs.map(r => [r.id, r]));
  const m = parser(pid, byId);
  const tokens = m.lexer(src || "");
  const groups = [];
  for (const t of tokens) {
    if ((t.type === "heading" && t.depth <= 3) || !groups.length) groups.push([]);
    groups[groups.length - 1].push(t);
  }
  return `<div class="doc">${groups.map(g => {
    g.links = tokens.links;
    const named = [...new Set([...g.map(t => t.raw).join("").matchAll(ID)].map(x => x[2]).filter(id => byId.has(id)))];
    const margin = named.map(id => marginRow(pid, byId.get(id))).join("");
    return `<div class="blk"><div class="prose">${m.parser(g)}</div><div class="margin"${named.length ? ' aria-label="Status of what this section names"' : ""}>${margin}</div></div>`;
  }).join("")}</div>`;
}

function marginRow(pid, r) {
  const link = r.type === "phase" ? href.phase(pid, r.id) : href.item(pid, r.id);
  const p = r.progress, total = p ? p.total - p.dropped : 0;
  const state = tag(r.state);
  return `<div class="co"><a href="${esc(link)}">${esc(r.title)}</a><code class="faint">${esc(r.id)}</code>` +
    `<span class="row3">${state}${p ? cells(p) : ""}${total ? `<span class="mono">${p.done}/${total}</span>` : ""}${r.questions ? `<span>${plural(r.questions, "question")}</span>` : ""}</span></div>`;
}
