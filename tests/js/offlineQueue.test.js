import { describe, expect, it } from "vitest";
import { defaultResponder, loadOutbox } from "./helpers/offlineQueueHarness.js";

function mountBadge(realm) {
  const badge = realm.document.createElement("span");
  badge.id = "offlineQueueCount";
  badge.classList.add("d-none");
  realm.document.body.appendChild(badge);
  return badge;
}

describe("offlineQueue.js amount deltas", () => {
  it("queues an amount delta instead of sending it while offline", async () => {
    const { proviant, calls } = await loadOutbox({ online: false });

    const result = await proviant.updateProductAmount(9, -2);

    expect(result.queued).toBe(true);
    expect(calls).toHaveLength(0);
    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0]).toMatchObject({ type: "updateAmount", status: "pending" });
    expect(queued[0].payload).toMatchObject({ productID: 9, delta: -2 });
  });

  it("sends the delta straight through while online", async () => {
    const { proviant, calls } = await loadOutbox({ online: true });

    const result = await proviant.updateProductAmount(9, 2);

    expect(result.queued).toBeUndefined();
    expect(result.code).toBe(200);
    expect(calls).toHaveLength(1);
    expect(calls[0].method).toBe("PATCH");
    // The online request carries a token too: a response lost after the server
    // applied the delta has to be replayable as a no-op.
    expect(calls[0].body.clientMutationId).toBeTruthy();
  });

  it("queues the delta when the online request throws, under the id it sent", async () => {
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: () => {
        throw new Error("network down");
      },
    });

    const result = await proviant.updateProductAmount(9, -1);

    expect(result.queued).toBe(true);
    expect(calls).toHaveLength(1);
    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].payload.clientMutationId).toBe(calls[0].body.clientMutationId);
  });

  it("stops the drain without burning an attempt when the request never reaches the server", async () => {
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: () => {
        throw new Error("network down");
      },
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    await proviant.offline.sync();

    expect(calls).toHaveLength(1);
    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].status).toBe("pending");
    expect(queued[0].retryCount).toBe(0);
    expect(queued[0].lastError).toBe("network down");
  });

  it("keeps the delta queued when the server answers with an error status", async () => {
    // _sendAmountDelta resolves for every JSON error body, so an unexamined
    // status here silently dropped the mutation on the floor.
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: () => ({ status: 503, ok: false, json: async () => ({ message: "unavailable" }) }),
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    await proviant.offline.sync();

    expect(calls).toHaveLength(1);
    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].lastError).toBe("HTTP 503");
  });

  it("keeps the delta pending when the session is rejected", async () => {
    // 401 is the token, not the mutation: a drain that runs after the session
    // expired must leave the delta retryable instead of spending an attempt
    // toward parking a change that was never refused.
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.method === "PATCH" ? { status: 401, ok: false, json: async () => ({ message: "user not authenticated" }) } : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    await proviant.offline.sync();

    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].status).toBe("pending");
    expect(queued[0].retryCount).toBe(0);
    expect(await proviant.offline.failedItems()).toHaveLength(0);
  });

  it("drops the delta when the product is gone", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: () => ({ status: 404, ok: false, json: async () => ({ message: "not found" }) }),
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: -1 } });
    await proviant.offline.sync();

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("stamps a mutation id so a replay cannot double-apply", async () => {
    const { proviant } = await loadOutbox({ online: false });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    const queued = await proviant.offline.pendingItems();

    expect(queued[0].payload.clientMutationId).toBe(queued[0].id);
  });

  it("reports a failure instead of throwing when IndexedDB is unavailable", async () => {
    // A private window can ship without IndexedDB entirely; every caller of the
    // wrappers chains .then with no catch, so a rejected enqueue would surface
    // as an unhandled promise with no feedback at all.
    const { proviant } = await loadOutbox({ online: false, withIndexedDB: false });

    const delta = await proviant.updateProductAmount(9, -1);

    expect(delta).toMatchObject({ code: 503, queued: false });
    expect(delta.message).toContain("Offline storage is unavailable");

    const create = await proviant.createProduct("4006381333931", "2030-01-01", 2);
    expect(create).toMatchObject({ code: 503, queued: false });
  });
});

