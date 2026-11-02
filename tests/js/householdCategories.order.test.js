import { describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * The category order is written to the server, and the list on the page is
 * moved before the write answers. A write that fails must put the row back:
 * the list on the page has to show the order the database holds.
 *
 * One load per file: every load adds another click listener to the shared
 * document, so a second load would fire the handler twice per click.
 */
const PAGE_MARKUP = `
  <ul id="householdCategoryList">
    <li class="list-group-item" data-category-key="dairy products">
      <input class="cat-name" value="Dairy Products">
      <input class="cat-shelf-life" value="">
      <input class="cat-days-after-opening" value="">
      <input type="checkbox" class="cat-visible" checked>
      <button type="button" class="btn-cat-save" data-category-key="dairy products">Save</button>
      <button type="button" class="btn-cat-up">Up</button>
      <button type="button" class="btn-cat-down">Down</button>
    </li>
    <li class="list-group-item" data-category-key="milk">
      <input class="cat-name" value="Milk">
      <input class="cat-shelf-life" value="">
      <input class="cat-days-after-opening" value="">
      <input type="checkbox" class="cat-visible" checked>
      <button type="button" class="btn-cat-save" data-category-key="milk">Save</button>
      <button type="button" class="btn-cat-up">Up</button>
      <button type="button" class="btn-cat-down">Down</button>
    </li>
  </ul>
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

function categoryKeys(realm) {
  return Array.from(realm.document.querySelectorAll("#householdCategoryList li[data-category-key]")).map(
    (row) => row.dataset.categoryKey,
  );
}

function freshCategoriesScript(fetchStub) {
  const { exports: proviant } = loadClassicScript("proviant.js", ["proviant"], {
    globals: { fetch: fetchStub },
  });
  proviant.showFeedback = () => {};

  const { realm } = loadClassicScript("householdCategories.js", [], {
    prelude: `localStorage.clear(); document.body.innerHTML = \`${PAGE_MARKUP}\`;`,
    globals: { proviant },
  });

  return { realm };
}

describe("householdCategories.js order", () => {
  it("keeps the order the server refused", async () => {
    const fetchStub = async () => ({
      ok: false,
      status: 500,
      json: async () => ({ message: "nope" }),
    });
    const { realm } = freshCategoriesScript(fetchStub);

    realm.document
      .querySelector('li[data-category-key="milk"] .btn-cat-up')
      .dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(categoryKeys(realm)).toEqual(["dairy products", "milk"]);
  });

  it("keeps the order the server accepted", async () => {
    const fetchStub = async () => ({
      ok: true,
      status: 200,
      json: async () => ({ message: "saved" }),
    });
    const { realm } = freshCategoriesScript(fetchStub);

    realm.document
      .querySelector('li[data-category-key="milk"] .btn-cat-up')
      .dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(categoryKeys(realm)).toEqual(["milk", "dairy products"]);
  });
});
