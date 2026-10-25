import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

// The partial consume/waste surface is reached by a click on the row button
// and left by a click on one of the dialog buttons. Both hops are delegation,
// and a selector that no longer matches fails silently: the dialog never
// opens, the confirm never fires, and nothing throws. The clicks therefore
// have to be dispatched for real.
//
// One load only: every load adds another click listener to the shared
// document, so a second load would fire the handler twice per click.
const PAGE_MARKUP = `
  <div id="productRows">
    <div class="list-row" data-id="7">
      <div class="col-actions">
        <button type="button" class="btn-partial-action--icon btn-partial-action" data-product-id="7"></button>
      </div>
    </div>
  </div>
  <dialog id="partialActionModal">
    <div id="partialActionProductName"></div>
    <input id="partialActionQuantity" type="number" value="1">
    <div id="partialActionQuantityError"></div>
    <small id="partialActionStock"></small>
    <button id="btnPartialConsumeConfirm" type="button"></button>
    <button id="btnPartialWasteConfirm" type="button"></button>
  </dialog>
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("products.js partial action delegation", () => {
  it("opens the dialog from the row button and consumes through the dialog button", async () => {
    const actions = [];
    const fetchStub = vi.fn().mockImplementation(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve([{ id: 7, productName: "Yogurt", quantity: 5 }]),
      }),
    );

    class Dialog {
      constructor(element) {
        this.element = element;
      }
      show() {
        this.element.setAttribute("open", "");
      }
      hide() {
        this.element.removeAttribute("open");
      }
    }

    const { realm } = loadClassicScript("products.js", [], {
      prelude: `localStorage.clear(); document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
      globals: {
        fetch: fetchStub,
        bootstrap: { Dialog },
        proviant: {
          consumeProduct: async (productID, quantity) => {
            actions.push({ action: "consume", productID, quantity });
            return { code: 200, message: "recorded", removed: quantity };
          },
          wasteProduct: async (productID, quantity) => {
            actions.push({ action: "waste", productID, quantity });
            return { code: 200, message: "recorded", removed: quantity };
          },
          showFeedback: () => {},
        },
      },
    });
    const doc = realm.document;

    doc
      .querySelector(".btn-partial-action")
      .dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(doc.getElementById("partialActionModal").hasAttribute("open")).toBe(true);
    expect(doc.getElementById("partialActionStock").textContent).toContain("5");

    doc.getElementById("partialActionQuantity").value = "2";
    doc
      .getElementById("btnPartialConsumeConfirm")
      .dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(actions).toEqual([{ action: "consume", productID: 7, quantity: 2 }]);
    expect(doc.getElementById("partialActionModal").hasAttribute("open")).toBe(false);
  });
});