describe("offlineQueue.js product creates", () => {
  it("queues an offline create under its client sync id", async () => {
    const { proviant, calls } = await loadOutbox({ online: false });

    const result = await proviant.createProduct("4006381333931", "2030-01-01", 2);

    expect(result).toMatchObject({ code: 202, queued: true });
    expect(calls).toHaveLength(0);
    const queued = await proviant.offline.pendingItems();
    expect(queued[0].id).toBe(result.clientSyncId);
    expect(queued[0].payload.clientSyncId).toBe(result.clientSyncId);
  });

  it("sends the client sync id online, so a lost response cannot duplicate the product", async () => {
    const { proviant, calls } = await loadOutbox({ online: true });

    const result = await proviant.createProduct("4006381333931", "2030-01-01", 2);

    expect(result.code).toBe(201);
    expect(calls).toHaveLength(1);
    expect(calls[0].body.clientSyncId).toBeTruthy();
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("queues the create when the online request fails after the server committed", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: () => ({ status: 0, ok: false, json: async () => Promise.reject(new Error("network down")) }),
    });

    const result = await proviant.createProduct("4006381333931", "2030-01-01", 2);

    expect(result).toMatchObject({ code: 202, queued: true });
    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].payload.clientSyncId).toBe(result.clientSyncId);
  });

  it("treats an already-synced replay as success and clears the outbox", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.url.includes("/sync-offline")
          ? {
              status: 200,
              ok: true,
              json: async () => ({
                results: call.body.items.map((draft, index) => ({ index, clientSyncId: draft.clientSyncId, status: "already-synced", productId: 12 })),
              }),
            }
          : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await proviant.offline.sync();

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("keeps a create queued when the server rejects the item", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.url.includes("/sync-offline")
          ? {
              status: 200,
              ok: true,
              json: async () => ({
                results: call.body.items.map((draft, index) => ({ index, clientSyncId: draft.clientSyncId, status: "failed", reason: "invalid expiry date" })),
              }),
            }
          : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await proviant.offline.sync();

    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].lastError).toBe("invalid expiry date");
  });

  it("keeps a create queued when the server does not report it", async () => {
    // A 200 whose results never named this item is a contract violation, not a
    // sync: dropping the op as done would lose the product for good.
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.url.includes("/sync-offline")
          ? { status: 200, ok: true, json: async () => ({ results: [] }) }
          : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await proviant.offline.sync();

    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].lastError).toBe("server did not report this item");
  });

  it("parks a create the server marked terminal instead of retrying it", async () => {
    // A blank sync id, one over the length bound, or one held by another
    // member's private product fails identically forever. Spending five drains
    // on it tells the user nothing the park does not, five drains earlier.
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.url.includes("/sync-offline")
          ? {
              status: 200,
              ok: true,
              json: async () => ({
                results: call.body.items.map((draft, index) => ({
                  index,
                  clientSyncId: draft.clientSyncId,
                  status: "failed",
                  reason: "client sync id belongs to another product",
                  terminal: true,
                })),
              }),
            }
          : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await proviant.offline.sync();

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    const parked = await proviant.offline.failedItems();
    expect(parked).toHaveLength(1);
    expect(parked[0].lastError).toBe("client sync id belongs to another product");
    expect(parked[0].retryCount).toBe(0);
  });

  it("retries a non-terminal create failure against its full budget", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.url.includes("/sync-offline")
          ? {
              status: 200,
              ok: true,
              json: async () => ({
                results: call.body.items.map((draft, index) => ({ index, clientSyncId: draft.clientSyncId, status: "failed", reason: "database error" })),
              }),
            }
          : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    for (let attempt = 0; attempt < 3; attempt++) {
      await proviant.offline.sync();
    }

    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].retryCount).toBe(3);
  });

  it("parks a request-level refusal instead of walking the batch into failed", async () => {
    // 400 is a verdict on the body — resubmitting it byte for byte earns the
    // same 400 — so the whole backlog must surface at once, not after five
    // rounds of the same answer.
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.url.includes("/sync-offline")
          ? { status: 400, ok: false, json: async () => ({ message: "request body too large" }) }
          : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await proviant.offline.sync();

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    const parked = await proviant.offline.failedItems();
    expect(parked).toHaveLength(1);
    expect(parked[0].lastError).toBe("request body too large");
  });

  it("keeps retrying a server error, which is not a verdict on the request", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.url.includes("/sync-offline")
          ? { status: 500, ok: false, json: async () => ({ message: "boom" }) }
          : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    for (let attempt = 0; attempt < 3; attempt++) {
      await proviant.offline.sync();
    }

    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].retryCount).toBe(3);
  });

  it("parks an amount delta the server refuses", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.method === "PATCH" ? { status: 400, ok: false, json: async () => ({ message: "clientMutationId too long" }) } : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    await proviant.offline.sync();

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    const parked = await proviant.offline.failedItems();
    expect(parked).toHaveLength(1);
    expect(parked[0].lastError).toBe("HTTP 400");
  });

  it("hands the backlog back when the session is rejected, instead of parking it", async () => {
    // An expired token refused at the batch endpoint would otherwise be read as
    // a verdict on the drafts: every batch parked, the delta behind them walked
    // into 'failed', and signing in again would not have brought any of it back.
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.url.includes("/sync-offline")
          ? { status: 401, ok: false, json: async () => ({ message: "user not authenticated" }) }
          : defaultResponder(call),
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    await proviant.offline.sync();

    // The refused session stops the drain before the delta is attempted —
    // nothing is charged while signing out.
    expect(calls).toHaveLength(1);
    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(2);
    expect(queued.every((item) => item.status === "pending" && item.retryCount === 0)).toBe(true);
    expect(await proviant.offline.failedItems()).toHaveLength(0);
  });

  it("stops the drain when the connection drops, leaving the batch for the re-arm", async () => {
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: () => {
        throw new Error("network down");
      },
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    await proviant.offline.sync();

    // Creates and deltas share the dead connection — neither may burn an
    // attempt, and the delta must not be attempted after the create failed
    // transport.
    expect(calls).toHaveLength(1);
    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(2);
    expect(queued.every((item) => item.status === "pending" && item.retryCount === 0)).toBe(true);
  });
});

