// A small promise wrapper over IndexedDB, and Forage's schema.

const DB_NAME = "forage";
const DB_VERSION = 3;

// Version history:
//   1-2: original PWA — books, changes (its outgoing queue), meta, booksellers
//   3:   reads and ops (the new queue). The old "changes" store is kept until
//        the app has sent whatever it still holds (see sync.js).
function upgrade(db) {
  const has = (name) => db.objectStoreNames.contains(name);
  if (!has("books")) db.createObjectStore("books", { keyPath: "id" });
  if (!has("meta")) db.createObjectStore("meta", { keyPath: "key" });
  if (!has("booksellers")) db.createObjectStore("booksellers", { keyPath: "id" });
  if (!has("reads")) db.createObjectStore("reads", { keyPath: "id" });
  if (!has("ops")) db.createObjectStore("ops", { keyPath: "seq", autoIncrement: true });
}

let dbPromise = null;

export function open() {
  if (!dbPromise) {
    dbPromise = new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = () => upgrade(req.result);
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
      req.onblocked = () => reject(new Error("Close other Forage tabs and reload."));
    });
  }
  return dbPromise;
}

export const done = (req) =>
  new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });

export const complete = (t) =>
  new Promise((resolve, reject) => {
    t.oncomplete = () => resolve();
    t.onerror = () => reject(t.error);
    t.onabort = () => reject(t.error || new Error("transaction aborted"));
  });

export async function hasStore(name) {
  return (await open()).objectStoreNames.contains(name);
}

export async function getAll(store) {
  const db = await open();
  return done(db.transaction(store).objectStore(store).getAll());
}

export async function getMeta(key) {
  const db = await open();
  const row = await done(db.transaction("meta").objectStore("meta").get(key));
  return row ? row.value : undefined;
}

export async function setMeta(key, value) {
  const db = await open();
  const t = db.transaction("meta", "readwrite");
  t.objectStore("meta").put({ key, value });
  return complete(t);
}

/** Runs fn(stores) inside one readwrite transaction over the named stores. */
export async function write(names, fn) {
  const db = await open();
  const t = db.transaction(names, "readwrite");
  const stores = Object.fromEntries(names.map((n) => [n, t.objectStore(n)]));
  fn(stores);
  return complete(t);
}
