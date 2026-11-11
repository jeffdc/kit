// The placeholder page: the project list from GET /api/projects, refetched
// whenever GET /api/events says a project changed.

const esc = s => String(s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

function row(p) {
  const name = `<b>${esc(p.name)}</b><span class="meta">${esc(p.path)}</span>`;
  if (p.error) {
    return `<tr class="bad"><td>${name}</td><td colspan="7"><code>${esc(p.error.code)}</code> ${esc(p.error.error)}</td></tr>`;
  }
  const s = p.summary;
  const branch = `<code>${esc(s.integration)}</code>${s.ahead_of_release ? ` · ${s.ahead_of_release} ahead of <code>${esc(s.release)}</code>` : ""}`;
  const plans = s.plans.map(pl => `<div>${esc(pl.title)}${pl.state === "active" ? "" : ` <span class="meta">${esc(pl.state)}</span>`}</div>`).join("");
  const n = v => `<td class="n">${v}</td>`;
  return `<tr><td>${name}<span class="meta">${branch}</span></td><td>${plans || '<span class="meta">none</span>'}</td>` +
    n(s.needs_you.reviews) + n(s.needs_you.questions) + n(s.needs_you.decisions) + n(s.agents) + n(s.untriaged) +
    `<td class="n mut">${esc(new Date(s.last_change).toLocaleString())}</td></tr>`;
}

async function load() {
  const status = document.getElementById("status");
  const table = document.getElementById("projects");
  try {
    const res = await fetch("/api/projects");
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || res.statusText);
    if (!body.projects.length) {
      status.textContent = "No projects yet. Run any mull command in a repository with a mull branch to add it.";
      table.hidden = true;
      status.hidden = false;
      return;
    }
    table.tBodies[0].innerHTML = body.projects.map(row).join("");
    table.hidden = false;
    status.hidden = true;
  } catch (err) {
    status.textContent = `Couldn't load projects: ${err.message}`;
    status.hidden = false;
  }
}

// refresh loads the list, once more after the current load if asked again
// meanwhile, so a burst of events is at most two fetches.
let loading = null, again = false;
function refresh() {
  if (loading) { again = true; return; }
  loading = load().finally(() => {
    loading = null;
    if (again) { again = false; refresh(); }
  });
}

// The browser reconnects a dropped stream itself; whatever changed while it
// was down is caught by refetching when it reopens.
const events = new EventSource("/api/events");
let dropped = false;
events.addEventListener("project", refresh);
events.onerror = () => { dropped = true; };
events.onopen = () => { if (dropped) { dropped = false; refresh(); } };

refresh();
