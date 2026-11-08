const CACHE_NAME = 'forage-v6';
const APP_ASSETS = ['./', './index.html', './app.js', './style.css', './manifest.json'];

self.addEventListener('install', (event) => {
  self.skipWaiting();
  // cache: 'reload' skips the browser's HTTP cache, which may hold stale
  // copies from before the server sent Cache-Control: no-cache.
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) =>
      cache.addAll(APP_ASSETS.map((url) => new Request(url, { cache: 'reload' })))
    )
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((names) =>
      Promise.all(
        names
          .filter((name) => name !== CACHE_NAME)
          .map((name) => caches.delete(name))
      )
    ).then(() => self.clients.claim())
  );
});

// Network-first: try the network, keep a copy for offline use, and fall back
// to that copy when the network fails.
function networkFirst(request, fetchRequest) {
  return fetch(fetchRequest)
    .then((response) => {
      if (response.ok) {
        const clone = response.clone();
        caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
      }
      return response;
    })
    .catch(() => caches.match(request));
}

self.addEventListener('fetch', (event) => {
  const { request } = event;

  if (request.method !== 'GET') return;

  const url = new URL(request.url);

  // API calls always go straight to the network.
  if (url.pathname.startsWith('/api/')) return;

  if (url.origin === 'https://openlibrary.org') {
    event.respondWith(networkFirst(request, request));
    return;
  }

  if (url.origin !== location.origin) return;

  // Revalidate app files with the server rather than trusting the HTTP cache.
  event.respondWith(networkFirst(request, new Request(request, { cache: 'no-cache' })));
});
