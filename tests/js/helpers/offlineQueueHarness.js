import { IDBFactory } from "fake-indexeddb";
import { loadClassicScript } from "./loadScript.js";

/**
 * offlineQueue.js is a classic script that reaches into proviant.js: it wraps
 * proviant.createProduct / updateProductAmount / getProducts and sends amount
 * deltas through proviant._sendAmountDelta. So both files are loaded into the
 * same realm, in the order base.tmpl declares them.
 *
 * Each load gets its own IDBFactory. IndexedDB is realm-global and persists for
 * the whole file, so a shared factory would leak one test's outbox into the
 * next — the same reason the realm's localStorage and document are reset.
 */
export async function loadOutbox({ online = true, responder = defaultResponder, prelude = "", offlinePrelude = "", withIndexedDB = true } = {}) {
  const calls = [];
  const indexedDB = new IDBFactory();
  const fetchStub = async (url, options) => {
    const call = {
      url: String(url),
      method: (options && options.method) || "GET",
      body: options && options.body ? JSON.parse(options.body) : null,
    };
    calls.push(call);
    return responder(call);
  };

  const globals = {
    fetch: fetchStub,
    navigator: { onLine: online },
  };
  if (withIndexedDB) {
    globals.indexedDB = indexedDB;
  } else {
    // The realm outlives a single load, so a factory defined by an earlier
    // test would still satisfy openDB's `in window` check here.
    prelude = "delete window.indexedDB;" + prelude;
  }
  const reset = "localStorage.clear(); document.body.innerHTML = '';" + prelude;

  const base = loadClassicScript("proviant.js", ["proviant"], {
    prelude: reset,
    globals: { ...globals },
  });
  // base.tmpl loads proviant.js first; the outbox script reads window.proviant,
  // which proviant.js publishes for exactly this reason.
  if (base.realm.proviant !== base.exports.proviant) {
    throw new Error("proviant.js did not publish window.proviant for offlineQueue.js to find.");
  }

  const loaded = loadClassicScript("offlineQueue.js", [], { prelude: offlinePrelude, globals });
  const proviant = loaded.realm.proviant;
  // The legacy import runs at load; waiting on it keeps a test from draining an
  // outbox the migration has not finished filling.
  await proviant.offline.ready;
  return { proviant, realm: loaded.realm, calls, indexedDB };
}

export function defaultResponder(call) {
  if (call.url.includes("/sync-offline")) {
    const items = call.body.items;
    return {
      status: 200,
      ok: true,
      json: async () => ({
        results: items.map((draft, index) => ({ index, clientSyncId: draft.clientSyncId, status: "created", productId: 7 + index })),
      }),
    };
  }
  if (call.method === "POST") {
    return { status: 201, ok: true, json: async () => ({}) };
  }
  return { status: 200, ok: true, json: async () => ({ message: "Product 7 amount updated" }) };
}