describe("offlineQueue.js request pacing", () => {
  it("schedules its own drain for an op enqueued while online", async () => {
    // An op can be queued mid-session (the request threw after the page was
    // up): no online event is coming for it, so without a scheduled drain it
    // sits until the next navigation.
    const { proviant, realm } = await loadOutbox({
      online: true,
      responder: () => {
        throw new Error("network down");
      },
      offlinePrelude:
        "window.__timerDelays = []; var __origSetTimeout = window.setTimeout; " +
        "window.setTimeout = function (fn, ms) { window.__timerDelays.push(ms); return __origSetTimeout(fn, ms); };",
    });

    await proviant.updateProductAmount(9, -1);

    expect(await proviant.offline.pendingItems()).toHaveLength(1);
    expect(realm.__timerDelays).toContain(30000);
  });

  it("does not schedule a drain for an op enqueued while offline", async () => {
    const { proviant, realm } = await loadOutbox({
      online: false,
      offlinePrelude:
        "window.__timerDelays = []; var __origSetTimeout = window.setTimeout; " +
        "window.setTimeout = function (fn, ms) { window.__timerDelays.push(ms); return __origSetTimeout(fn, ms); };",
    });

    await proviant.updateProductAmount(9, -1);

    expect(await proviant.offline.pendingItems()).toHaveLength(1);
    expect(realm.__timerDelays).toEqual([]);
  });

  it("sends the whole create backlog in one request", async () => {
    // One request per queued create against a burst-limited endpoint meant
    // every create after the first was refused.
    const { proviant, calls } = await loadOutbox({ online: true });

    for (let i = 0; i < 5; i++) {
      await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: `400638133393${i}` } });
    }
    await proviant.offline.sync();

    const syncCalls = calls.filter((call) => call.url.includes("/sync-offline"));
    expect(syncCalls).toHaveLength(1);
    expect(syncCalls[0].body.items).toHaveLength(5);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("retries a refused create within the same drain, without burning an attempt", async () => {
    let refusals = 0;
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: (call) => {
        if (!call.url.includes("/sync-offline")) return defaultResponder(call);
        refusals++;
        if (refusals === 1) {
          return { status: 429, ok: false, headers: { get: () => "0" }, json: async () => ({ message: "too many" }) };
        }
        return defaultResponder(call);
      },
    });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await proviant.offline.sync();

    expect(calls.filter((call) => call.url.includes("/sync-offline"))).toHaveLength(2);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("keeps a refused amount delta pending instead of burning an attempt", async () => {
    // Every PATCH is refused, not just the first: the startup drain at 1.5s and
    // the re-armed drain otherwise succeed on a second attempt and empty the
    // outbox before this assertion runs.
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) => {
        if (call.method !== "PATCH") return defaultResponder(call);
        return { status: 429, ok: false, json: async () => ({ message: "too many" }) };
      },
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    await proviant.offline.sync();

    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(1);
    expect(queued[0].status).toBe("pending");
    expect(queued[0].retryCount).toBe(0);
  });

  it("runs one drain at a time", async () => {
    // The online event and the startup timer land together; without a guard both
    // walk the outbox and every op goes out twice.
    const { proviant, calls } = await loadOutbox({ online: true });

    await proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } });
    await Promise.all([proviant.offline.sync(), proviant.offline.sync(), proviant.offline.sync()]);

    expect(calls.filter((call) => call.url.includes("/sync-offline"))).toHaveLength(1);
  });
});

