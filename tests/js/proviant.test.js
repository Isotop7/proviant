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
  it("sends an quantity delta straight through", async () => {
    const { proviant, calls } = freshProviant({ online: true });

    const result = await proviant.updateProductQuantity(9, 4);

    expect(result.queued).toBeUndefined();
    expect(result.code).toBe(200);
    expect(calls).toHaveLength(1);
  });

  it("carries the client mutation id when the offline queue supplies one", async () => {
    const { proviant, realm } = freshProviant({ online: true });
    const sentBodies = [];
    realm.fetch = async (url, options) => {
      sentBodies.push(JSON.parse(options.body));
      return { status: 200, json: async () => ({ message: "ok" }) };
    };

    await proviant._sendQuantityDelta(9, -1, "mutation-7");

    expect(sentBodies[0]).toEqual({ delta: -1, clientMutationId: "mutation-7" });
  });

  it("tags a rejected fetch so the outbox knows the request never left", async () => {
    const { proviant, realm } = freshProviant({ online: true });
    realm.fetch = async () => {
      throw new TypeError("Failed to fetch");
    };

    const err = await proviant._sendQuantityDelta(9, -1, "m-1").then(() => null, (e) => e);

    expect(err.transport).toBe(true);
    expect(err.message).toBe("Failed to fetch");
  });

  it("leaves an answered response with an unparseable body untagged", async () => {
    // The server did answer, so the mutation deserves a real verdict: burning
    // an attempt parks it visibly, while tagging it transport would retry a
    // broken body forever without ever surfacing the failure. The status rides
    // along so the outbox can pace a 5xx error page instead of burning an
    // attempt per op against it.
    const { proviant, realm } = freshProviant({ online: true });
    realm.fetch = async () => ({
      status: 502,
      ok: false,
      json: async () => {
        throw new SyntaxError("Unexpected token < in JSON");
      },
    });

    const err = await proviant._sendQuantityDelta(9, -1, "m-1").then(() => null, (e) => e);

    expect(err.transport).toBeUndefined();
    expect(err.message).toBe("Unexpected token < in JSON");
    expect(err.status).toBe(502);
  });

  it("sends the client sync id on an ordinary create", async () => {
    const { proviant, realm } = freshProviant({ online: true });
    const sentBodies = [];
    realm.fetch = async (url, options) => {
      sentBodies.push(JSON.parse(options.body));
      return { status: 201, json: async () => ({}) };
    };

    await proviant.createProduct("4006381333931", "2030-01-01", 2, null, false, "sync-42");

    expect(sentBodies[0]).toMatchObject({ barcode: "4006381333931", clientSyncId: "sync-42" });
  });

  it("sends the pack count and no unit on a create", async () => {
    // The stock is counted in packs: the body carries the count, and a unit
    // would be a field the server no longer knows.
    const { proviant, realm } = freshProviant({ online: true });
    const sentBodies = [];
    realm.fetch = async (url, options) => {
      sentBodies.push(JSON.parse(options.body));
      return { status: 201, json: async () => ({}) };
    };

    await proviant.createProduct("4006381333931", "2030-01-01", 3, null, false, null);

    expect(sentBodies[0]).toMatchObject({ quantity: 3 });
    expect(sentBodies[0]).not.toHaveProperty("amount");
    expect(sentBodies[0]).not.toHaveProperty("unit");
  });

  it("asks for a separate row when the duplicate refusal was seen", async () => {
    // The retry of a refused create has to get past the refusal, or it lands
    // on the same refusal again.
    const { proviant, realm } = freshProviant({ online: true });
    const sentBodies = [];
    realm.fetch = async (url, options) => {
      sentBodies.push(JSON.parse(options.body));
      return { status: 201, json: async () => ({}) };
    };

    await proviant.createProduct("4006381333931", "2030-01-01", 1, null, false, "sync-42", true);

    expect(sentBodies[0]).toMatchObject({ allowDuplicate: true, clientSyncId: "sync-42" });
  });

  it("keeps the allow flag out of an ordinary create", async () => {
    const { proviant, realm } = freshProviant({ online: true });
    const sentBodies = [];
    realm.fetch = async (url, options) => {
      sentBodies.push(JSON.parse(options.body));
      return { status: 201, json: async () => ({}) };
    };

    await proviant.createProduct("4006381333931", "2030-01-01", 1, null, false, null);

    expect(sentBodies[0]).not.toHaveProperty("allowDuplicate");
  });

  it("hands the matched product of a refusal to the caller", async () => {
    // The prompt needs the row the server matched: its id to merge into and
    // its name to show.
    const { proviant, realm } = freshProviant({ online: true });
    realm.fetch = async () => ({
      status: 409,
      json: async () => ({
        message: "a matching product already exists",
        duplicate: { id: 42, productName: "Yogurt", quantity: 2 },
      }),
    });

    const response = await proviant.createProduct("4006381333931", "2030-01-01", 1, null, false, "sync-42");

    expect(response.code).toBe(409);
    expect(response.duplicate).toEqual({ id: 42, productName: "Yogurt", quantity: 2 });
    expect(response.clientSyncId).toBe("sync-42");
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

  // Every quantity surface renders the pack count the same way: the count and
  // the localized pack noun.
  describe("quantityLabel", () => {
    it("renders the count with the pack noun", () => {
      expect(proviant.quantityLabel(1)).toBe("1 qty");
      expect(proviant.quantityLabel(2)).toBe("2 qty");
      expect(proviant.quantityLabel(6)).toBe("6 qty");
    });

    it("renders the count of a label element", () => {
      const { proviant: fresh, realm } = freshProviant();
      const element = realm.document.createElement("span");
      element.dataset.quantity = "1";

      fresh.renderQuantityLabel(element);
      expect(element.textContent).toBe("1 qty");

      element.dataset.quantity = "2";
      fresh.renderQuantityLabel(element);
      expect(element.textContent).toBe("2 qty");
    });

    it("leaves an element without a count alone", () => {
      const { proviant: fresh, realm } = freshProviant();
      const element = realm.document.createElement("span");
      element.textContent = "2 qty";

      fresh.renderQuantityLabel(element);
      expect(element.textContent).toBe("2 qty");
    });
  });
});
