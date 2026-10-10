import { beforeEach, describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * Every test loads a fresh copy of proviant.js. The script reads localStorage
 * and document.cookie during load and installs a fetch interceptor at module
 * scope, so a fresh copy is the only way to get a clean slate.
 *
 * All of that state lives on jsdom's realm, which survives for the whole test
 * file — so it has to be reset explicitly. Without the reset, localStorage
 * written by one test is replayed into the next test's queue, and a badge left
 * in document.body is still the element getElementById resolves.
 */
const RESET_REALM = "localStorage.clear(); document.body.innerHTML = '';";

function clearCsrfCookie(realm) {
  realm.document.cookie = "csrf_token=; Max-Age=0";
}

function freshProviant({ cookie = "", online = true } = {}) {
  const calls = [];
  const fetchStub = async (url, options) => {
    calls.push({ url, method: (options && options.method) || "GET", headers: (options && options.headers) || null });
    return { status: 200, json: async () => ({ message: "ok" }) };
  };

  const { exports: captured, realm } = loadClassicScript("proviant.js", ["proviant"], {
    prelude: RESET_REALM,
    globals: { fetch: fetchStub, navigator: { onLine: online } },
  });
  clearCsrfCookie(realm);
  // jsdom honours only the first pair of a multi-cookie assignment, whereas a
  // browser receives one Set-Cookie header per pair. Set them one at a time so
  // a cookie *list* can be reproduced faithfully.
  for (const pair of cookie.split(";").map((part) => part.trim()).filter(Boolean)) {
    realm.document.cookie = pair;
  }
  return { proviant: captured.proviant, realm, calls, fetch: realm.fetch };
}

describe("proviant.js CSRF interceptor", () => {
  it("adds the token to mutating methods", async () => {
    const { realm, calls } = freshProviant({ cookie: "csrf_token=tok42" });

    await realm.fetch("https://x.test/a", { method: "POST" });
    await realm.fetch("https://x.test/a", { method: "PATCH" });
    await realm.fetch("https://x.test/a", { method: "PUT" });
    await realm.fetch("https://x.test/a", { method: "DELETE" });

    expect(calls.map((c) => c.method)).toEqual(["POST", "PATCH", "PUT", "DELETE"]);
    expect(calls.map((c) => c.headers["X-CSRF-Token"])).toEqual(["tok42", "tok42", "tok42", "tok42"]);
  });

  it("leaves GET requests untouched", async () => {
    const { realm, calls } = freshProviant({ cookie: "csrf_token=tok42" });

    await realm.fetch("https://x.test/a");

    expect(calls[0].headers).toBeNull();
  });

  it("treats a lowercase method as mutating", async () => {
    const { realm, calls } = freshProviant({ cookie: "csrf_token=tok42" });

    await realm.fetch("https://x.test/a", { method: "post" });

    expect(calls[0].headers["X-CSRF-Token"]).toBe("tok42");
  });

  it("sends no token when the cookie is absent", async () => {
    const { realm, calls } = freshProviant({ cookie: "" });

    await realm.fetch("https://x.test/a", { method: "POST" });

    expect(calls[0].headers?.["X-CSRF-Token"]).toBeUndefined();
  });

  it("preserves caller-supplied headers", async () => {
    const { realm, calls } = freshProviant({ cookie: "csrf_token=tok42" });

    await realm.fetch("https://x.test/a", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
    });

    expect(calls[0].headers).toEqual({ "Content-Type": "application/json", "X-CSRF-Token": "tok42" });
  });

  it("reads the token out of a cookie list, not just a lone cookie", () => {
    const { proviant } = freshProviant({ cookie: "a=1; csrf_token=multi%20token; b=2" });

    expect(proviant._getCsrfToken()).toBe("multi token");
  });
});