describe("offlineQueue.js retry budget", () => {
  it("parks an op that keeps failing and takes it out of the pending set", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: () => ({ status: 500, ok: false, json: async () => ({ message: "boom" }) }),
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    for (let attempt = 0; attempt < 5; attempt++) {
      await proviant.offline.sync();
    }

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    const failed = await proviant.offline.failedItems();
    expect(failed).toHaveLength(1);
    expect(failed[0].retryCount).toBe(5);
    expect(failed[0].lastError).toBe("HTTP 500");
  });

  it("burns an attempt when the server answers with an unparseable body", async () => {
    // The response arrived, so the failure is real and must be surfaced —
    // tagging it transport would retry a broken body forever, silently.
    const { proviant } = await loadOutbox({
      online: true,
      responder: () => ({
        status: 502,
        ok: false,
        json: async () => {
          throw new SyntaxError("Unexpected token < in JSON");
        },
      }),
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    for (let attempt = 0; attempt < 5; attempt++) {
      await proviant.offline.sync();
    }

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    const failed = await proviant.offline.failedItems();
    expect(failed).toHaveLength(1);
    expect(failed[0].retryCount).toBe(5);
    expect(failed[0].lastError).toBe("Unexpected token < in JSON");
  });

  it("brings a parked op back on request", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: () => ({ status: 500, ok: false, json: async () => ({ message: "boom" }) }),
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    for (let attempt = 0; attempt < 5; attempt++) {
      await proviant.offline.sync();
    }
    const failed = await proviant.offline.failedItems();

    await proviant.offline.retryFailed(failed[0].id);

    expect(await proviant.offline.failedItems()).toHaveLength(0);
    expect(await proviant.offline.pendingItems()).toHaveLength(1);
  });

  it("lets a parked op be discarded", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: () => ({ status: 500, ok: false, json: async () => ({ message: "boom" }) }),
    });

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    for (let attempt = 0; attempt < 5; attempt++) {
      await proviant.offline.sync();
    }
    const failed = await proviant.offline.failedItems();

    await proviant.offline.discard(failed[0].id);

    expect(await proviant.offline.failedItems()).toHaveLength(0);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });
});

