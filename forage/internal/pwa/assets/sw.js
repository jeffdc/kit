// Service worker. The server stamps the build version into BUILD below, so
// each deploy installs a fresh cache and the app reloads into it.
const BUILD = "__FORAGE_BUILD__";
const CACHE = "forage-" + BUILD;
const DEV = BUILD.startsWith("dev");

const APP = [
  "./", "index.html", "style.css", "manifest.json",
  "js/main.js", "js/core.js", "js/store.js", "js/sync.js", "js/idb.js", "js/views.js", "js/icons.js",
  "icons/icon-180.png", "icons/icon-192.png",
];

self.addEventListener("install", (event) => {
  self.skipWaiting();
  // cache: "reload" skips the HTTP cache so the new build's files are fetched fresh.
  event.waitUntil(caches.open(CACHE).then((c) => c.addAll(APP.map((u) => new Request(u, { cache: "reload" })))));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((names) => Promise.all(names.filter((n) => n !== CACHE).map((n) => caches.delete(n))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== location.origin || url.pathname.startsWith("/api/")) return;

  // Pages load the app shell, whatever query they carry (?add=…).
  const key = req.mode === "navigate" ? "./" : req;

  if (DEV) {
    // Development: always fetch, so edits show on reload.
    event.respondWith(fetch(req).catch(() => caches.match(key)));
    return;
  }
  // Production: the cache holds exactly this build's files.
  event.respondWith(
    caches.match(key, { ignoreSearch: req.mode === "navigate" }).then((hit) => hit || fetch(req)),
  );
});
