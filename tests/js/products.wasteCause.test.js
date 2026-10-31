import { describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

const RESET_REALM = "localStorage.clear(); document.body.innerHTML = '';";

function setupWasteCause() {
  const actionCalls = [];
  const bulkCalls = [];
  const feedback = [];
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
  const recordAction = (action) => async (productID, quantity, _mutationID, cause) => {
    actionCalls.push({ action, productID, quantity, cause });
    return { code: 200, message: "recorded" };
  };
  const { exports, realm } = loadClassicScript(
    "products.js",
    ["openPartialActionModal", "submitPartialAction", "openBulkWasteModal", "submitBulkWaste"],
    {
      prelude: RESET_REALM + "window.__reloads = 0; function clearBulkSelectionAndReload() { window.__reloads++; }",
      globals: {
        fetch: async () => ({
          ok: true,
          json: async () => [{ id: 7, productName: "Yogurt", quantity: 5 }],
        }),
        bootstrap: { Dialog },
        proviant: {
          consumeProduct: recordAction("consume"),
          wasteProduct: recordAction("waste"),
          showFeedback: (type, title, message) => feedback.push({ type, title, message }),
          bulkAction: async (type, ids, options) => {
            bulkCalls.push({ type, ids, options });
          },
        },
      },
    },
  );
  realm.document.body.innerHTML = `
    <dialog id="partialActionModal">
      <div id="partialActionProductName"></div>
      <input id="partialActionQuantity" type="number">
      <div id="partialActionQuantityError"></div>
      <small id="partialActionStock"></small>
      <select id="partialActionCause">
        <option value=""></option>
        <option value="spoiled"></option>
        <option value="overbought"></option>
      </select>
      <button id="btnPartialConsumeConfirm" type="button"><span class="btn-label">Consume</span></button>
      <button id="btnPartialWasteConfirm" type="button"><span class="btn-label">Waste</span></button>
      <button type="button" data-bs-dismiss="dialog"></button>
    </dialog>
    <dialog id="bulkWasteModal">
      <p id="bulkWasteCount"></p>
      <select id="bulkWasteCause">
        <option value=""></option>
        <option value="forgotten"></option>
      </select>
      <button id="btnBulkWasteConfirm" type="button"></button>
      <button type="button" data-bs-dismiss="dialog"></button>
    </dialog>
  `;
  return { ...exports, realm, actionCalls, bulkCalls, feedback };
}

describe("products.js waste cause", () => {
  it("sends the chosen cause with a waste", async () => {
    const { openPartialActionModal, submitPartialAction, realm, actionCalls } = setupWasteCause();
    await openPartialActionModal("7");
    realm.document.getElementById("partialActionCause").value = "spoiled";

    await submitPartialAction("waste");

    expect(actionCalls).toEqual([{ action: "waste", productID: 7, quantity: 1, cause: "spoiled" }]);
  });

  it("sends no cause with a consume", async () => {
    const { openPartialActionModal, submitPartialAction, realm, actionCalls } = setupWasteCause();
    await openPartialActionModal("7");
    realm.document.getElementById("partialActionCause").value = "spoiled";

    await submitPartialAction("consume");

    expect(actionCalls).toEqual([{ action: "consume", productID: 7, quantity: 1, cause: "" }]);
  });

  it("resets the cause when the dialog opens again", async () => {
    const { openPartialActionModal, realm } = setupWasteCause();
    await openPartialActionModal("7");
    realm.document.getElementById("partialActionCause").value = "spoiled";

    await openPartialActionModal("7");

    expect(realm.document.getElementById("partialActionCause").value).toBe("");
  });

  it("sends one cause for the whole bulk selection", async () => {
    const { openBulkWasteModal, submitBulkWaste, realm, bulkCalls } = setupWasteCause();
    openBulkWasteModal([3, 4]);
    realm.document.getElementById("bulkWasteCause").value = "forgotten";

    await submitBulkWaste();

    expect(bulkCalls).toEqual([
      { type: "delete", ids: [3, 4], options: { wasteCause: "forgotten" } },
    ]);
  });

  it("sends no cause when the bulk selection has none", async () => {
    const { openBulkWasteModal, submitBulkWaste, bulkCalls } = setupWasteCause();
    openBulkWasteModal([3]);

    await submitBulkWaste();

    expect(bulkCalls).toEqual([
      { type: "delete", ids: [3], options: { wasteCause: "" } },
    ]);
  });
});
