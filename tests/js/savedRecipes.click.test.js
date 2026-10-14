import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * Every load of savedRecipes.js registers another document-level click
 * listener, so a second load in the same realm would fire every handler twice
 * per click. Keeping the delegation test alone in its own file removes that
 * coupling: this is the only load, so exactly one listener exists.
 */
const PAGE_MARKUP = `
<div id="recipeEditor" class="d-none">
  <span id="recipeEditorTitle"></span>
  <input id="recipeName">
  <input id="recipeServings">
  <input id="recipeSourceUrl">
  <textarea id="recipeDescription"></textarea>
  <textarea id="recipeInstructions"></textarea>
  <div id="ingredientRows"></div>
  <button id="btnAddIngredient"></button>
  <button id="btnSaveRecipe"></button>
  <button id="btnCancelRecipe"></button>
</div>
<div id="savedRecipesContainer"></div>
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("savedRecipes.js click delegation", () => {
  it("opens the editor, manages ingredient rows and confirms cooks", async () => {
    const showConfirm = vi.fn();
    const { exports: captured, realm } = loadClassicScript("savedRecipes.js", ["renderSavedRecipes"], {
      prelude: `document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
      globals: {
        proviant: {
          getProducts: vi.fn().mockResolvedValue({ code: 200, products: [] }),
          getRecipes: vi.fn().mockResolvedValue({ code: 200, recipes: [] }),
          getRecipe: vi.fn(),
          createRecipe: vi.fn(),
          updateRecipe: vi.fn(),
          deleteRecipe: vi.fn(),
          cookRecipe: vi.fn(),
          showFeedback: vi.fn(),
          showConfirm,
        },
      },
    });
    const doc = realm.document;

    doc.dispatchEvent(new realm.Event("DOMContentLoaded"));
    await flush();

    const newButton = doc.createElement("button");
    newButton.id = "btnNewRecipe";
    doc.body.appendChild(newButton);
    newButton.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    const editor = doc.getElementById("recipeEditor");
    expect(editor.classList.contains("d-none")).toBe(false);
    expect(doc.querySelectorAll("[data-ingredient-row]").length).toBe(1);

    doc.getElementById("btnAddIngredient").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    expect(doc.querySelectorAll("[data-ingredient-row]").length).toBe(2);

    doc.querySelector("[data-ingredient-remove]").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    expect(doc.querySelectorAll("[data-ingredient-row]").length).toBe(1);

    doc.getElementById("btnCancelRecipe").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    expect(editor.classList.contains("d-none")).toBe(true);

    captured.renderSavedRecipes([
      { id: 7, name: "Soup", servings: 2, description: "", ingredients: [] },
    ]);
    doc.querySelector("[data-recipe-cook]").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    expect(showConfirm).toHaveBeenCalledTimes(1);
    expect(showConfirm.mock.calls[0][0]).toBe("Cook recipe");

    doc.dispatchEvent(new realm.CustomEvent("proviant:save-recipe-suggestion", {
      detail: {
        title: "Pancakes",
        imageUrl: "https://img.test/pancakes.jpg",
        sourceUrl: "https://recipes.test/pancakes",
        ingredients: [{ name: "250 g flour", matched: false }, { name: "2 eggs", matched: true }, { name: "salt", matched: false }],
      },
    }));
    await flush();

    expect(editor.classList.contains("d-none")).toBe(false);
    expect(doc.getElementById("recipeName").value).toBe("Pancakes");
    expect(doc.getElementById("recipeSourceUrl").value).toBe("https://recipes.test/pancakes");
    expect(editor.dataset.imageUrl).toBe("https://img.test/pancakes.jpg");
    const rows = doc.querySelectorAll("[data-ingredient-row]");
    expect(rows.length).toBe(3);
    expect(rows[0].querySelector("[data-ingredient-name]").value).toBe("flour");
    expect(rows[0].querySelector("[data-ingredient-amount]").value).toBe("250");
    expect(rows[0].querySelector("[data-ingredient-unit]").value).toBe("g");
    expect(rows[1].querySelector("[data-ingredient-name]").value).toBe("eggs");
    expect(rows[1].querySelector("[data-ingredient-amount]").value).toBe("2");
    expect(rows[2].querySelector("[data-ingredient-name]").value).toBe("salt");
  });
});
