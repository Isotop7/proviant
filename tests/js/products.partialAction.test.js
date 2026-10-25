import { describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

const RESET_REALM = "localStorage.clear(); document.body.innerHTML = '';";

function setupPartialDialog({ fetchStub, actionResult } = {}) {
  const actionCalls = [];
  const feedback = [];
  const dialogs = [];
  const result = actionResult || { code: 200, message: "recorded" };
  const productFetch = fetchStub || (async () => ({
    ok: true,
    json: async () => [{ id: 7, productName: "Yogurt", quantity: 5 }],
  }));
  class Dialog {
    constructor(element) {
      this.element = element;
      dialogs.push(this);
    }
    show() {
      this.element.setAttribute("open", "");
    }
    hide() {
      this.element.removeAttribute("open");
    }
  }
  const recordAction = (action) => async (productID, quantity) => {
    actionCalls.push({ action, productID, quantity });
    return result;
  };
  const { exports, realm } = loadClassicScript(
    "products.js",
    ["openPartialActionModal", "submitPartialAction"],
    {
      prelude: RESET_REALM + "window.__reloads = 0; function clearBulkSelectionAndReload() { window.__reloads++; }",
      globals: {
        fetch: productFetch,
        bootstrap: { Dialog },
        proviant: {
          consumeProduct: recordAction("consume"),
          wasteProduct: recordAction("waste"),
          showFeedback: (type, title, message) => feedback.push({ type, title, message }),
        },
      },
    },
  );
  realm.document.body.innerHTML = `
    <button id="trigger" type="button">Open</button>
    <dialog id="partialActionModal">
      <div id="partialActionProductName"></div>
      <input id="partialActionQuantity" type="number">
      <div id="partialActionQuantityError"></div>
      <small id="partialActionStock"></small>
      <button id="btnPartialConsumeConfirm" type="button"><span class="btn-label">Consume</span></button>
      <button id="btnPartialWasteConfirm" type="button"><span class="btn-label">Waste</span></button>
      <button type="button" data-bs-dismiss="dialog"></button>
    </dialog>
  `;
  return { ...exports, realm, actionCalls, feedback, dialogs };
}

describe("products.js partial consume/waste dialog", () => {
  it("opens with fresh stock and a default of one pack", async () => {
    const { openPartialActionModal, realm, dialogs } = setupPartialDialog();
    const trigger = realm.document.getElementById("trigger");

    await openPartialActionModal("7", trigger);

    const input = realm.document.getElementById("partialActionQuantity");
    expect(input.max).toBe("5");
    expect(input.value).toBe("1");
    expect(realm.document.getElementById("partialActionProductName").textContent).toBe("Yogurt");
    expect(realm.document.getElementById("partialActionStock").textContent).toBe("5 packs available");
    expect(dialogs).toHaveLength(1);
    expect(realm.document.getElementById("partialActionModal").hasAttribute("open")).toBe(true);
    expect(trigger.disabled).toBe(false);
    expect(trigger.getAttribute("aria-busy")).toBeNull();
    expect(trigger.innerHTML).toBe("Open");
  });

  it("submits the entered quantity to consume and waste", async () => {
    for (const action of ["consume", "waste"]) {
      const { openPartialActionModal, submitPartialAction, realm, actionCalls } = setupPartialDialog();
      await openPartialActionModal("7");
      realm.document.getElementById("partialActionQuantity").value = "2";

      await submitPartialAction(action);

      expect(actionCalls).toEqual([{ action, productID: 7, quantity: 2 }]);
      expect(realm.document.getElementById("partialActionModal").hasAttribute("open")).toBe(false);
    }
  });

  it("rejects an invalid quantity inline without sending", async () => {
    const { openPartialActionModal, submitPartialAction, realm, actionCalls } = setupPartialDialog();
    await openPartialActionModal("7");
    const input = realm.document.getElementById("partialActionQuantity");
    input.value = "99";

    await submitPartialAction("consume");

    expect(actionCalls).toHaveLength(0);
    expect(input.classList.contains("is-invalid")).toBe(true);
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(realm.document.getElementById("partialActionQuantityError").textContent).toContain("1 to 5");
    expect(realm.document.getElementById("partialActionModal").hasAttribute("open")).toBe(true);
  });

  it("does not fall back to stale DOM stock when the fresh fetch fails", async () => {
    const { openPartialActionModal, realm, feedback, dialogs } = setupPartialDialog({
      fetchStub: async () => ({ ok: false }),
    });
    const staleQuantity = realm.document.createElement("span");
    staleQuantity.dataset.quantity = "5";
    realm.document.body.appendChild(staleQuantity);

    await openPartialActionModal("7");

    expect(dialogs).toHaveLength(0);
    expect(feedback).toHaveLength(1);
    expect(feedback[0].type).toBe("error");
    expect(realm.document.getElementById("partialActionModal").hasAttribute("open")).toBe(false);
  });

  it("reports a queued action without showing applied success or reloading", async () => {
    const { openPartialActionModal, submitPartialAction, realm, feedback } = setupPartialDialog({
      actionResult: { code: 202, queued: true, message: "queued" },
    });
    await openPartialActionModal("7");

    await submitPartialAction("consume");

    expect(feedback).toHaveLength(1);
    expect(feedback[0].type).toBe("info");
    expect(feedback[0].message).toContain("back online");
    expect(realm.__reloads).toBe(0);
    expect(realm.document.getElementById("partialActionModal").hasAttribute("open")).toBe(false);
  });

  it("reports the packs the server removed, not the ones requested", async () => {
    const { openPartialActionModal, submitPartialAction, realm, feedback } = setupPartialDialog({
      actionResult: { code: 200, message: "recorded", removed: 1 },
    });
    await openPartialActionModal("7");
    realm.document.getElementById("partialActionQuantity").value = "5";

    await submitPartialAction("consume");

    expect(feedback).toHaveLength(1);
    expect(feedback[0].message).toBe("1 pack consumed");
  });

  it("keeps the newer product when two are opened at once", async () => {
    const pending = [];
    const fetchStub = (url) =>
      new Promise((resolve) => {
        pending.push({ url, resolve });
      });
    const { openPartialActionModal, realm } = setupPartialDialog({ fetchStub });

    const first = openPartialActionModal("7");
    const second = openPartialActionModal("9");
    pending[1].resolve({
      ok: true,
      json: async () => [{ id: 9, productName: "Eggs", quantity: 4 }],
    });
    pending[0].resolve({
      ok: true,
      json: async () => [{ id: 7, productName: "Yogurt", quantity: 5 }],
    });
    await Promise.all([first, second]);

    expect(realm.document.getElementById("partialActionProductName").textContent).toBe("Eggs");
    expect(realm.document.getElementById("partialActionQuantity").max).toBe("4");
  });
});
