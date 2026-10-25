import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

// Offline, the separate product is queued and the create answers with 202.
// The row is written when the outbox drains, so the user is told it is queued
// — not that it failed.
//
// One load per file: every load adds another click listener to the shared
// document, so a second load of the same file would fire the handler twice
// per click. The file that walks the three choices therefore lives next to
// this one.

const PAGE_MARKUP = `
  <input type="text" id="barcode" pattern="[0-9]{13}" value="4006381333931">
  <input type="date" id="expireAt" value="2026-10-29">
  <input type="number" id="quantity" value="3">
  <select id="modalStorageLocation"><option value="4" selected>Fridge</option></select>
  <button type="button" id="btnAddProduct">Add</button>
  <dialog id="duplicateProductModal">
    <div id="duplicateProductName"></div>
    <div id="duplicateProductExpiry"></div>
    <div id="duplicateProductLocation"></div>
    <div id="duplicateProductAnnouncement" aria-live="polite"></div>
    <button type="button" id="btnDuplicateAddPack"></button>
    <button type="button" id="btnDuplicateAddSeparately"></button>
    <button type="button" id="btnDuplicateCancel"></button>
  </dialog>
`;

const DUPLICATE = {
  id: 42,
  barcode: "4006381333931",
  productName: "Yogurt",
  expireAt: "2026-10-29T00:00:00Z",
  quantity: 2,
  storageLocation: { id: 4, name: "Fridge" },
};

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("duplicate prompt", () => {
  it("treats a queued separate product as a success", async () => {
    const creates = [];
    const feedbacks = [];

    const proviantMock = {
      createProduct: async (...args) => {
        creates.push(args);
        // The server refuses the first attempt as a duplicate; the separate
        // product is queued while the browser is offline.
        if (args[6]) return { code: 202, message: "queued" };
        return {
          code: 409,
          message: "a matching product already exists",
          duplicate: DUPLICATE,
          clientSyncId: "sync-42",
        };
      },
      updateProductQuantity: async () => ({ code: 200, message: "recorded" }),
      showFeedback: (type, title, message) => {
        feedbacks.push({ type, title, message });
      },
    };

    const instances = new WeakMap();
    class Dialog {
      constructor(element) {
        this.element = element;
        instances.set(element, this);
      }
      show() {
        this.element.setAttribute("open", "");
      }
      hide() {
        this.element.removeAttribute("open");
      }
      static getInstance(element) {
        return instances.get(element) || null;
      }
      static getOrCreateInstance(element) {
        return instances.get(element) || new Dialog(element);
      }
    }

    const globals = {
      fetch: vi.fn(),
      bootstrap: { Dialog },
      proviant: proviantMock,
      Html5Qrcode: class {
        constructor() {}
        getState() {
          return "SCANNING";
        }
      },
      Html5QrcodeSupportedFormats: { EAN_13: 13 },
    };

    loadClassicScript("productsCreate.js", [], {
      prelude: `localStorage.clear(); document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
      globals,
    });

    const { realm } = loadClassicScript("productsDuplicate.js", [], { globals });
    const doc = realm.document;

    doc.getElementById("btnAddProduct").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    doc.getElementById("btnDuplicateAddSeparately").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(creates).toHaveLength(2);
    expect(feedbacks).toHaveLength(1);
    expect(feedbacks[0].type).toBe("success");
    expect(feedbacks[0].message).toBe("Saved — it will sync automatically.");
  });
});
