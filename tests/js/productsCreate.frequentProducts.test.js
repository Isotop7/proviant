import { beforeAll, describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * The "frequently bought" strip fills the add-product form with what the
 * household already knows about a product. The tap is the whole feature, and
 * it is delegation: a selector that no longer matches fails silently — the
 * form stays empty and nothing throws. The click therefore has to be
 * dispatched for real.
 *
 * One load only: every load adds another click listener to the shared
 * document, so a second load would fire the handler twice per click. The
 * markup is therefore set once and the fields are reset per test, not the
 * markup — the script captured its elements when it loaded.
 */
const PAGE_MARKUP = `
  <dialog id="addProductModal">
    <div id="modalStepScan">
      <input id="barcode" type="text" pattern="[0-9]{13}" value="">
      <div id="frequentProducts" class="frequent-products d-none">
        <div id="frequentProductsList"></div>
      </div>
    </div>
    <div id="productData" class="d-none">
      <img id="productInfoImage" alt="">
      <div id="productInfoName"></div>
      <div id="productInfoGenericName"></div>
      <small id="storageHint" class="d-none"></small>
      <input id="productName" type="text" value="">
      <input id="packSize" type="text" value="">
      <select id="productCategory">
        <option value="">No category</option>
        <option value="dairy products" data-shelf-life-days="5">Dairy products</option>
      </select>
      <input id="expireAt" type="date" value="">
      <small id="expirySuggestion" class="d-none"></small>
      <select id="modalStorageLocation">
        <option value="">No location</option>
        <option value="3">Fridge</option>
      </select>
      <input id="quantity" type="hidden" value="1">
      <select id="instanceDropdown"></select>
      <button id="btnAddProduct" type="button"></button>
      <button id="btnShowProduct" type="button"></button>
      <button id="btnDeleteProductModal" type="button"></button>
      <button id="btnArchiveProductModal" type="button"></button>
      <button id="btnShowProducts" type="button"></button>
    </div>
  </dialog>
`;

const STAPLES = [
  {
    barcode: "4006381333931",
    productName: "Milk",
    packSize: "1 l",
    categoryKey: "dairy products",
    storageLocationId: 3,
    imageUrl: "/product-images/4006381333931.jpg",
    purchaseCount: 12,
  },
  {
    barcode: "40063813339312",
    productName: "Imported Yoghurt",
    packSize: "500 g",
    purchaseCount: 5,
  },
];

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

let realm;
let fetchBehaviour;

beforeAll(() => {
  const fetchStub = vi.fn(() => fetchBehaviour());

  loadClassicScript("proviant.js", [], {
    prelude: `localStorage.clear(); document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
    globals: { fetch: fetchStub },
  });

  const loaded = loadClassicScript("productsCreate.js", [], {
    prelude: `localStorage.clear(); document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
    globals: {
      fetch: fetchStub,
      proviantLimits: { maxProductNameLength: 191, maxPackSizeLength: 60 },
      Html5Qrcode: class {
        start() {
          return Promise.resolve();
        }
        stop() {
          return Promise.resolve();
        }
        clear() {}
        getState() {
          return 0;
        }
      },
      Html5QrcodeSupportedFormats: { EAN_13: 13 },
      Html5QrcodeScannerState: { SCANNING: 2 },
    },
  });
  realm = loaded.realm;
});

async function openDialog() {
  realm.document
    .getElementById("addProductModal")
    .dispatchEvent(new realm.Event("shown.bs.dialog"));
  await flush();
  await flush();
  await flush();
}

function resetForm() {
  const doc = realm.document;
  doc.getElementById("barcode").value = "";
  doc.getElementById("productName").value = "";
  doc.getElementById("packSize").value = "";
  doc.getElementById("expireAt").value = "";
  delete doc.getElementById("expireAt").dataset.userEdited;
  delete doc.getElementById("expireAt").dataset.suggested;
  doc.getElementById("productCategory").value = "";
  doc.getElementById("modalStorageLocation").value = "";
}

// A date the user typed is theirs: the household's shelf life may not take it.
function userSetsExpiry(value) {
  const expireAt = realm.document.getElementById("expireAt");
  expireAt.value = value;
  expireAt.dispatchEvent(new realm.Event("input", { bubbles: true }));
}

describe("productsCreate.js frequently bought strip", () => {
  it("fills the form with what the household knows", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve(STAPLES),
      });
    resetForm();

    await openDialog();

    const doc = realm.document;
    const chips = doc.querySelectorAll(".frequent-product");
    expect(chips.length).toBe(1);
    expect(chips[0].textContent).toBe("Milk");
    expect(doc.getElementById("frequentProducts").classList.contains("d-none")).toBe(false);

    chips[0].dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("barcode").value).toBe("4006381333931");
    expect(doc.getElementById("productName").value).toBe("Milk");
    expect(doc.getElementById("packSize").value).toBe("1 l");
    expect(doc.getElementById("productCategory").value).toBe("dairy products");
    expect(doc.getElementById("modalStorageLocation").value).toBe("3");
  });

  it("names the shelf life of the category without touching the date", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve(STAPLES),
      });
    resetForm();

    await openDialog();

    const doc = realm.document;
    doc.querySelector(".frequent-product").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    // The suggestion carries no date: the shelf life the household gave the
    // category is named, and the user sets the date.
    expect(doc.getElementById("expireAt").value).toBe("");
    const hint = doc.getElementById("expirySuggestion");
    expect(hint.classList.contains("d-none")).toBe(false);
    expect(hint.textContent).toContain("Dairy products");
    expect(hint.textContent).toContain("5");
  });

  it("leaves a date the user set alone", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve(STAPLES),
      });
    resetForm();

    await openDialog();

    const doc = realm.document;
    userSetsExpiry("2030-01-02");

    doc.querySelector(".frequent-product").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("expireAt").value).toBe("2030-01-02");
    // The date is the user's: the shelf life is not needed to explain it.
    expect(doc.getElementById("expirySuggestion").classList.contains("d-none")).toBe(true);
  });

  it("shows the picture the household's product had", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve(STAPLES),
      });
    resetForm();

    await openDialog();

    const doc = realm.document;
    doc.querySelector(".frequent-product").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    const image = doc.getElementById("productInfoImage");
    expect(image.getAttribute("src")).toBe("/product-images/4006381333931.jpg");
    expect(image.classList.contains("d-none")).toBe(false);
  });

  it("asks for no picture at all when the product has none", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve([{ ...STAPLES[1], barcode: "4006381333931" }]),
      });
    resetForm();

    await openDialog();

    const doc = realm.document;
    doc.querySelector(".frequent-product").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    // An empty src queues an error event that outlives the picture that
    // follows it, so the attribute is removed instead.
    const image = doc.getElementById("productInfoImage");
    expect(image.hasAttribute("src")).toBe(false);
    expect(image.classList.contains("d-none")).toBe(true);
  });

  it("keeps the name within the bytes the column holds", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve(STAPLES),
      });
    resetForm();

    await openDialog();

    const doc = realm.document;
    const name = doc.getElementById("productName");
    // The column counts bytes, so a name of multi-byte characters is cut
    // before the character count the field promises.
    name.value = "ä".repeat(200);
    name.dispatchEvent(new realm.Event("input", { bubbles: true }));

    expect(new TextEncoder().encode(name.value).length).toBeLessThanOrEqual(191);
    expect(name.value.length).toBeLessThan(200);
  });

  it("offers only the barcodes the barcode field accepts", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve(STAPLES),
      });
    resetForm();

    await openDialog();

    const doc = realm.document;
    const chips = doc.querySelectorAll(".frequent-product");
    // The imported yoghurt carries a 14-digit barcode the field rejects: a
    // chip for it would fill a field that blocks the submit.
    expect(chips.length).toBe(1);
    expect(chips[0].dataset.barcode).toBe("4006381333931");
  });

  it("leaves no trace of the previous product in the banner", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve(STAPLES),
      });
    resetForm();

    const doc = realm.document;
    doc.getElementById("productInfoGenericName").textContent = "en:eggs, en:breakfast";
    doc.getElementById("storageHint").textContent = "Fridge door";
    doc.getElementById("storageHint").classList.remove("d-none");

    await openDialog();

    doc.querySelector(".frequent-product").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("productInfoName").textContent).toBe("Milk");
    expect(doc.getElementById("productInfoGenericName").textContent).toBe("");
    expect(doc.getElementById("storageHint").textContent).toBe("");
    expect(doc.getElementById("storageHint").classList.contains("d-none")).toBe(true);
  });

  it("keeps the chips buttons", async () => {
    fetchBehaviour = () =>
      Promise.resolve({
        status: 200,
        json: () => Promise.resolve(STAPLES),
      });
    resetForm();

    await openDialog();

    const chip = realm.document.querySelector(".frequent-product");
    // A listitem role would take the button role with it.
    expect(chip.getAttribute("role")).toBeNull();
    expect(chip.tagName).toBe("BUTTON");
  });

  it("stays empty when the server is not there and leaves the form working", async () => {
    fetchBehaviour = () => Promise.reject(new Error("offline"));
    resetForm();

    await openDialog();

    const doc = realm.document;
    expect(doc.querySelectorAll(".frequent-product").length).toBe(0);
    expect(doc.getElementById("frequentProducts").classList.contains("d-none")).toBe(true);

    // The form is untouched by the failure: the user can still type.
    doc.getElementById("barcode").value = "4006381333931";
    doc.getElementById("productName").value = "Milk";
    expect(doc.getElementById("productName").value).toBe("Milk");
  });
});
