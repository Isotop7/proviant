import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * Buying a list item opens a dialog, fills the expiry from the shelf life the
 * household gave the item's category and posts the purchase. The dialog and
 * the row are reached by delegation, and a selector that no longer matches
 * fails silently: the dialog never opens and nothing is posted. The clicks
 * therefore have to be dispatched for real.
 *
 * One load only: every load adds another click listener to the shared
 * document, so a second load would fire the handler twice per click. The rows
 * of the three cases share the one load instead.
 */
const ITEMS = [
  { ID: 7, name: "Milk", category: "dairy products", categoryKey: "dairy products", quantity: 2, packSize: "1 l", checked: false },
  { ID: 8, name: "Bread", category: "bakery", categoryKey: "", quantity: 1, packSize: "", checked: false },
  { ID: 9, name: "Eggs", category: "dairy products", categoryKey: "dairy products", quantity: 1, packSize: "", checked: false },
  { ID: 10, name: "Olives", category: "olives", categoryKey: "", quantity: 1, packSize: "", checked: false },
  { ID: 11, name: "Oat milk", category: "dairy products", categoryKey: "dairy products", ProductID: 5, quantity: 3, packSize: "1 l", checked: false },
  { ID: 12, name: "Yoghurt", category: "dairy products", categoryKey: "dairy products", quantity: 1, packSize: "", checked: false },
  { ID: 13, name: "Milk", category: "dairy products", categoryKey: "dairy products", quantity: 1, packSize: "", checked: false },
];

const CATEGORIES = [
  { categoryKey: "dairy products", displayName: "Dairy Products", defaultShelfLifeDays: 7 },
  { categoryKey: "bakery", displayName: "Bakery" },
];

function row(item) {
  return `
    <div class="list-group-item shopping-list__item" data-id="${item.ID}" id="item-${item.ID}">
      <input type="checkbox" class="item-checkbox list-checkbox" data-item-id="${item.ID}">
      <div class="shopping-list__actions">
        <button type="button" class="btn-icon" data-buy-item="${item.ID}">
          <i class="bi bi-bag-check"></i>
        </button>
      </div>
    </div>`;
}

