import { describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * A product's category may be a category the household hides. The edit form
 * shows the category the product carries, hidden or not, so the form starts
 * with the product's own category. An unchanged category is never sent: the
 * empty selection would mean "no category" and would clear it behind the
 * user's back.
 *
 * One load per file: every load adds another listener to the shared document.
 */
const RESET_REALM = "localStorage.clear(); document.body.innerHTML = '';";

const FORM_MARKUP = `
  <form id="editProductForm">
    <span id="labelProductID">42</span>
    <input type="text" id="inputProductName" value="Milk">
    <input type="hidden" id="categoriesHidden" value="en:milk">
    <input type="hidden" id="countriesHidden" value="">
    <input type="text" id="inputImageURL" value="">
    <input type="date" id="inputExpireAt" value="2030-01-02">
    <input type="number" id="inputQuantity" value="1">
    <select id="productCategory" data-selected-category-key="milk">CATEGORY_OPTIONS</select>
    <small id="expirySuggestion" class="d-none"></small>
  </form>
`;

// The category the product carries is listed, hidden or not: the form shows
// what the product actually is.
const CATEGORY_OPTIONS = `
      <option value="">No category</option>
      <option value="milk" selected>Milk</option>
      <option value="cheese" data-shelf-life-days="5">Cheese</option>
`;

// A form that does not list the category the product carries: the empty
// selection then means "the list does not show it", not "no category".
const CATEGORY_OPTIONS_WITHOUT_THE_CATEGORY = `
      <option value="">No category</option>
      <option value="cheese" data-shelf-life-days="5">Cheese</option>
`;

function freshEditScript(categoryOptions) {
  const { exports: proviant } = loadClassicScript("proviant.js", ["proviant"], {
    prelude: RESET_REALM,
    globals: { fetch: async () => ({ status: 200, json: async () => ({}) }) },
  });

  const sent = [];
  proviant.editProduct = async (product) => {
    sent.push(product);
    return { code: 200, message: "saved" };
  };
  proviant.showFeedback = () => {};

  const markup = FORM_MARKUP.replace("CATEGORY_OPTIONS", categoryOptions);
  const { exports: captured, realm } = loadClassicScript("productsEdit.js", ["editProduct"], {
    prelude:
      `${RESET_REALM}\ndocument.body.innerHTML = \`${markup}\`;\n` +
      // jsdom implements no innerText, and the form reads the product id from
      // it. The shim mirrors what a browser returns.
      `Object.defineProperty(document.getElementById('labelProductID'), 'innerText', ` +
      `{ get: function () { return this.textContent; } });`,
    globals: { proviant },
  });

  return { ...captured, realm, sent };
}

describe("productsEdit.js category handling", () => {
  it("does not send an unchanged category", () => {
    const { editProduct, sent } = freshEditScript(CATEGORY_OPTIONS);

    editProduct();

    expect(sent).toHaveLength(1);
    expect(sent[0].categoryKey).toBeUndefined();
  });

  it("does not clear a category the list does not show", () => {
    const { editProduct, sent } = freshEditScript(CATEGORY_OPTIONS_WITHOUT_THE_CATEGORY);

    editProduct();

    expect(sent).toHaveLength(1);
    expect(sent[0].categoryKey).toBeUndefined();
  });

  it("sends a category the user picked", () => {
    const { editProduct, realm, sent } = freshEditScript(CATEGORY_OPTIONS);

    const select = realm.document.getElementById("productCategory");
    select.value = "cheese";
    editProduct();

    expect(sent).toHaveLength(1);
    expect(sent[0].categoryKey).toBe("cheese");
  });

  it("sends the empty selection when the user clears a listed category", () => {
    const { editProduct, realm, sent } = freshEditScript(CATEGORY_OPTIONS);

    const select = realm.document.getElementById("productCategory");
    select.value = "";
    editProduct();

    expect(sent).toHaveLength(1);
    expect(sent[0].categoryKey).toBe("");
  });
});