describe("offlineQueue.js badge", () => {
  it("counts the pending ops", async () => {
    const { proviant, realm } = await loadOutbox({ online: false });
    const badge = mountBadge(realm);

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });

    expect(badge.textContent).toBe("(1 pending)");
    expect(badge.classList.contains("d-none")).toBe(false);
  });

  it("hides itself once the outbox drains", async () => {
    const { proviant, realm } = await loadOutbox({ online: false });
    const badge = mountBadge(realm);

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    expect(badge.classList.contains("d-none")).toBe(false);

    await proviant.offline.discard((await proviant.offline.pendingItems())[0].id);
    expect(badge.classList.contains("d-none")).toBe(true);
  });

  it("shows the failure count and surfaces the parked error", async () => {
    const { proviant, realm } = await loadOutbox({
      online: true,
      responder: () => ({ status: 500, ok: false, json: async () => ({ message: "boom" }) }),
    });
    const badge = mountBadge(realm);

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    for (let attempt = 0; attempt < 5; attempt++) {
      await proviant.offline.sync();
    }

    expect(badge.textContent).toBe("(0 pending), 1 failed");
    expect(badge.title).toBe("HTTP 500");
  });

  it("survives an absent badge", async () => {
    const { proviant, realm } = await loadOutbox({ online: false });
    expect(realm.document.getElementById("offlineQueueCount")).toBeNull();

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });

    expect(await proviant.offline.pendingItems()).toHaveLength(1);
  });
});

describe("offlineQueue.js legacy migration", () => {
  const legacyPrelude =
    "localStorage.setItem('proviant_offline_queue', JSON.stringify(" +
    "[{ productID: 4, delta: -1, ts: 1 }, { productID: 5, delta: 2, ts: 2 }]));";
  // The legacy queue merged repeated deltas per product, so a second entry for
  // one product should not exist — but a half-written localStorage value can
  // still carry one, and dropping it would drop a change the user made.
  const duplicatedPrelude =
    "localStorage.setItem('proviant_offline_queue', JSON.stringify(" +
    "[{ productID: 4, delta: -1, ts: 1 }, { productID: 4, delta: 2, ts: 1 }]));";

  it("moves the legacy queue into the outbox and clears the legacy key", async () => {
    const { proviant, realm } = await loadOutbox({ online: false, prelude: legacyPrelude });

    const queued = await proviant.offline.pendingItems();
    expect(queued.map((item) => item.payload)).toEqual([
      { productID: 4, delta: -1, clientMutationId: expect.any(String) },
      { productID: 5, delta: 2, clientMutationId: expect.any(String) },
    ]);
    expect(realm.localStorage.getItem("proviant_offline_queue")).toBeNull();
    expect(realm.localStorage.getItem("proviant_offline_queue_backup")).toBeTruthy();
  });

  it("derives outbox ids so a repeated migration cannot double-apply", async () => {
    // The bug this pins: a migration that dies half way leaves the legacy key
    // behind, and re-running it with fresh random ids would send every already
    // imported op again as a new mutation the server has never seen.
    const first = await loadOutbox({ online: false, prelude: legacyPrelude });
    const firstIds = (await first.proviant.offline.pendingItems()).map((item) => item.id);
    const backup = first.realm.localStorage.getItem("proviant_offline_queue_backup");

    const second = await loadOutbox({
      online: false,
      prelude: "localStorage.setItem('proviant_offline_queue', " + JSON.stringify(backup) + ");",
    });
    const secondIds = (await second.proviant.offline.pendingItems()).map((item) => item.id);

    expect(secondIds).toEqual(firstIds);
  });

  it("keeps both entries when a legacy queue repeats a product id", async () => {
    // The bug this pins: keying every entry on the product id made the second
    // overwrite the first, so a delta the user made vanished with no trace and
    // no retry left to recover it.
    const { proviant } = await loadOutbox({ online: false, prelude: duplicatedPrelude });

    const queued = await proviant.offline.pendingItems();
    expect(queued).toHaveLength(2);
    expect(queued[0].id).not.toBe(queued[1].id);
    expect(queued.map((item) => item.payload.delta)).toEqual([-1, 2]);
  });

  it("still derives repeatable ids for a duplicated legacy entry", async () => {
    const first = await loadOutbox({ online: false, prelude: duplicatedPrelude });
    const firstIds = (await first.proviant.offline.pendingItems()).map((item) => item.id);
    const backup = first.realm.localStorage.getItem("proviant_offline_queue_backup");

    const second = await loadOutbox({
      online: false,
      prelude: "localStorage.setItem('proviant_offline_queue', " + JSON.stringify(backup) + ");",
    });
    const secondIds = (await second.proviant.offline.pendingItems()).map((item) => item.id);

    expect(secondIds).toEqual(firstIds);
    expect(new Set(secondIds).size).toBe(2);
  });
});

