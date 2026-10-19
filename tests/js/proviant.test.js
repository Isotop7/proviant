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
  it("sends an amount delta straight through", async () => {
    const { proviant, calls } = freshProviant({ online: true });

    const result = await proviant.updateProductAmount(9, 4);

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

    await proviant._sendAmountDelta(9, -1, "mutation-7");

    expect(sentBodies[0]).toEqual({ delta: -1, clientMutationId: "mutation-7" });
  });

  it("tags a rejected fetch so the outbox knows the request never left", async () => {
    const { proviant, realm } = freshProviant({ online: true });
    realm.fetch = async () => {
      throw new TypeError("Failed to fetch");
    };

    const err = await proviant._sendAmountDelta(9, -1, "m-1").then(() => null, (e) => e);

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

    const err = await proviant._sendAmountDelta(9, -1, "m-1").then(() => null, (e) => e);

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

  // Display-only mirror of util/units.go FromBase; the amounts are stored in
  // base units (g / ml / count) and rendered in the household system.
  describe("formatUnit", () => {
    it("renders mass in the metric system", () => {
      expect(proviant.formatUnit(500, "g", "metric")).toBe("500 g");
      expect(proviant.formatUnit(1500, "g", "metric")).toBe("1.5 kg");
    });

    it("renders a unit-less amount as its bare quantity", () => {
      expect(proviant.formatUnit(3, "", "metric")).toBe("3");
    });

    it("leaves a count unit untouched", () => {
      expect(proviant.formatUnit(3, "pkg", "metric")).toBe("3 pkg");
    });

    it("renders mass in the imperial system", () => {
      expect(proviant.formatUnit(454, "g", "imperial")).toBe("1 lb");
      expect(proviant.formatUnit(100, "g", "imperial")).toBe("3.5 oz");
    });

    it("renders volume in the imperial system", () => {
      expect(proviant.formatUnit(480, "ml", "imperial")).toBe("2 cup");
      expect(proviant.formatUnit(30, "ml", "imperial")).toBe("2 tbsp");
    });

    it("passes an unknown unit through", () => {
      expect(proviant.formatUnit(4, "handful", "metric")).toBe("4 handful");
    });
  });

  // A measured amount is one package no matter its size, and a single count
  // item is one package too, so both read as the localized "1 qty".
  describe("quantityLabel", () => {
    it("shows a measured amount as one package", () => {
      expect(proviant.quantityLabel(500, "g", "metric")).toBe("1 qty");
      expect(proviant.quantityLabel(2000, "ml", "metric")).toBe("1 qty");
    });

    it("shows a single count item as one package", () => {
      expect(proviant.quantityLabel(1, "pcs", "metric")).toBe("1 qty");
      expect(proviant.isPackageQuantity(1, "jar")).toBe(true);
    });

    it("keeps the unit for several count items", () => {
      expect(proviant.quantityLabel(6, "pcs", "metric")).toBe("6 pcs");
      expect(proviant.isPackageQuantity(6, "pcs")).toBe(false);
    });

    it("leaves a unit-less or unknown amount alone", () => {
      expect(proviant.quantityLabel(2, "", "metric")).toBe("2");
      expect(proviant.quantityLabel(4, "handful", "metric")).toBe("4 handful");
    });

    it("resolves an alias the way the server does", () => {
      expect(proviant.isPackageQuantity(1, "pc")).toBe(true);
      expect(proviant.quantityLabel(1, "piece", "metric")).toBe("1 qty");
      expect(proviant.formatUnit(2000, "Millilitres", "metric")).toBe("2 l");
    });

    it("drives the stepper label through the same rule", () => {
      const { proviant: fresh, realm } = freshProviant();
      const element = realm.document.createElement("span");
      element.dataset.amount = "1";
      element.dataset.unit = "pcs";

      fresh.renderAmountLabel(element);
      expect(element.textContent).toBe("1 qty");

      element.dataset.amount = "2";
      fresh.renderAmountLabel(element);
      expect(element.textContent).toBe("2 pcs");
    });
  });

  // The stepper must move by one *displayed* unit, otherwise a "2 kg" label
  // stays at 2 kg when the server adds a single gram.
  describe("amountStep", () => {
    it("uses the displayed unit as the step", () => {
      expect(proviant.amountStep(2000, "g", "metric")).toBe(1000);
      expect(proviant.amountStep(500, "g", "metric")).toBe(1);
      expect(proviant.amountStep(2000, "ml", "metric")).toBe(1000);
      expect(proviant.amountStep(454, "g", "imperial")).toBe(454);
      expect(proviant.amountStep(30, "ml", "imperial")).toBe(15);
    });

    it("steps a count or unit-less amount by one", () => {
      expect(proviant.amountStep(3, "pkg", "metric")).toBe(1);
      expect(proviant.amountStep(3, "", "metric")).toBe(1);
    });
  });

  // Rescaling the edit-form amount when the picker changes keeps 2000 g from
  // being saved as 2000 kg, and a conversion that would round returns null so
  // the caller can refuse the switch instead of altering the quantity.
  describe("convertUnit", () => {
    it("rescales between units of one kind", () => {
      expect(proviant.convertUnit(2000, "g", "kg")).toBe(2);
      expect(proviant.convertUnit(2, "kg", "g")).toBe(2000);
      expect(proviant.convertUnit(1, "l", "ml")).toBe(1000);
      expect(proviant.convertUnit(30, "ml", "tbsp")).toBe(2);
    });

    it("returns null when the conversion would round", () => {
      expect(proviant.convertUnit(500, "g", "kg")).toBeNull();
      expect(proviant.convertUnit(454, "g", "lb")).toBeNull();
    });

    it("returns null when no conversion is defined", () => {
      expect(proviant.convertUnit(2000, "g", "")).toBeNull();
      expect(proviant.convertUnit(2000, "g", "ml")).toBeNull();
      expect(proviant.convertUnit(1, "pcs", "pkg")).toBeNull();
    });
  });
});
