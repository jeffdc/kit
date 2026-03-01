package export

import (
	"encoding/json"
	"html/template"
	"io"

	"forage/internal/model"
)

type templateData struct {
	Books       template.JS
	Booksellers template.JS
}

func Generate(books []model.Book, booksellers []model.Bookseller, w io.Writer) error {
	if books == nil {
		books = []model.Book{}
	}
	if booksellers == nil {
		booksellers = []model.Bookseller{}
	}

	booksJSON, err := json.Marshal(books)
	if err != nil {
		return err
	}
	sellersJSON, err := json.Marshal(booksellers)
	if err != nil {
		return err
	}

	tmpl, err := template.New("library").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	return tmpl.Execute(w, templateData{
		Books:       template.JS(booksJSON),
		Booksellers: template.JS(sellersJSON),
	})
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Forage Library</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f8f7f4; color: #333; padding: 16px; max-width: 800px; margin: 0 auto; }
h1 { font-size: 1.4em; margin-bottom: 12px; color: #222; }
.controls { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 16px; }
.controls input { flex: 1; min-width: 200px; padding: 8px 12px; border: 1px solid #ccc; border-radius: 6px; font-size: 14px; }
.controls select { padding: 8px 12px; border: 1px solid #ccc; border-radius: 6px; font-size: 14px; background: #fff; }
.count { font-size: 13px; color: #888; margin-bottom: 12px; }
.book { background: #fff; border: 1px solid #e0ddd8; border-radius: 8px; padding: 14px; margin-bottom: 10px; }
.book-title { font-weight: 600; font-size: 1.05em; }
.book-author { color: #666; font-size: 0.95em; }
.book-meta { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; }
.tag { background: #eee8d5; color: #555; padding: 2px 8px; border-radius: 4px; font-size: 12px; }
.status { font-size: 12px; padding: 2px 8px; border-radius: 4px; font-weight: 500; }
.status-wishlist { background: #dbeafe; color: #1e40af; }
.status-reading { background: #fef3c7; color: #92400e; }
.status-paused { background: #f3e8ff; color: #6b21a8; }
.status-read { background: #d1fae5; color: #065f46; }
.rating { color: #d97706; font-size: 13px; }
.book-notes { margin-top: 8px; font-size: 0.9em; color: #555; white-space: pre-wrap; }
.empty { text-align: center; color: #999; padding: 40px 0; }
.sort-controls { display: flex; gap: 4px; align-items: center; font-size: 13px; color: #666; }
.sort-controls button { background: none; border: 1px solid #ccc; border-radius: 4px; padding: 4px 8px; font-size: 12px; cursor: pointer; }
.sort-controls button.active { background: #333; color: #fff; border-color: #333; }
.book-sellers { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; }
.book-sellers a { font-size: 11px; padding: 2px 7px; border: 1px solid #ccc; border-radius: 4px; color: #555; text-decoration: none; white-space: nowrap; }
.book-sellers a:hover { background: #eee8d5; color: #333; }
</style>
</head>
<body>
<h1>Forage Library</h1>
<div class="controls">
  <input type="text" id="search" placeholder="Search books..." autocomplete="off">
  <select id="filter">
    <option value="all">All</option>
    <option value="wishlist" selected>Wishlist</option>
    <option value="reading">Reading</option>
    <option value="paused">Paused</option>
    <option value="read">Read</option>
  </select>
</div>
<div class="sort-controls">
  Sort:
  <button data-sort="title" class="active">Title</button>
  <button data-sort="author">Author</button>
  <button data-sort="rating">Rating</button>
  <button data-sort="date_added">Date</button>
</div>
<div class="count" id="count"></div>
<div id="books"></div>
<script>
const books = {{.Books}};
const booksellers = {{.Booksellers}};
let currentSort = "title";
let sortDir = 1;

function render() {
  const q = document.getElementById("search").value.toLowerCase();
  const status = document.getElementById("filter").value;

  let filtered = books.filter(b => {
    if (status !== "all" && b.status !== status) return false;
    if (q) {
      const hay = [b.title, b.author, (b.tags||[]).join(" "), b.body||""].join(" ").toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });

  filtered.sort((a, b) => {
    let va = a[currentSort] || "", vb = b[currentSort] || "";
    if (currentSort === "rating") { va = va || 0; vb = vb || 0; return (vb - va) * sortDir; }
    if (typeof va === "string") va = va.toLowerCase();
    if (typeof vb === "string") vb = vb.toLowerCase();
    return (va < vb ? -1 : va > vb ? 1 : 0) * sortDir;
  });

  document.getElementById("count").textContent = filtered.length + " book" + (filtered.length !== 1 ? "s" : "");

  if (filtered.length === 0) {
    document.getElementById("books").innerHTML = '<div class="empty">No books found.</div>';
    return;
  }

  document.getElementById("books").innerHTML = filtered.map(b => {
    const stars = b.rating ? '★'.repeat(b.rating) + '☆'.repeat(5 - b.rating) : '';
    const tags = (b.tags || []).map(t => '<span class="tag">' + esc(t) + '</span>').join("");
    const notes = b.body ? '<div class="book-notes">' + esc(b.body) + '</div>' : '';
    const query = encodeURIComponent(b.title + " " + b.author);
    const sellerLinks = booksellers.length ? '<div class="book-sellers">' + booksellers.map(s =>
      '<a href="' + s.url.replace('{query}', query) + '" target="_blank" rel="noopener">' + esc(s.name) + '</a>'
    ).join("") + '</div>' : '';
    return '<div class="book">' +
      '<div class="book-title">' + esc(b.title) + '</div>' +
      '<div class="book-author">' + esc(b.author) + '</div>' +
      '<div class="book-meta">' +
        '<span class="status status-' + b.status + '">' + b.status + '</span>' +
        (stars ? '<span class="rating">' + stars + '</span>' : '') +
        tags +
      '</div>' +
      notes +
      sellerLinks +
    '</div>';
  }).join("");
}

function esc(s) {
  const d = document.createElement("div");
  d.textContent = s || "";
  return d.innerHTML;
}

document.getElementById("search").addEventListener("input", render);
document.getElementById("filter").addEventListener("change", render);
document.querySelectorAll(".sort-controls button").forEach(btn => {
  btn.addEventListener("click", () => {
    const s = btn.dataset.sort;
    if (currentSort === s) { sortDir *= -1; } else { currentSort = s; sortDir = 1; }
    document.querySelectorAll(".sort-controls button").forEach(b => b.classList.remove("active"));
    btn.classList.add("active");
    render();
  });
});
render();
</script>
</body>
</html>`