describe("proviant.js offline queue", () => {
  it("merges repeated deltas for one product", () => {
    const { proviant } = freshProviant();

    proviant._enqueueAmountDelta(7, 2);
    proviant._enqueueAmountDelta(7, 3);

    expect(proviant._offlineQueue).toHaveLength(1);
    expect(proviant._offlineQueue[0].delta).toBe(5);
  });

  it("drops an entry once the delta nets back to zero", () => {
    const { proviant } = freshProviant();

    proviant._enqueueAmountDelta(7, 2);
    proviant._enqueueAmountDelta(8, 1);
    proviant._enqueueAmountDelta(7, -2);

    expect(proviant._offlineQueue.map((op) => op.productID)).toEqual([8]);
  });

  it("keeps separate entries per product", () => {
    const { proviant } = freshProviant();

    proviant._enqueueAmountDelta(7, 2);
    proviant._enqueueAmountDelta(8, 1);

    expect(proviant._offlineQueue.map((op) => [op.productID, op.delta])).toEqual([
      [7, 2],
      [8, 1],
    ]);
  });

  it("persists the queue to localStorage", () => {
    const { proviant, realm } = freshProviant();

    proviant._enqueueAmountDelta(7, 2);

    expect(JSON.parse(realm.localStorage.getItem("proviant_offline_queue"))).toEqual([
      expect.objectContaining({ productID: 7, delta: 2 }),
    ]);
  });

  it("queues instead of sending while offline", async () => {
    const { proviant, calls } = freshProviant({ online: false });

    const result = await proviant.updateProductAmount(9, 4);

    expect(result.queued).toBe(true);
    expect(calls).toHaveLength(0);
    expect(proviant._offlineQueue.map((op) => op.productID)).toContain(9);
  });

  it("sends straight through while online", async () => {
    const { proviant, calls } = freshProviant({ online: true });

    const result = await proviant.updateProductAmount(9, 4);

    expect(result.queued).toBeUndefined();
    expect(result.code).toBe(200);
    expect(calls).toHaveLength(1);
    expect(proviant._offlineQueue).toHaveLength(0);
  });

  it("restores a queue persisted by an earlier session", () => {
    const { exports: captured } = loadClassicScript("proviant.js", ["proviant"], {
      prelude:
        "localStorage.clear();" +
        "localStorage.setItem('proviant_offline_queue', JSON.stringify([{ productID: 4, delta: -1, ts: 1 }]));",
      globals: { fetch: async () => ({ status: 200, json: async () => ({}) }) },
    });

    expect(captured.proviant._offlineQueue).toEqual([{ productID: 4, delta: -1, ts: 1 }]);
  });
});

describe("proviant.js offline badge", () => {
  it("shows a pending count once items are queued", () => {
    const { proviant, realm } = freshProviant();
    const badge = realm.document.createElement("span");
    badge.id = "offlineQueueCount";
    badge.classList.add("d-none");
    realm.document.body.appendChild(badge);

    proviant._enqueueAmountDelta(7, 2);

    expect(badge.textContent).toBe("(1 pending)");
    expect(badge.classList.contains("d-none")).toBe(false);
  });

  it("hides itself again when the queue empties", () => {
    const { proviant, realm } = freshProviant();
    const badge = realm.document.createElement("span");
    badge.id = "offlineQueueCount";
    realm.document.body.appendChild(badge);

    proviant._enqueueAmountDelta(7, 2);
    expect(badge.classList.contains("d-none")).toBe(false);

    proviant._enqueueAmountDelta(7, -2);
    expect(badge.classList.contains("d-none")).toBe(true);
  });

  // _updateOfflineBadge only rewrites textContent while the queue is non-empty,
  // so the count left behind by a hidden badge is stale — but it is never shown,
  // and the next enqueue refreshes it.
  it("refreshes the count when items are queued again", () => {
    const { proviant, realm } = freshProviant();
    const badge = realm.document.createElement("span");
    badge.id = "offlineQueueCount";
    realm.document.body.appendChild(badge);

    proviant._enqueueAmountDelta(7, 2);
    proviant._enqueueAmountDelta(7, -2);
    proviant._enqueueAmountDelta(8, 1);
    proviant._enqueueAmountDelta(9, 1);

    expect(badge.textContent).toBe("(2 pending)");
    expect(badge.classList.contains("d-none")).toBe(false);
  });

  it("does nothing when the badge is absent", () => {
    const { proviant, realm } = freshProviant();
    expect(realm.document.getElementById("offlineQueueCount")).toBeNull();

    // The queue is still updated — only the badge update is skipped.
    expect(() => proviant._enqueueAmountDelta(7, 2)).not.toThrow();
    expect(proviant._offlineQueue.map((op) => op.productID)).toEqual([7]);
  });
});

