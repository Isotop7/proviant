import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

// The duplicate prompt is reached by clicking "add" on a product the household
// already has, and left by clicking one of the three choices. Both hops are
// delegation, and a selector that no longer matches fails silently: the
// dialog never opens, the choice never fires, and nothing throws. The clicks
// therefore have to be dispatched for real.
//
// One load per file: every load adds another click listener to the shared
// document, so a second load of the same file would fire the handler twice
// per click. The two files handle different selectors, so they can share the
// document.
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
  it("offers the three choices and acts on the one the user picks", async () => {
    const creates = [];
    const quantityUpdates = [];
    let conflictWithoutDuplicate = false;

    const fetchStub = vi.fn().mockImplementation(() =>
      Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 42 }),
      }),
    );

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

    // Both loads share these functions: the loader copies the mock onto the
    // realm's window, so the second load would otherwise replace the first
    // one's proviant object.
    const proviantMock = {
      createProduct: async (...args) => {
        creates.push(args);
        // The server refuses the first attempt as a duplicate and writes the
        // row when the user asks for a separate product.
        if (args[6]) return { code: 201, message: "created" };
        if (conflictWithoutDuplicate) {
          return { code: 409, message: "clientSyncId is already used by another product" };
        }
        return {
          code: 409,
          message: "a matching product already exists",
          duplicate: DUPLICATE,
          clientSyncId: "sync-42",
        };
      },
      updateProductQuantity: async (productID, delta) => {
        quantityUpdates.push({ productID, delta });
        return { code: 200, message: "recorded" };
      },
      showFeedback: () => {},
    };

    const globals = {
      fetch: fetchStub,
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
    const addButton = doc.getElementById("btnAddProduct");

    // The refusal opens the prompt, and the live region announces the three
    // choices.
    addButton.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(doc.getElementById("duplicateProductModal").hasAttribute("open")).toBe(true);
    expect(doc.getElementById("duplicateProductName").textContent).toBe("Yogurt");
    const announcement = doc.getElementById("duplicateProductAnnouncement").textContent;
    expect(announcement).toContain("Add a pack to the existing product");
    expect(announcement).toContain("Create a separate product");
    expect(announcement).toContain("Cancel and add nothing");
    expect(creates).toHaveLength(1);

    // "Add a pack" grows the matched row and writes nothing else.
    doc.getElementById("btnDuplicateAddPack").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(quantityUpdates).toEqual([{ productID: 42, delta: 3 }]);
    expect(creates).toHaveLength(1);
    expect(doc.getElementById("duplicateProductModal").hasAttribute("open")).toBe(false);

    // "Add separately" asks for a row of its own, with the same sync id.
    addButton.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    doc.getElementById("btnDuplicateAddSeparately").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    // The separate product is the third create: the two refusals and the
    // attempt the user asked for.
    expect(creates).toHaveLength(3);
    expect(creates[2][5]).toBe("sync-42");
    expect(creates[2][6]).toBe(true);
    expect(quantityUpdates).toHaveLength(1);

    // "Cancel" writes nothing.
    addButton.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    doc.getElementById("btnDuplicateCancel").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(creates).toHaveLength(4);
    expect(quantityUpdates).toHaveLength(1);
    expect(doc.getElementById("duplicateProductModal").hasAttribute("open")).toBe(false);

    // A conflict that names no duplicate has nothing to offer, so no prompt
    // opens.
    conflictWithoutDuplicate = true;
    addButton.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(creates).toHaveLength(5);
    expect(doc.getElementById("duplicateProductModal").hasAttribute("open")).toBe(false);
  });
});
