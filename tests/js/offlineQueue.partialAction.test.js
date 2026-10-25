import { describe, expect, it } from "vitest";
import { loadOutbox } from "./helpers/offlineQueueHarness.js";

describe("offlineQueue.js partial consume/waste", () => {
  it("queues consume and waste actions with their quantity and mutation id", async () => {
    for (const action of ["consume", "waste"]) {
      const { proviant, calls } = await loadOutbox({ online: false });

      const result = await proviant[`${action}Product`](9, 2);

      expect(result).toMatchObject({ code: 202, queued: true });
      expect(calls).toHaveLength(0);
      const queued = await proviant.offline.pendingItems();
      expect(queued).toHaveLength(1);
      expect(queued[0]).toMatchObject({ type: `${action}Product`, status: "pending" });
      expect(queued[0].payload).toMatchObject({ productID: 9, quantity: 2, clientMutationId: queued[0].id });
    }
  });

  it("keeps whole-stock calls bodyless at the API level", async () => {
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: () => ({ status: 200, ok: true, json: async () => ({ message: "recorded" }) }),
    });

    const result = await proviant.consumeProduct(9);

    expect(result.code).toBe(200);
    expect(calls).toHaveLength(1);
    expect(calls[0].method).toBe("POST");
    expect(calls[0].url).toContain("/api/v1/products/9/consume");
    expect(calls[0].body).not.toHaveProperty("quantity");
    expect(calls[0].body.clientMutationId).toBeTruthy();
  });

  it("reuses one mutation id across a lost response and the queued drain", async () => {
    let attempt = 0;
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: () => {
        attempt++;
        if (attempt === 1) throw new Error("response lost");
        return { status: 200, ok: true, json: async () => ({ message: "recorded" }) };
      },
    });

    const result = await proviant.consumeProduct(9, 1);
    expect(result).toMatchObject({ code: 202, queued: true });
    const queued = await proviant.offline.pendingItems();
    const mutationID = calls[0].body.clientMutationId;
    expect(queued[0].payload.clientMutationId).toBe(mutationID);

    await proviant.offline.sync();

    expect(calls).toHaveLength(2);
    expect(calls[1].body).toMatchObject({ quantity: 1, clientMutationId: mutationID });
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("drains quantity deltas and partial actions in their original order", async () => {
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: () => ({ status: 200, ok: true, json: async () => ({ message: "recorded" }) }),
    });
    const now = Date.now();
    await proviant.offline.enqueue({ type: "updateAmount", createdAt: now, payload: { productID: 9, delta: 1 } });
    await proviant.offline.enqueue({ type: "consumeProduct", createdAt: now + 1, payload: { productID: 9, quantity: 1 } });
    await proviant.offline.enqueue({ type: "wasteProduct", createdAt: now + 2, payload: { productID: 9, quantity: 1 } });

    await proviant.offline.sync();

    expect(calls.map((call) => [call.method, call.url])).toEqual([
      ["PATCH", expect.stringContaining("/api/v1/products/9/quantity")],
      ["POST", expect.stringContaining("/api/v1/products/9/consume")],
      ["POST", expect.stringContaining("/api/v1/products/9/waste")],
    ]);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("drops a replayed full action when the archived product now answers 404", async () => {
    const { proviant } = await loadOutbox({
      online: true,
      responder: () => ({ status: 404, ok: false, json: async () => ({ message: "Product not found" }) }),
    });
    await proviant.offline.enqueue({ type: "wasteProduct", payload: { productID: 9 } });

    await proviant.offline.sync();

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    expect(await proviant.offline.failedItems()).toHaveLength(0);
  });

  it("keeps a throttled action ahead of the ones queued behind it", async () => {
    // A later consume that overtakes an earlier one reverses what the user
    // asked for: the earlier pack would still be counted after the later one
    // archived the product.
    const sent = [];
    let attempt = 0;
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: (call) => {
        sent.push(call.url);
        attempt++;
        if (attempt === 1) {
          return {
            status: 429,
            ok: false,
            headers: { get: () => "0" },
            json: async () => ({ message: "slow down" }),
          };
        }
        return { status: 200, ok: true, json: async () => ({ message: "recorded" }) };
      },
    });
    const now = Date.now();
    await proviant.offline.enqueue({ type: "consumeProduct", createdAt: now, payload: { productID: 9, quantity: 1 } });
    await proviant.offline.enqueue({ type: "wasteProduct", createdAt: now + 1, payload: { productID: 9, quantity: 1 } });

    await proviant.offline.sync();

    expect(sent).toHaveLength(3);
    expect(sent[0]).toContain("/consume");
    expect(sent[1]).toContain("/consume");
    expect(sent[2]).toContain("/waste");
    expect(calls).toHaveLength(3);
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
  });

  it("retries a conflicting action in place instead of parking it", async () => {
    // The server answers 409 when its guarded write lost the race: the same
    // request can succeed a moment later, so the op must stay queued. A 4xx
    // would be treated as a refusal and parked as failed for good.
    const sent = [];
    let attempt = 0;
    const { proviant } = await loadOutbox({
      online: true,
      responder: (call) => {
        sent.push(call.url);
        attempt++;
        if (attempt === 1) {
          return {
            status: 409,
            ok: false,
            json: async () => ({ message: "Product was changed by another request, please retry" }),
          };
        }
        return { status: 200, ok: true, json: async () => ({ message: "recorded" }) };
      },
    });
    await proviant.offline.enqueue({ type: "consumeProduct", payload: { productID: 9, quantity: 1 } });

    await proviant.offline.sync();

    expect(sent).toHaveLength(2);
    expect(sent[0]).toContain("/consume");
    expect(sent[1]).toContain("/consume");
    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    expect(await proviant.offline.failedItems()).toHaveLength(0);
  });

  it("parks a queued action the server can no longer deduplicate", async () => {
    // The server forgets an applied mutation id after a bounded window, so a
    // replay past it would apply the change a second time.
    const { proviant, calls } = await loadOutbox({
      online: true,
      responder: () => ({ status: 200, ok: true, json: async () => ({ message: "recorded" }) }),
    });
    const longAgo = Date.now() - 31 * 24 * 60 * 60 * 1000;
    await proviant.offline.enqueue({ type: "consumeProduct", createdAt: longAgo, payload: { productID: 9, quantity: 1 } });

    await proviant.offline.sync();

    expect(calls).toHaveLength(0);
    const failed = await proviant.offline.failedItems();
    expect(failed).toHaveLength(1);
    expect(failed[0].lastError).toContain("too old");
  });
});
