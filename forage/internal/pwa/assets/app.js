// Forage PWA - IndexedDB data layer and UI rendering
(function () {
  "use strict";

  var DB_NAME = "forage";
  var DB_VERSION = 1;
  var _db = null;
  var _booksellers = [];

  function openDB() {
    return new Promise(function (resolve, reject) {
      var req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = function (e) {
        var d = e.target.result;
        if (!d.objectStoreNames.contains("books")) {
          d.createObjectStore("books", { keyPath: "id" });
        }
        if (!d.objectStoreNames.contains("changes")) {
          d.createObjectStore("changes", { autoIncrement: true });
        }
        if (!d.objectStoreNames.contains("meta")) {
          d.createObjectStore("meta", { keyPath: "key" });
        }
      };
      req.onsuccess = function (e) {
        _db = e.target.result;
        resolve(_db);
      };
      req.onerror = function () {
        reject(req.error);
      };
    });
  }

  function tx(storeNames, mode) {
    var t = _db.transaction(storeNames, mode);
    return t;
  }

  function reqToPromise(req) {
    return new Promise(function (resolve, reject) {
      req.onsuccess = function () { resolve(req.result); };
      req.onerror = function () { reject(req.error); };
    });
  }

  function txComplete(t) {
    return new Promise(function (resolve, reject) {
      t.oncomplete = function () { resolve(); };
      t.onerror = function () { reject(t.error); };
    });
  }

  function genId() {
    var bytes = new Uint8Array(2);
    crypto.getRandomValues(bytes);
    var hex = "";
    for (var i = 0; i < bytes.length; i++) {
      hex += ("0" + bytes[i].toString(16)).slice(-2);
    }
    return hex;
  }

  var db = {
    init: function () {
      return openDB().then(function () {
        // Store booksellers for rendering
        if (window.__FORAGE_DATA__ && window.__FORAGE_DATA__.booksellers) {
          _booksellers = window.__FORAGE_DATA__.booksellers;
        }

        var t = tx(["meta"], "readonly");
        var store = t.objectStore("meta");
        return reqToPromise(store.get("dataVersion")).then(function (entry) {
          var seedVersion = window.__FORAGE_DATA_VERSION__ || "";
          var storedVersion = entry ? entry.value : "";

          if (!storedVersion || (seedVersion && seedVersion > storedVersion)) {
            return db._seed(seedVersion);
          }
        });
      });
    },

    _seed: function (version) {
      var books = (window.__FORAGE_DATA__ && window.__FORAGE_DATA__.books) || [];
      var t = tx(["books", "meta", "changes"], "readwrite");
      var bookStore = t.objectStore("books");
      var metaStore = t.objectStore("meta");
      var changeStore = t.objectStore("changes");

      bookStore.clear();
      changeStore.clear();

      for (var i = 0; i < books.length; i++) {
        bookStore.put(books[i]);
      }
      metaStore.put({ key: "dataVersion", value: version });

      return txComplete(t);
    },

    listBooks: function () {
      var t = tx(["books"], "readonly");
      return reqToPromise(t.objectStore("books").getAll());
    },

    getBook: function (id) {
      var t = tx(["books"], "readonly");
      return reqToPromise(t.objectStore("books").get(id));
    },

    createBook: function (book) {
      if (!book.id) book.id = genId();
      var t = tx(["books", "changes"], "readwrite");
      t.objectStore("books").put(book);
      t.objectStore("changes").add({ op: "create", book: book, ts: new Date().toISOString() });
      return txComplete(t).then(function () { return book; });
    },

    updateBook: function (id, fields) {
      var t1 = tx(["books"], "readonly");
      return reqToPromise(t1.objectStore("books").get(id)).then(function (existing) {
        if (!existing) throw new Error("book not found: " + id);
        var updated = {};
        for (var k in existing) updated[k] = existing[k];
        for (var f in fields) updated[f] = fields[f];

        var t2 = tx(["books", "changes"], "readwrite");
        t2.objectStore("books").put(updated);
        t2.objectStore("changes").add({ op: "update", id: id, fields: fields, ts: new Date().toISOString() });
        return txComplete(t2).then(function () { return updated; });
      });
    },

    deleteBook: function (id) {
      var t = tx(["books", "changes"], "readwrite");
      t.objectStore("books").delete(id);
      t.objectStore("changes").add({ op: "delete", id: id, ts: new Date().toISOString() });
      return txComplete(t);
    },

    getChanges: function () {
      var t = tx(["changes"], "readonly");
      return reqToPromise(t.objectStore("changes").getAll());
    },

    clearChanges: function () {
      var t = tx(["changes"], "readwrite");
      t.objectStore("changes").clear();
      return txComplete(t);
    },

    getChangeCount: function () {
      var t = tx(["changes"], "readonly");
      return reqToPromise(t.objectStore("changes").count());
    }
  };

  // Expose db globally
  window.forageDB = db;

  // --- UI Rendering ---

  var currentSort = "title";
  var sortDir = 1;
  var debounceTimer = null;

  function esc(s) {
    var d = document.createElement("div");
    d.textContent = s || "";
    return d.innerHTML;
  }

  function populateTagFilter(books) {
    var tagSet = {};
    books.forEach(function (b) {
      (b.tags || []).forEach(function (t) { if (t) tagSet[t] = true; });
    });
    var tags = Object.keys(tagSet).sort();
    var sel = document.getElementById("tag-filter");
    var current = sel.value;
    sel.innerHTML = '<option value="all">All tags</option>';
    tags.forEach(function (t) {
      var opt = document.createElement("option");
      opt.value = t;
      opt.textContent = t;
      sel.appendChild(opt);
    });
    sel.value = current;
  }

  function render() {
    db.listBooks().then(function (books) {
      populateTagFilter(books);

      var q = document.getElementById("search").value.toLowerCase();
      var status = document.getElementById("filter").value;
      var tag = document.getElementById("tag-filter").value;

      var filtered = books.filter(function (b) {
        if (status !== "all" && b.status !== status) return false;
        if (tag !== "all" && (!b.tags || b.tags.indexOf(tag) === -1)) return false;
        if (q) {
          var hay = [b.title, b.author, (b.tags || []).join(" "), b.body || ""].join(" ").toLowerCase();
          if (hay.indexOf(q) === -1) return false;
        }
        return true;
      });

      filtered.sort(function (a, b) {
        var va = a[currentSort] || "";
        var vb = b[currentSort] || "";
        if (currentSort === "rating") {
          va = va || 0;
          vb = vb || 0;
          return (vb - va) * sortDir;
        }
        if (typeof va === "string") va = va.toLowerCase();
        if (typeof vb === "string") vb = vb.toLowerCase();
        return (va < vb ? -1 : va > vb ? 1 : 0) * sortDir;
      });

      document.getElementById("count").textContent = filtered.length + " book" + (filtered.length !== 1 ? "s" : "");

      if (filtered.length === 0) {
        document.getElementById("books").innerHTML = '<div class="empty">No books found.</div>';
        return;
      }

      updateSyncBadge();

      document.getElementById("books").innerHTML = filtered.map(function (b) {
        var stars = b.rating ? "\u2605".repeat(b.rating) + "\u2606".repeat(5 - b.rating) : "";
        var tags = (b.tags || []).map(function (t) {
          return '<span class="tag">' + esc(t) + "</span>";
        }).join("");
        var notes = b.body ? '<div class="book-notes">' + esc(b.body) + "</div>" : "";
        var query = encodeURIComponent(b.title + " " + b.author);
        var sellerLinks = _booksellers.length ? '<div class="book-sellers">' + _booksellers.map(function (s) {
          return '<a href="' + s.url.replace("{query}", query) + '" target="_blank" rel="noopener">' + esc(s.name) + "</a>";
        }).join("") + "</div>" : "";

        return '<div class="book" data-id="' + esc(b.id) + '">' +
          '<div class="book-title">' + esc(b.title) + "</div>" +
          '<div class="book-author">' + esc(b.author) + "</div>" +
          '<div class="book-meta">' +
            '<span class="status status-' + b.status + '">' + b.status + "</span>" +
            (stars ? '<span class="rating">' + stars + "</span>" : "") +
            tags +
          "</div>" +
          notes +
          sellerLinks +
        "</div>";
      }).join("");
    });
  }

  function debouncedRender() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(render, 300);
  }

  // --- Modal System ---

  var _editingId = null;

  function openModal(mode, book) {
    var overlay = document.getElementById("modal-overlay");
    var title = document.getElementById("modal-title");
    var form = document.getElementById("book-form");
    var deleteBtn = form.querySelector(".btn-delete");
    var lookupContainer = document.getElementById("lookup-container");

    form.reset();
    lookupContainer.innerHTML = "";

    if (mode === "edit" && book) {
      _editingId = book.id;
      title.textContent = "Edit Book";
      document.getElementById("field-title").value = book.title || "";
      document.getElementById("field-author").value = book.author || "";
      document.getElementById("field-status").value = book.status || "wishlist";
      document.getElementById("field-rating").value = String(book.rating || 0);
      document.getElementById("field-tags").value = (book.tags || []).join(", ");
      document.getElementById("field-notes").value = book.body || "";
      deleteBtn.classList.remove("hidden");
    } else {
      _editingId = null;
      title.textContent = "Add Book";
      deleteBtn.classList.add("hidden");
      injectLookup();
    }

    overlay.classList.remove("hidden");
  }

  function closeModal() {
    document.getElementById("modal-overlay").classList.add("hidden");
    _editingId = null;
  }

  function handleSave(e) {
    e.preventDefault();
    var titleVal = document.getElementById("field-title").value.trim();
    var authorVal = document.getElementById("field-author").value.trim();
    if (!titleVal || !authorVal) return;

    var rawTags = document.getElementById("field-tags").value;
    var tags = rawTags ? rawTags.split(",").map(function (t) { return t.trim(); }).filter(Boolean) : [];
    var rating = parseInt(document.getElementById("field-rating").value, 10) || 0;
    var status = document.getElementById("field-status").value;
    var notes = document.getElementById("field-notes").value.trim();

    if (_editingId) {
      db.updateBook(_editingId, {
        title: titleVal,
        author: authorVal,
        status: status,
        rating: rating,
        tags: tags,
        body: notes
      }).then(function () {
        closeModal();
        render();
      });
    } else {
      db.createBook({
        title: titleVal,
        author: authorVal,
        status: status,
        rating: rating,
        tags: tags,
        body: notes,
        date_added: new Date().toISOString().slice(0, 10)
      }).then(function () {
        closeModal();
        render();
      });
    }
  }

  function handleDelete() {
    if (!_editingId) return;
    if (!confirm("Delete this book?")) return;
    db.deleteBook(_editingId).then(function () {
      closeModal();
      render();
    });
  }

  // --- Open Library Lookup ---

  function injectLookup() {
    var container = document.getElementById("lookup-container");
    container.innerHTML = '<button type="button" class="lookup-btn">Look up on Open Library</button>';
    var btn = container.querySelector(".lookup-btn");

    btn.addEventListener("click", function () {
      var titleVal = document.getElementById("field-title").value.trim();
      var authorVal = document.getElementById("field-author").value.trim();
      if (!titleVal && !authorVal) return;

      // Remove old results
      var old = container.querySelector(".lookup-results, .lookup-msg");
      if (old) old.remove();

      var url = "https://openlibrary.org/search.json?title=" +
        encodeURIComponent(titleVal) + "&author=" + encodeURIComponent(authorVal) +
        "&limit=5&fields=title,author_name,first_publish_year";

      fetch(url).then(function (res) {
        return res.json();
      }).then(function (data) {
        var docs = data.docs || [];
        if (docs.length === 0) {
          var msg = document.createElement("div");
          msg.className = "lookup-msg";
          msg.textContent = "No matches found";
          container.appendChild(msg);
          return;
        }

        var list = document.createElement("div");
        list.className = "lookup-results";
        docs.forEach(function (doc) {
          var item = document.createElement("div");
          item.className = "lookup-result";
          var authors = (doc.author_name || []).join(", ");
          var year = doc.first_publish_year ? " (" + doc.first_publish_year + ")" : "";
          item.innerHTML = '<div class="lookup-title">' + esc(doc.title) + '</div>' +
            '<div class="lookup-detail">' + esc(authors) + esc(year) + '</div>';
          item.addEventListener("click", function () {
            document.getElementById("field-title").value = doc.title || "";
            document.getElementById("field-author").value = authors;
            list.remove();
          });
          list.appendChild(item);
        });
        container.appendChild(list);
      }).catch(function () {
        var msg = document.createElement("div");
        msg.className = "lookup-msg";
        msg.textContent = "Lookup unavailable offline";
        container.appendChild(msg);
      });
    });
  }

  // --- Sync / Download Changes ---

  function updateSyncBadge() {
    db.getChangeCount().then(function (n) {
      var btn = document.getElementById("btn-download");
      if (n === 0) {
        btn.disabled = true;
        btn.innerHTML = "No changes";
      } else {
        btn.disabled = false;
        btn.innerHTML = "Download Changes <span class=\"badge\">" + n + "</span>";
      }
    });
  }

  function handleDownload() {
    db.getChanges().then(function (changes) {
      if (!changes || changes.length === 0) return;

      var payload = {
        version: 1,
        exported: new Date().toISOString(),
        changes: changes
      };

      var blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
      var url = URL.createObjectURL(blob);
      var a = document.createElement("a");
      a.href = url;
      a.download = "forage-changes.json";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      if (confirm("Clear changelog?")) {
        db.clearChanges().then(function () {
          updateSyncBadge();
        });
      }
    });
  }

  // --- Event Binding ---

  document.addEventListener("DOMContentLoaded", function () {
    db.init().then(function () {
      render();

      document.getElementById("search").addEventListener("input", debouncedRender);
      document.getElementById("filter").addEventListener("change", render);
      document.getElementById("tag-filter").addEventListener("change", render);

      document.querySelectorAll(".sort-controls button").forEach(function (btn) {
        btn.addEventListener("click", function () {
          var s = btn.dataset.sort;
          if (currentSort === s) {
            sortDir *= -1;
          } else {
            currentSort = s;
            sortDir = 1;
          }
          document.querySelectorAll(".sort-controls button").forEach(function (b) {
            b.classList.remove("active");
          });
          btn.classList.add("active");
          render();
        });
      });

      // Download changes button
      document.getElementById("btn-download").addEventListener("click", handleDownload);

      // FAB opens add modal
      document.getElementById("fab-add").addEventListener("click", function () {
        openModal("add");
      });

      // Book card tap opens edit modal
      document.getElementById("books").addEventListener("click", function (e) {
        var card = e.target.closest(".book[data-id]");
        if (!card) return;
        // Don't open modal if clicking a seller link
        if (e.target.closest(".book-sellers a")) return;
        var id = card.dataset.id;
        db.getBook(id).then(function (book) {
          if (book) openModal("edit", book);
        });
      });

      // Modal form submit
      document.getElementById("book-form").addEventListener("submit", handleSave);

      // Cancel button
      document.querySelector(".btn-cancel").addEventListener("click", closeModal);

      // Delete button
      document.querySelector(".btn-delete").addEventListener("click", handleDelete);

      // Close on overlay click
      document.getElementById("modal-overlay").addEventListener("click", function (e) {
        if (e.target === this) closeModal();
      });
    });
  });
})();
