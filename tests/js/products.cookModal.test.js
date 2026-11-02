import { beforeEach, describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * The cook modal is the one surface where the pack count of a product is
 * entered by hand, so the row has to carry the stock it may not exceed. The
 * row is written with one data attribute and read with another, and a
 * mismatch does not throw: the read side falls back to 1 and the cook
 * silently consumes a single pack no matter what was entered.
 */
const RESET_REALM = "localStorage.clear(); document.body.innerHTML = '';";

function freshProducts(cookedItems, listBody) {
  const body = listBody === undefined ? [{ id: 7, productName: "Flour", quantity: 5 }] : listBody;
  const fetchStub = async (url) => {
    if (url.indexOf("/api/v1/products?ids=") === 0) {
      return {
        ok: true,
        json: async () => body,
      };
    }
    return { ok: true, json: async () => ({}) };
  };

  const { exports: captured, realm: scriptRealm } = loadClassicScript(
    "products.js",
    ["openCookModalInner", "submitCook"],
    {
      prelude: RESET_REALM,
      globals: {
        fetch: fetchStub,
        proviant: {
          cookProducts: async (items) => {
            cookedItems.push(...items);
            return { code: 200, consumed: items.length, partial: 0, errors: [] };
          },
          showFeedback: () => {},
        },
      },
    },
  );

  scriptRealm.document.body.innerHTML = `
    <div id="cookModalItems"></div>
    <button id="btnCookConfirm" type="button"></button>
  `;

  return { ...captured, realm: scriptRealm };
}

describe("products.js cook modal", () => {
  let cooked;

  beforeEach(() => {
    cooked = [];
  });

  it("carries the stock of the product on the row", async () => {
    const { openCookModalInner, realm } = freshProducts(cooked);

    await openCookModalInner([7], realm.document.getElementById("cookModalItems"));

    const row = realm.document.querySelector(".cook-item-row");
    expect(row).not.toBeNull();
    expect(row.dataset.maxQuantity).toBe("5");
  });

  it("cooks the count the row carries, not a single pack", async () => {
    const { openCookModalInner, submitCook, realm } = freshProducts(cooked);

    await openCookModalInner([7], realm.document.getElementById("cookModalItems"));
    await submitCook();

    expect(cooked).toEqual([{ productId: 7, quantity: 5 }]);
  });

  it("clamps an entered count to the stock the row carries", async () => {
    const { openCookModalInner, submitCook, realm } = freshProducts(cooked);

    await openCookModalInner([7], realm.document.getElementById("cookModalItems"));
    const input = realm.document.querySelector(".cook-item-row input");
    input.value = "99";
    await submitCook();

    expect(cooked).toEqual([{ productId: 7, quantity: 5 }]);
  });

  it("reads the products out of the paged list envelope", async () => {
    const { openCookModalInner, realm } = freshProducts(cooked, {
      products: [{ id: 7, productName: "Flour", quantity: 5 }],
      total: 1,
      limit: 100,
      offset: 0,
    });

    await openCookModalInner([7], realm.document.getElementById("cookModalItems"));

    const row = realm.document.querySelector(".cook-item-row");
    expect(row).not.toBeNull();
    expect(row.dataset.maxQuantity).toBe("5");
  });
});