const PAGE_MARKUP = `
<div id="shoppingListContainer" class="card shopping-list">
  <div class="list-group list-group-flush">
    ${ITEMS.map(row).join("")}
  </div>
</div>
<dialog class="dialog" id="purchaseItemDialog">
  <div class="dialog-header"><div class="dialog-subtitle" id="purchaseItemName"></div></div>
  <form id="purchaseForm">
    <div class="dialog-body">
      <input type="number" id="purchaseQuantity" value="1">
      <p id="purchaseExistingProduct" class="d-none"></p>
      <div data-purchase-field="expiry">
        <input type="date" id="purchaseExpiry">
        <small id="purchaseExpiryHint" class="d-none"></small>
      </div>
      <div data-purchase-field="category">
        <select id="purchaseCategory"></select>
      </div>
      <div data-purchase-field="packSize">
        <input type="text" id="purchasePackSize">
      </div>
      <div data-purchase-field="location">
        <select id="purchaseStorageLocation">
          <option value="">None</option>
          <option value="3">Fridge</option>
        </select>
      </div>
      <div data-purchase-field="barcode">
        <input type="text" id="purchaseBarcode">
      </div>
    </div>
    <div class="dialog-footer">
      <button type="submit" id="btnPurchase">
        <span class="btn-label">Add to pantry</span>
        <span class="btn-busy">Working</span>
      </button>
    </div>
  </form>
</dialog>
<div id="purchaseAnnouncement" class="visually-hidden" aria-live="polite" aria-atomic="true"></div>
<dialog class="dialog" id="duplicateProductModal">
  <div class="dialog-header">
    <h5 class="dialog-title mb-0" id="duplicateProductModalLabel">This product is already in your pantry</h5>
    <div class="dialog-subtitle" id="duplicateProductName"></div>
  </div>
  <div class="dialog-body">
    <p id="duplicateProductExpiry"></p>
    <p id="duplicateProductLocation"></p>
    <div id="duplicateProductAnnouncement" class="visually-hidden" aria-live="polite"></div>
  </div>
  <div class="dialog-footer">
    <button type="button" class="btn btn-outline theme-secondary" id="btnDuplicateCancel">Cancel</button>
    <button type="button" class="btn btn-outline theme-secondary" id="btnDuplicateAddSeparately">Add separately</button>
    <button type="button" class="btn btn-solid theme-primary" id="btnDuplicateAddPack">Add a pack</button>
  </div>
</dialog>
<script type="application/json" id="shoppingListData">${JSON.stringify(ITEMS)}</script>
<script type="application/json" id="shoppingListCategories">${JSON.stringify(CATEGORIES)}</script>
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

function expectedDate(days) {
  const date = new Date();
  date.setDate(date.getDate() + days);
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}

describe("shoppingList.js buy action", () => {
  it("fills the expiry from the category default and posts the purchase", async () => {
    let resolveInFlight;
    const inFlight = new Promise((resolve) => {
      resolveInFlight = resolve;
    });
    const seen = {};
    const fetchStub = vi.fn((url) => {
      const key = String(url);
      seen[key] = (seen[key] || 0) + 1;
      if (key.includes("/shopping-list/9/purchase")) {
        return Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({ message: "boom" }) });
      }
      if (key.includes("/shopping-list/13/purchase")) {
        if (seen[key] === 1) {
          return Promise.resolve({
            ok: false,
            status: 409,
            json: () =>
              Promise.resolve({
                message: "a matching product already exists",
                action: "Add a pack to the existing product, create a separate one, or cancel",
                duplicate: { id: 99, productName: "Milk", expireAt: "2030-01-02T00:00:00Z" },
              }),
          });
        }
        return Promise.resolve({
          ok: true,
          status: 201,
          json: () =>
            Promise.resolve({
              item: { id: 13, checked: true, productId: 99 },
              product: { id: 99, productName: "Milk", quantity: 1 },
              created: false,
            }),
        });
      }
      if (key.includes("/shopping-list/12/purchase")) {
        return inFlight.then(() => ({
          ok: true,
          status: 201,
          json: () =>
            Promise.resolve({
              item: { id: 12, checked: true, productId: 43 },
              product: { id: 43, productName: "Yoghurt", quantity: 1 },
              created: true,
            }),
        }));
      }
      return Promise.resolve({
        ok: true,
        status: 201,
        json: () =>
          Promise.resolve({
            item: { id: 7, checked: true, productId: 42 },
            product: { id: 42, productName: "Milk", quantity: 3 },
            created: true,
          }),
      });
    });

    // The announcement of the prompt must be filled after the dialog is open:
    // a closed dialog is hidden, and a change to a hidden live region
    // announces nothing.
    let announcementWhenOpened = null;
    class Dialog {
      constructor(element) {
        this.element = element;
      }
      show() {
        this.element.setAttribute("open", "");
        if (this.element.id === "duplicateProductModal") {
          const region = this.element.querySelector("#duplicateProductAnnouncement");
          announcementWhenOpened = region ? region.textContent : null;
        }
      }
      hide() {
        this.element.removeAttribute("open");
      }
      static getOrCreateInstance(element) {
        return new Dialog(element);
      }
      static getInstance(element) {
        return new Dialog(element);
      }
    }

    loadClassicScript("proviant.js", ["proviant"], {
      prelude: "localStorage.clear(); document.body.innerHTML = '';",
      globals: { fetch: fetchStub },
    });

    loadClassicScript("productsDuplicate.js", [], {
      prelude: "localStorage.clear(); document.body.innerHTML = '';",
      globals: { fetch: fetchStub, bootstrap: { Dialog } },
    });

    const { realm } = loadClassicScript("shoppingList.js", [], {
      prelude: `localStorage.clear(); document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
      globals: { fetch: fetchStub, bootstrap: { Dialog } },
    });
    const doc = realm.document;

    doc.dispatchEvent(new realm.Event("DOMContentLoaded"));

    // The buy button opens the dialog and fills it from the item.
    doc.querySelector('[data-buy-item="7"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("purchaseItemDialog").hasAttribute("open")).toBe(true);
    expect(doc.getElementById("purchaseItemName").textContent).toBe("Milk");
    expect(doc.getElementById("purchaseQuantity").value).toBe("2");
    expect(doc.getElementById("purchasePackSize").value).toBe("1 l");
    expect(doc.getElementById("purchaseCategory").value).toBe("dairy products");
    expect(doc.getElementById("purchaseExpiry").value).toBe(expectedDate(7));
    expect(doc.getElementById("purchaseExpiryHint").classList.contains("d-none")).toBe(false);

    // The purchase carries what the user confirmed.
    doc.getElementById("purchaseQuantity").value = "3";
    doc.getElementById("purchaseExpiry").value = "2030-01-02";
    doc.getElementById("purchaseStorageLocation").value = "3";
    doc.getElementById("purchaseForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();
    await flush();

    expect(fetchStub).toHaveBeenCalledTimes(1);
    const [url, options] = fetchStub.mock.calls[0];
    expect(String(url)).toContain("/api/v1/shopping-list/7/purchase");
    expect(options.method).toBe("POST");
    expect(JSON.parse(options.body)).toEqual({
      quantity: 3,
      expireAt: "2030-01-02",
      categoryKey: "dairy products",
      packSize: "1 l",
      barcode: "",
      storageLocationId: 3,
    });

    // The bought item is checked off and announced.
    expect(doc.getElementById("item-7").classList.contains("checked")).toBe(true);
    expect(doc.getElementById("purchaseAnnouncement").textContent).toContain("Milk");
    expect(doc.getElementById("purchaseItemDialog").hasAttribute("open")).toBe(false);

    // A category without a shelf life leaves the expiry empty and asks for it.
    doc.querySelector('[data-buy-item="8"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("purchaseExpiry").value).toBe("");
    expect(doc.getElementById("purchaseExpiryHint").classList.contains("d-none")).toBe(true);
    expect(doc.activeElement).toBe(doc.getElementById("purchaseExpiry"));

    // A purchase that fails leaves the item unchecked.
    doc.querySelector('[data-buy-item="9"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    doc.getElementById("purchaseForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();
    await flush();

    expect(fetchStub).toHaveBeenCalledTimes(2);
    expect(doc.getElementById("item-9").classList.contains("checked")).toBe(false);

    // An item whose category names no canonical category keeps its own: the
    // purchase carries no category, so the server does not clear the item's.
    doc.querySelector('[data-buy-item="10"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("purchaseCategory").value).toBe("");
    doc.getElementById("purchaseForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();
    await flush();

    expect(fetchStub).toHaveBeenCalledTimes(3);
    expect(JSON.parse(fetchStub.mock.calls[2][1].body)).not.toHaveProperty("categoryKey");

    // A product the household already tracks keeps its own date, category,
    // pack size and location: the fields that would be ignored are hidden.
    doc.querySelector('[data-buy-item="11"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("purchaseExistingProduct").classList.contains("d-none")).toBe(false);
    expect(doc.getElementById("purchaseExistingProduct").textContent).toContain("Oat milk");
    doc.querySelectorAll("[data-purchase-field]").forEach((field) => {
      expect(field.classList.contains("d-none")).toBe(true);
    });

    // A free-text item shows them again.
    doc.querySelector('[data-buy-item="7"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("purchaseExistingProduct").classList.contains("d-none")).toBe(true);
    doc.querySelectorAll("[data-purchase-field]").forEach((field) => {
      expect(field.classList.contains("d-none")).toBe(false);
    });

    // A purchase that is still in flight belongs to the item it was made
    // for: it must not check another item's row nor close its dialog.
    doc.querySelector('[data-buy-item="12"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    doc.getElementById("purchaseForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    doc.querySelector('[data-buy-item="11"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    resolveInFlight();
    await flush();
    await flush();

    expect(doc.getElementById("item-12").classList.contains("checked")).toBe(true);
    expect(doc.getElementById("item-11").classList.contains("checked")).toBe(false);
    expect(doc.getElementById("purchaseItemDialog").hasAttribute("open")).toBe(true);
    expect(doc.getElementById("purchaseItemName").textContent).toBe("Oat milk");

    // A bought product that matches one the household already has opens the
    // same-item prompt: the packs belong to that product, or they are a
    // product of their own.
    doc.querySelector('[data-buy-item="13"]').dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    doc.getElementById("purchaseForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();
    await flush();

    expect(doc.getElementById("duplicateProductModal").hasAttribute("open")).toBe(true);
    expect(doc.getElementById("duplicateProductName").textContent).toBe("Milk");
    expect(announcementWhenOpened).toBe("");
    expect(doc.getElementById("duplicateProductAnnouncement").textContent).toContain("Milk");

    doc.getElementById("btnDuplicateAddPack").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    const retry = fetchStub.mock.calls[fetchStub.mock.calls.length - 1];
    expect(JSON.parse(retry[1].body).addToProductId).toBe(99);
    expect(doc.getElementById("item-13").classList.contains("checked")).toBe(true);
  });
});
