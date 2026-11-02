import { beforeEach, describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * The expiry date the household's category suggests is only a suggestion: the
 * date the user set is theirs and is never overwritten. The rule is the
 * server's — among all categories whose key the free text names, the longest
 * key wins — so the form and the server name the same category.
 */
const RESET_REALM = "localStorage.clear(); document.body.innerHTML = '';";

const FORM_MARKUP = `
  <select id="productCategory">
    <option value="">No category</option>
    <option value="dairy products" data-shelf-life-days="5">Dairy Products</option>
    <option value="milk" data-shelf-life-days="3">Milk</option>
    <option value="cheese">Cheese</option>
  </select>
  <input type="date" id="expireAt" value="">
  <small id="expirySuggestion" class="d-none"></small>
`;

function expectedDate(days) {
  const date = new Date();
  date.setDate(date.getDate() + days);
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}

function freshCreateScript() {
  loadClassicScript("proviant.js", ["proviant"], {
    prelude: RESET_REALM,
    globals: { fetch: async () => ({ status: 200, json: async () => ({}) }) },
  });

  const { exports: captured, realm } = loadClassicScript(
    "productsCreate.js",
    ["selectResolvedCategory", "applyCategorySuggestion"],
    {
      prelude: `${RESET_REALM}\ndocument.body.innerHTML = \`${FORM_MARKUP}\`;`,
      globals: {
        Html5Qrcode: class {
          start() { return Promise.resolve(); }
          stop() { return Promise.resolve(); }
          clear() {}
          getState() { return 0; }
        },
        Html5QrcodeSupportedFormats: { EAN_13: 13 },
        Html5QrcodeScannerState: { SCANNING: 2 },
      },
    },
  );

  return { ...captured, realm };
}

describe("productsCreate.js category suggestion", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  it("fills the expiry date with the shelf life of the resolved category", () => {
    const { selectResolvedCategory, realm } = freshCreateScript();

    selectResolvedCategory("en:dairy products,en:milk");

    const select = realm.document.getElementById("productCategory");
    const expireAt = realm.document.getElementById("expireAt");
    expect(select.value).toBe("dairy products");
    expect(expireAt.value).toBe(expectedDate(5));
  });

  it("names the source of the filled date", () => {
    const { selectResolvedCategory, realm } = freshCreateScript();

    selectResolvedCategory("milk");

    const hint = realm.document.getElementById("expirySuggestion");
    expect(hint.classList.contains("d-none")).toBe(false);
    expect(hint.textContent).toContain("Milk");
  });

  it("leaves the date alone when the category has no shelf life", () => {
    const { selectResolvedCategory, realm } = freshCreateScript();
    const expireAt = realm.document.getElementById("expireAt");

    selectResolvedCategory("cheese");

    expect(realm.document.getElementById("productCategory").value).toBe("cheese");
    expect(expireAt.value).toBe("");
  });

  it("leaves the date alone for a category that names no canonical category", () => {
    const { selectResolvedCategory, realm } = freshCreateScript();

    selectResolvedCategory("en:snacks");

    expect(realm.document.getElementById("productCategory").value).toBe("");
    expect(realm.document.getElementById("expireAt").value).toBe("");
  });

  it("never overwrites a date the user set", () => {
    const { selectResolvedCategory, applyCategorySuggestion, realm } = freshCreateScript();
    const expireAt = realm.document.getElementById("expireAt");

    selectResolvedCategory("milk");
    const suggested = expireAt.value;
    expect(suggested).toBe(expectedDate(3));

    expireAt.value = "2030-01-02";
    expireAt.dispatchEvent(new realm.Event("input", { bubbles: true }));

    applyCategorySuggestion();

    expect(expireAt.value).toBe("2030-01-02");
  });

  it("hides the hint when the date is the user's own", () => {
    const { selectResolvedCategory, applyCategorySuggestion, realm } = freshCreateScript();
    const expireAt = realm.document.getElementById("expireAt");
    const hint = realm.document.getElementById("expirySuggestion");

    selectResolvedCategory("milk");
    expect(hint.classList.contains("d-none")).toBe(false);

    expireAt.value = "2030-01-02";
    expireAt.dispatchEvent(new realm.Event("input", { bubbles: true }));

    applyCategorySuggestion();

    expect(hint.classList.contains("d-none")).toBe(true);
  });
});
