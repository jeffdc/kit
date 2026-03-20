// Forage PWA - IndexedDB data layer, API sync, and UI rendering
(function () {
  "use strict";

  var DB_NAME = "forage";
  var DB_VERSION = 1;
  var _db = null;
  var _booksellers = [];

  // --- API Key Management ---

  function getApiKey() {
    return localStorage.getItem("forage_api_key") || null;
  }

  function setApiKey(key) {
    localStorage.setItem("forage_api_key", key);
  }

  function apiHeaders() {
    return {
      "Authorization": "Bearer " + getApiKey(),
      "Content-Type": "application/json"
    };
  }

  // --- IndexedDB Helpers ---

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

  // --- API Sync Layer ---

  function getStoredServerVersion() {
    var t = tx(["meta"], "readonly");
    return reqToPromise(t.objectStore("meta").get("serverVersion")).then(function (entry) {
      return entry ? entry.value : "";
    });
  }

  function setStoredServerVersion(version) {
    var t = tx(["meta"], "readwrite");
    t.objectStore("meta").put({ key: "serverVersion", value: version });
    return txComplete(t);
  }

  function syncFromServer() {
    if (!getApiKey()) return Promise.resolve();

    return fetch("/api/version", { headers: apiHeaders() }).then(function (res) {
      if (!res.ok) throw new Error("HTTP " + res.status);
      return res.json();
    }).then(function (data) {
      var serverVersion = data.version || "";
      return getStoredServerVersion().then(function (storedVersion) {
        if (serverVersion && serverVersion > storedVersion) {
          return Promise.all([
            fetch("/api/books", { headers: apiHeaders() }).then(function (r) {
              if (!r.ok) throw new Error("HTTP " + r.status);
              return r.json();
            }),
            fetch("/api/booksellers", { headers: apiHeaders() }).then(function (r) {
              if (!r.ok) throw new Error("HTTP " + r.status);
              return r.json();
            })
          ]).then(function (results) {
            var books = results[0] || [];
            var booksellers = results[1] || [];

            _booksellers = booksellers;

            // Reseed books WITHOUT clearing changes
            var t = tx(["books", "meta"], "readwrite");
            var bookStore = t.objectStore("books");
            var metaStore = t.objectStore("meta");

            bookStore.clear();
            for (var i = 0; i < books.length; i++) {
              bookStore.put(books[i]);
            }
            metaStore.put({ key: "serverVersion", value: serverVersion });
            metaStore.put({ key: "dataVersion", value: serverVersion });

            return txComplete(t);
          });
        }
      });
    }).catch(function () {
      // Silently continue with local data
    });
  }

  function pushChanges() {
    if (!getApiKey()) return Promise.resolve();

    var t = tx(["changes"], "readonly");
    return reqToPromise(t.objectStore("changes").getAll()).then(function (changes) {
      if (!changes || changes.length === 0) return;

      var payload = {
        version: 1,
        exported: new Date().toISOString(),
        changes: changes
      };

      return fetch("/api/changes", {
        method: "POST",
        headers: apiHeaders(),
        body: JSON.stringify(payload)
      }).then(function (res) {
        if (!res.ok) throw new Error("HTTP " + res.status);
        return res.json();
      }).then(function () {
        return db.clearChanges();
      }).then(function () {
        return syncFromServer();
      });
    }).catch(function () {
      // Silently fail -- changes remain in IndexedDB for next attempt
    });
  }

  function syncAll() {
    return pushChanges().then(function () {
      return syncFromServer();
    }).then(function () {
      updateSyncStatus();
    }).catch(function () {
      updateSyncStatus();
    });
  }

  // --- Database ---

  var db = {
    init: function () {
      return openDB().then(function () {
        // Try to sync from server first
        if (getApiKey()) {
          return syncFromServer().then(function () {
            // After sync, populate booksellers from server data (already done in syncFromServer)
            // Fall back to embedded data if booksellers are still empty
            if (_booksellers.length === 0 && window.__FORAGE_DATA__ && window.__FORAGE_DATA__.booksellers) {
              _booksellers = window.__FORAGE_DATA__.booksellers;
            }
          }).then(function () {
            // Check if we have any books; if not, fall back to embedded data
            return db.listBooks().then(function (books) {
              if (books.length === 0 && window.__FORAGE_DATA__ && window.__FORAGE_DATA__.books && window.__FORAGE_DATA__.books.length > 0) {
                return db._seedFromEmbedded();
              }
            });
          });
        } else {
          // No API key: use embedded data as fallback
          if (window.__FORAGE_DATA__ && window.__FORAGE_DATA__.booksellers) {
            _booksellers = window.__FORAGE_DATA__.booksellers;
          }
          return db.listBooks().then(function (books) {
            if (books.length === 0 && window.__FORAGE_DATA__ && window.__FORAGE_DATA__.books && window.__FORAGE_DATA__.books.length > 0) {
              return db._seedFromEmbedded();
            } else {
              // Check version-based seeding for backward compat
              var t = tx(["meta"], "readonly");
              var store = t.objectStore("meta");
              return reqToPromise(store.get("dataVersion")).then(function (entry) {
                var seedVersion = window.__FORAGE_DATA_VERSION__ || "";
                var storedVersion = entry ? entry.value : "";
                if (!storedVersion || (seedVersion && seedVersion > storedVersion)) {
                  return db._seed(seedVersion);
                }
              });
            }
          });
        }
      });
    },

    _seedFromEmbedded: function () {
      var books = (window.__FORAGE_DATA__ && window.__FORAGE_DATA__.books) || [];
      var seedVersion = window.__FORAGE_DATA_VERSION__ || "";
      var t = tx(["books", "meta"], "readwrite");
      var bookStore = t.objectStore("books");
      var metaStore = t.objectStore("meta");

      bookStore.clear();
      for (var i = 0; i < books.length; i++) {
        bookStore.put(books[i]);
      }
      if (seedVersion) {
        metaStore.put({ key: "dataVersion", value: seedVersion });
      }

      return txComplete(t);
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
      return txComplete(t).then(function () {
        // Push changes in background
        pushChanges().then(function () { updateSyncStatus(); });
        return book;
      });
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
        return txComplete(t2).then(function () {
          // Push changes in background
          pushChanges().then(function () { updateSyncStatus(); });
          return updated;
        });
      });
    },

    deleteBook: function (id) {
      var t = tx(["books", "changes"], "readwrite");
      t.objectStore("books").delete(id);
      t.objectStore("changes").add({ op: "delete", id: id, ts: new Date().toISOString() });
      return txComplete(t).then(function () {
        // Push changes in background
        pushChanges().then(function () { updateSyncStatus(); });
      });
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

  var currentSort = "date_added";
  var sortDir = -1;
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

      updateSyncStatus();

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

        // Secondary info line
        var infoParts = [];
        if (b.date_added) infoParts.push("Added " + formatDate(b.date_added));
        if (b.date_read) infoParts.push("Read " + formatDate(b.date_read));
        if (b.page_count) infoParts.push(b.page_count + "p");
        if (b.first_published) infoParts.push(b.first_published);
        var infoLine = infoParts.length ? '<div class="book-info">' + esc(infoParts.join(" \u00B7 ")) + "</div>" : "";

        return '<div class="book" data-id="' + esc(b.id) + '">' +
          '<div class="book-title">' + esc(b.title) + "</div>" +
          '<div class="book-author">' + esc(b.author) + "</div>" +
          '<div class="book-meta">' +
            '<span class="status status-' + b.status + '">' + b.status + "</span>" +
            (stars ? '<span class="rating">' + stars + "</span>" : "") +
            tags +
          "</div>" +
          infoLine +
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
    document.getElementById("subject-chips").innerHTML = "";

    if (mode === "edit" && book) {
      _editingId = book.id;
      title.textContent = "Edit Book";
      document.getElementById("field-title").value = book.title || "";
      document.getElementById("field-author").value = book.author || "";
      document.getElementById("field-status").value = book.status || "wishlist";
      document.getElementById("field-rating").value = String(book.rating || 0);
      document.getElementById("field-tags").value = (book.tags || []).join(", ");
      document.getElementById("field-pages").value = book.page_count || "";
      document.getElementById("field-year").value = book.first_published || "";
      document.getElementById("field-isbn").value = book.isbn || "";
      document.getElementById("field-notes").value = book.body || "";
      deleteBtn.classList.remove("hidden");
    } else {
      _editingId = null;
      title.textContent = "Add Book";
      deleteBtn.classList.add("hidden");
    }

    injectLookup();
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
    var pageCount = parseInt(document.getElementById("field-pages").value, 10) || 0;
    var firstPublished = parseInt(document.getElementById("field-year").value, 10) || 0;
    var isbn = document.getElementById("field-isbn").value.trim();
    var notes = document.getElementById("field-notes").value.trim();

    var fields = {
      title: titleVal,
      author: authorVal,
      status: status,
      rating: rating,
      tags: tags,
      page_count: pageCount,
      first_published: firstPublished,
      isbn: isbn,
      body: notes
    };

    if (_editingId) {
      db.updateBook(_editingId, fields).then(function () {
        closeModal();
        render();
      });
    } else {
      fields.date_added = new Date().toISOString().slice(0, 10);
      db.createBook(fields).then(function () {
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
      document.getElementById("subject-chips").innerHTML = "";

      btn.disabled = true;
      btn.textContent = "Looking up\u2026";

      var params = [];
      if (titleVal) params.push("title=" + encodeURIComponent(titleVal));
      if (authorVal) params.push("author=" + encodeURIComponent(authorVal));
      if (!titleVal && authorVal) params.push("q=" + encodeURIComponent(authorVal));
      params.push("limit=5");
      params.push("fields=title,author_name,first_publish_year,number_of_pages_median,isbn,subject");
      var url = "https://openlibrary.org/search.json?" + params.join("&");

      fetch(url).then(function (res) {
        if (!res.ok) throw new Error("HTTP " + res.status);
        return res.json();
      }).then(function (data) {
        var docs = data.docs || [];
        if (docs.length === 0) {
          btn.disabled = false;
          btn.textContent = "Look up on Open Library";
          var msg = document.createElement("div");
          msg.className = "lookup-msg";
          msg.textContent = "No matches found";
          container.appendChild(msg);
          return;
        }

        btn.disabled = false;
        btn.textContent = "Look up on Open Library";

        var list = document.createElement("div");
        list.className = "lookup-results";
        docs.forEach(function (doc) {
          var item = document.createElement("div");
          item.className = "lookup-result";
          var authors = (doc.author_name || []).join(", ");
          var details = [];
          if (doc.first_publish_year) details.push(doc.first_publish_year);
          if (doc.number_of_pages_median) details.push(doc.number_of_pages_median + "p");
          item.innerHTML = '<div class="lookup-title">' + esc(doc.title) + '</div>' +
            '<div class="lookup-detail">' + esc(authors) + (details.length ? " \u00B7 " + esc(details.join(", ")) : "") + '</div>';
          item.addEventListener("click", function () {
            document.getElementById("field-title").value = doc.title || "";
            document.getElementById("field-author").value = authors;
            if (doc.number_of_pages_median) {
              document.getElementById("field-pages").value = doc.number_of_pages_median;
            }
            if (doc.first_publish_year) {
              document.getElementById("field-year").value = doc.first_publish_year;
            }
            // Prefer ISBN-13
            var isbns = doc.isbn || [];
            var isbn13 = "";
            for (var i = 0; i < isbns.length; i++) {
              if (isbns[i].length === 13) { isbn13 = isbns[i]; break; }
            }
            document.getElementById("field-isbn").value = isbn13 || isbns[0] || "";
            list.remove();
            showSubjectChips(doc.subject || []);
          });
          list.appendChild(item);
        });
        container.appendChild(list);
      }).catch(function (err) {
        btn.disabled = false;
        btn.textContent = "Look up on Open Library";
        var msg = document.createElement("div");
        msg.className = "lookup-msg";
        msg.textContent = navigator.onLine ? "Lookup failed: " + err.message : "Lookup unavailable offline";
        container.appendChild(msg);
      });
    });
  }

  function showSubjectChips(subjects) {
    var container = document.getElementById("subject-chips");
    container.innerHTML = "";
    if (!subjects.length) return;

    // Deduplicate and limit to 20 most relevant
    var seen = {};
    var unique = [];
    subjects.forEach(function (s) {
      var lower = s.toLowerCase();
      if (!seen[lower]) {
        seen[lower] = true;
        unique.push(s);
      }
    });
    unique = unique.slice(0, 20);

    var label = document.createElement("div");
    label.className = "chips-label";
    label.textContent = "Tap subjects to add as tags:";
    container.appendChild(label);

    unique.forEach(function (subj) {
      var chip = document.createElement("button");
      chip.type = "button";
      chip.className = "subject-chip";
      chip.textContent = subj;
      chip.addEventListener("click", function () {
        var tagsInput = document.getElementById("field-tags");
        var existing = tagsInput.value ? tagsInput.value.split(",").map(function (t) { return t.trim(); }) : [];
        var lower = subj.toLowerCase();
        if (existing.some(function (t) { return t.toLowerCase() === lower; })) {
          chip.classList.add("chip-added");
          return;
        }
        existing.push(subj.toLowerCase());
        tagsInput.value = existing.filter(Boolean).join(", ");
        chip.classList.add("chip-added");
      });
      container.appendChild(chip);
    });
  }

  // --- Sync Status UI ---

  function updateSyncStatus() {
    var btn = document.getElementById("sync-status");
    if (!btn) return;

    if (!getApiKey()) {
      btn.innerHTML = '<span class="sync-status"><span class="sync-dot gray"></span> No API key</span>';
      return;
    }

    if (!navigator.onLine) {
      btn.innerHTML = '<span class="sync-status"><span class="sync-dot gray"></span> Offline</span>';
      return;
    }

    db.getChangeCount().then(function (n) {
      if (n === 0) {
        btn.innerHTML = '<span class="sync-status"><span class="sync-dot green"></span> Synced</span>';
      } else {
        btn.innerHTML = '<span class="sync-status"><span class="sync-dot yellow"></span> ' + n + ' pending</span>';
      }
    });
  }

  function formatDate(dateStr) {
    if (!dateStr) return "";
    var parts = dateStr.split("-");
    if (parts.length !== 3) return dateStr;
    var months = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
    var m = parseInt(parts[1], 10) - 1;
    var d = parseInt(parts[2], 10);
    return months[m] + " " + d + ", " + parts[0];
  }

  function updateSortButtons() {
    document.querySelectorAll(".sort-controls button").forEach(function (b) {
      b.classList.remove("active");
      b.textContent = b.textContent.replace(/ [▲▼]/, "");
      if (b.dataset.sort === currentSort) {
        b.classList.add("active");
        b.textContent += sortDir === 1 ? " ▲" : " ▼";
      }
    });
  }

  // --- API Key Banner ---

  function showApiKeyBanner() {
    var banner = document.getElementById("api-key-banner");
    if (!banner) return;
    if (getApiKey()) {
      banner.classList.add("hidden");
    } else {
      banner.classList.remove("hidden");
    }
  }

  // --- Event Binding ---

  // Re-render and sync when switching back to this tab
  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "visible") {
      syncAll().then(function () { render(); });
    }
  });

  // Sync when coming back online
  window.addEventListener("online", function () {
    syncAll().then(function () { render(); });
  });

  document.addEventListener("DOMContentLoaded", function () {
    db.init().then(function () {
      render();
      updateSyncStatus();
      showApiKeyBanner();

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
          updateSortButtons();
          render();
        });
      });

      // Sync status button — click to trigger manual sync
      document.getElementById("sync-status").addEventListener("click", function () {
        var btn = document.getElementById("sync-status");
        btn.innerHTML = '<span class="sync-status"><span class="sync-dot gray"></span> Syncing...</span>';
        syncAll().then(function () {
          render();
        });
      });

      // API key banner save
      document.getElementById("api-key-save").addEventListener("click", function () {
        var input = document.getElementById("api-key-input");
        var key = input.value.trim();
        if (key) {
          setApiKey(key);
          showApiKeyBanner();
          syncAll().then(function () { render(); });
        }
      });

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

      // Handle ?title=...&author=... URL params (bookmarklet integration)
      var params = new URLSearchParams(window.location.search);
      if (params.has("title") || params.has("author")) {
        openModal("add");
        if (params.get("title")) document.getElementById("field-title").value = params.get("title");
        if (params.get("author")) document.getElementById("field-author").value = params.get("author");
        // Auto-trigger lookup
        var lookupBtn = document.querySelector(".lookup-btn");
        if (lookupBtn) lookupBtn.click();
        // Clean URL without reloading
        history.replaceState(null, "", window.location.pathname);
      }
    });
  });
})();
