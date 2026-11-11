// The JSON API (mull guide json, "mull serve") and its event stream.

export class APIError extends Error {
  constructor(status, code, message) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function call(path, init) {
  let res;
  try {
    res = await fetch(path, init);
  } catch (err) {
    throw new APIError(0, "unreachable", "mull serve isn't answering");
  }
  let body = null;
  try { body = await res.json(); } catch { /* not JSON */ }
  if (!res.ok) throw new APIError(res.status, body?.code || "", body?.error || res.statusText);
  return body;
}

const p = id => `/api/projects/${encodeURIComponent(id)}`;

export const api = {
  projects: () => call("/api/projects"),
  // build names the server's build ({build, version}).
  build: () => call("/api/build"),
  // unmulled: the repositories beside the registered ones that aren't on
  // mull (old mull data, or worked on recently).
  unmulled: () => call("/api/unmulled"),
  // page is one of a project's pages: sub is the path under the project
  // ("plans/<slug>", "items/<id>", "backlog?plan=<slug>"…), or "" for its
  // overview.
  page: (id, sub) => call(sub ? `${p(id)}/${sub}` : p(id)),
  tree: id => call(`${p(id)}/tree`),
  // search finds records by ID or title in every project.
  search: q => call(`/api/search?q=${encodeURIComponent(q)}`),
  revision: id => call(`${p(id)}/revision`),
  // seen marks the revision the page showed as seen; changes after it stay
  // new.
  seen: (id, commit) => call(`${p(id)}/seen`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ commit }),
  }),
  // ops writes mull data as the human: batch lines, chosen from the
  // revision at commit. A conflict (409) means what they touch changed since.
  ops: (id, commit, ops) => call(`${p(id)}/ops`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ commit, ops }),
  }),
};

// listen follows GET /api/events: project(id) for each project whose mull
// branch moved, and live(up) as the stream drops and comes back. The
// browser reconnects by itself; events sent while it was away aren't
// replayed, so reopened() asks for everything again.
export function listen({ project, live, reopened }) {
  const events = new EventSource("/api/events");
  let dropped = false;
  events.addEventListener("project", ev => {
    try { project(JSON.parse(ev.data).id); } catch { /* malformed event: ignore */ }
  });
  events.onopen = () => {
    live(true);
    if (dropped) { dropped = false; reopened(); }
  };
  events.onerror = () => { dropped = true; live(false); };
  return events;
}
