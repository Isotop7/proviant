import { describe, expect, it } from "vitest";
import { defaultResponder, loadOutbox } from "./helpers/offlineQueueHarness.js";

// Splitting a create backlog takes more than util.ReceiptBulkMaxItems items to
// observe, and one such test costs about a second of IndexedDB round trips. Both
// that wall clock and the accumulated timers of a shared window make it noisy
// beside the rest of the outbox suite — loadScript.js hands every load the same
// jsdom window, so a long test here lets an earlier load's drain timer fire
// during a later one. Same reason the delegation tests get a file each.
describe("offlineQueue.js create batching", () => {
  it("splits a backlog at the server's item cap", async () => {
    const batches = [];
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) => {
        if (!call.url.includes("/sync-offline")) return defaultResponder(call);
        batches.push(call.body.items.length);
        return defaultResponder(call);
      },
    });

    await Promise.all(
      Array.from({ length: 101 }, () =>
        proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } })
      )
    );
    await proviant.offline.sync();

    expect(batches).toEqual([100, 1]);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("splits a batch that would exceed the server's byte cap", async () => {
    // util.ReceiptBulkMaxBodyBytes is enforced server-side and answered as a
    // 4xx, which parks the whole batch. Splitting on the item count alone lets
    // one fat draft refuse every draft queued behind it, and a draft's name is
    // only clamped once the server has already read it.
    const batches = [];
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) => {
        if (!call.url.includes("/sync-offline")) return defaultResponder(call);
        batches.push(call.body.items.length);
        return defaultResponder(call);
      },
    });

    for (let i = 0; i < 3; i++) {
      await proviant.offline.enqueue({
        type: "createProduct",
        payload: { barcode: "4006381333931", productName: "x".repeat(200000) },
      });
    }
    await proviant.offline.sync();

    expect(batches).toEqual([2, 1]);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("measures the byte cap in bytes, not characters", async () => {
    // A name in a multi-byte script is several bytes per character. Measuring
    // the split in characters would undercount it and hand the server a body
    // over the very cap the split exists to respect.
    const batches = [];
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) => {
        if (!call.url.includes("/sync-offline")) return defaultResponder(call);
        batches.push(call.body.items.length);
        return defaultResponder(call);
      },
    });

    // Three drafts, each ~200k UTF-8 bytes from a 100k-character name: two fit
    // under the 413696-byte cap, but a split measured in characters sees only
    // 100k per draft and would send all three in one body.
    for (let i = 0; i < 3; i++) {
      await proviant.offline.enqueue({
        type: "createProduct",
        payload: { barcode: "4006381333931", productName: "ä".repeat(100000) },
      });
    }
    await proviant.offline.sync();

    expect(batches).toEqual([2, 1]);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("splits at the server-configured byte cap", async () => {
    // The cap arrives with the page as window.proviantOffline. A
    // smaller configured cap must tighten the split, or an operator
    // who lowers the server cap gets batches the client still fills
    // past it — and a refusal parks the whole batch.
    const batches = [];
    const { proviant } = await loadOutbox({
      online: true,
      offlinePrelude: "window.proviantOffline = { maxBatchBytes: 150000 };",
      responder: (call) => {
        if (!call.url.includes("/sync-offline")) return defaultResponder(call);
        batches.push(call.body.items.length);
        return defaultResponder(call);
      },
    });

    // Three drafts of ~200k payload bytes each: under the injected
    // 150000-byte cap every batch holds exactly one.
    for (let i = 0; i < 3; i++) {
      await proviant.offline.enqueue({
        type: "createProduct",
        payload: { barcode: "4006381333931", productName: "x".repeat(200000) },
      });
    }
    await proviant.offline.sync();

    expect(batches).toEqual([1, 1, 1]);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("charges a server error to one batch, not to the whole backlog", async () => {
    // The bug this pins: a 5xx fell through to the next batch, so a single
    // proxy blip spent one attempt of every batch at once and a short outage
    // walked an entire backlog into 'failed' with nothing left to retry.
    let requests = 0;
    let failing = true;
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) => {
        if (!call.url.includes("/sync-offline")) return defaultResponder(call);
        requests++;
        if (failing) return { status: 502, ok: false, json: async () => ({ message: "bad gateway" }) };
        return defaultResponder(call);
      },
    });

    await Promise.all(
      Array.from({ length: 101 }, () =>
        proviant.offline.enqueue({ type: "createProduct", payload: { barcode: "4006381333931" } })
      )
    );
    await proviant.offline.sync();

    expect(requests).toBe(1);
    const queued = await proviant.offline.pendingItems();
    expect(queued.filter((item) => item.retryCount === 1)).toHaveLength(100);
    expect(queued.filter((item) => item.retryCount === 0)).toHaveLength(1);

    // The batch that was never sent must still drain once the server recovers,
    // rather than arriving already spent.
    failing = false;
    await proviant.offline.sync();
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });
});