describe("proviant.js runWithButtonBusyState", () => {
  const makeButton = () => ({ innerHTML: "<b>Save</b>", disabled: false });

  it("marks the button busy, then shows a success icon", async () => {
    const { proviant } = freshProviant();
    const button = makeButton();
    let busyWhileWorking = null;

    await proviant.runWithButtonBusyState(button, async () => {
      busyWhileWorking = { disabled: button.disabled, html: button.innerHTML };
      return "done";
    }, "Saved!");

    expect(busyWhileWorking.disabled).toBe(true);
    expect(busyWhileWorking.html).toContain("hourglass-split");
    expect(button.innerHTML).toContain("bi-check-lg");
  });

  it("restores the button and reports a failure", async () => {
    const { proviant } = freshProviant();
    const button = makeButton();
    let reported = null;

    await proviant.runWithButtonBusyState(button, async () => {
      throw new Error("boom");
    }, "Saved!", (err) => {
      reported = err.message;
    });

    expect(reported).toBe("boom");
    expect(button.disabled).toBe(false);
    expect(button.innerHTML).toBe("<b>Save</b>");
  });

  it("swallows a returned CANCEL without treating it as success", async () => {
    const { proviant } = freshProviant();
    const button = makeButton();

    await proviant.runWithButtonBusyState(button, async () => proviant.CANCEL, "Saved!");

    expect(button.disabled).toBe(false);
    expect(button.innerHTML).toBe("<b>Save</b>");
  });

  it("swallows a thrown CANCEL too", async () => {
    const { proviant } = freshProviant();
    const button = makeButton();
    let reported = null;

    await proviant.runWithButtonBusyState(
      button,
      async () => {
        throw proviant.CANCEL;
      },
      "Saved!",
      (err) => {
        reported = err;
      },
    );

    expect(reported).toBeNull();
    expect(button.innerHTML).toBe("<b>Save</b>");
  });

  it("is a no-op without a button", async () => {
    const { proviant } = freshProviant();

    await expect(proviant.runWithButtonBusyState(null, async () => "x")).resolves.toBeUndefined();
  });
});

describe("proviant.js pure helpers", () => {
  let proviant;
  beforeEach(() => {
    proviant = freshProviant().proviant;
  });

  describe("formatDate", () => {
    it("zero-pads every component", () => {
      expect(proviant.formatDate(new Date(2026, 0, 5, 9, 3, 7))).toBe("2026-01-05 09:03:07");
    });

    it("does not double-pad two-digit values", () => {
      expect(proviant.formatDate(new Date(2026, 10, 30, 23, 59, 59))).toBe("2026-11-30 23:59:59");
    });
  });

  describe("badgifyCategories", () => {
    it("badges a category that carries a sub-label", () => {
      expect(proviant.badgifyCategories("Dairy:Milk", 5)).toBe(
        '<span class="badge text-bg-secondary me-3">Dairy</span>Milk</br>',
      );
    });

    it("renders a plain category without a badge", () => {
      expect(proviant.badgifyCategories("Bread", 5)).toBe("Bread</br>");
    });

    it("stops at the limit and ignores the overflow", () => {
      expect(proviant.badgifyCategories("a,b,c", 2)).toBe("a</br>b</br>");
    });

    it("returns nothing for a zero limit", () => {
      expect(proviant.badgifyCategories("a,b", 0)).toBe("");
    });

    it("trims surrounding whitespace", () => {
      expect(proviant.badgifyCategories("  Dairy : Milk  ", 1)).toBe(
        '<span class="badge text-bg-secondary me-3">Dairy</span>Milk</br>',
      );
    });

    // Known quirk, pinned so a future change is a deliberate decision:
    // "".split(",") yields [""], so an uncategorised product renders one stray
    // `</br>` rather than nothing. Harmless visually, but invalid markup.
    it("emits a stray break for an empty category string", () => {
      expect(proviant.badgifyCategories("", 5)).toBe("</br>");
    });
  });

  describe("colorExpiry", () => {
    it("marks a past date as danger", () => {
      expect(proviant.colorExpiry("2020-01-01")).toBe("bg-danger");
    });

    it("marks a future date as primary", () => {
      expect(proviant.colorExpiry("2030-01-01")).toBe("bg-primary");
    });
  });
});