describe("offlineQueue.js offline product list", () => {
  it("answers offline from the snapshot an earlier online read left behind", async () => {
    const { proviant } = await loadOutbox({
      online: false,
      prelude: "localStorage.setItem('proviant_product_snapshot', JSON.stringify([{ id: 3, productName: 'Milk' }]));",
    });

    const offline = await proviant.getProducts();

    expect(offline).toMatchObject({ code: 200, offline: true });
    expect(offline.products).toEqual([{ id: 3, productName: "Milk" }]);
  });

  it("caches the product list an online read returned", async () => {
    const products = [{ id: 3, productName: "Milk" }];
    const { proviant, realm } = await loadOutbox({
      online: true,
      responder: (call) =>
        call.method === "GET"
          ? { status: 200, ok: true, json: async () => products }
          : defaultResponder(call),
    });

    await proviant.getProducts();

    expect(JSON.parse(realm.localStorage.getItem("proviant_product_snapshot"))).toEqual(products);
  });

  it("parks an op type this build cannot execute instead of deleting it", async () => {
    const { proviant } = await loadOutbox({ online: true });

    await proviant.offline.enqueue({ type: "archiveProduct", payload: { productID: 9 } });
    expect(await proviant.offline.pendingItems()).toHaveLength(1);

    await proviant.offline.sync();

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    const failed = await proviant.offline.failedItems();
    expect(failed).toHaveLength(1);
    expect(failed[0].type).toBe("archiveProduct");
    expect(failed[0].lastError).toContain("archiveProduct");
  });
});

describe("offlineQueue.js open food facts cache", () => {
  it("answers a failed lookup from the cache instead of leaving the store write-only", async () => {
    let lookupFails = false;
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: (call) => {
        if (!call.url.includes("/openfoodfacts/")) return defaultResponder(call);
        if (lookupFails) throw new Error("network down");
        return { status: 200, ok: true, json: async () => ({ productName: "Oat milk" }) };
      },
    });

    const online = await proviant.getOpenFoodFactsData("4006381333931");
    expect(online).toEqual({ code: 200, message: { productName: "Oat milk" } });

    lookupFails = true;
    const offline = await proviant.getOpenFoodFactsData("4006381333931");

    expect(offline).toMatchObject({ code: 200, offline: true });
    expect(offline.message).toEqual({ productName: "Oat milk" });
    expect(calls.filter((call) => call.url.includes("/openfoodfacts/"))).toHaveLength(2);
  });

  it("still answers a lookup whose cache write fails", async () => {
    // The write follows an answered lookup, so a browser without IndexedDB —
    // the case the rest of this file defends against — must not have a
    // successful response turned into a failed scan.
    const { proviant } = await loadOutbox({
      online: true,
      withIndexedDB: false,
      responder: (call) => (call.url.includes("/openfoodfacts/") ? { status: 200, ok: true, json: async () => ({ productName: "Oat milk" }) } : defaultResponder(call)),
    });

    // Caught as a value rather than awaited to rejection: the rejection carries
    // a stack inside the shipped script, and the assertion reports on this
    // file's own frame instead.
    const result = await proviant.getOpenFoodFactsData("4006381333931").then((answer) => answer, () => null);

    expect(result).toEqual({ code: 200, message: { productName: "Oat milk" } });
  });

  it("still reports a failed lookup for an uncached barcode", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: () => {
        throw new Error("network down");
      },
    });

    const err = await proviant.getOpenFoodFactsData("4006381333931").then(() => null, (e) => e);

    expect(err.message).toBe("network down");
  });
});