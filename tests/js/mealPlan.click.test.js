import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * Every load of mealPlan.js registers another document-level click listener,
 * so a second load in the same realm would fire every handler twice per click.
 * Keeping the delegation test alone in its own file removes that coupling:
 * this is the only load, so exactly one listener exists.
 */
const PAGE_MARKUP = `
<span id="monthLabel"></span>
<select id="inputMealDate"></select>
<select id="inputMealRecipe"></select>
<input id="inputMealServings">
<button id="btnAddMeal"></button>
<button id="btnGenerateShoppingList"></button>
<div id="mealPlanGrid"></div>
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("mealPlan.js click delegation", () => {
  it("renders the month grid, adds meals and confirms deletions", async () => {
    const createMealPlan = vi.fn().mockResolvedValue({ code: 201, entry: {} });
    const showConfirm = vi.fn();
    const { exports: captured, realm } = loadClassicScript("mealPlan.js", ["renderMealPlanEntries"], {
      prelude: `document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
      globals: {
        proviant: {
          getRecipes: vi.fn().mockResolvedValue({ code: 200, recipes: [{ id: 5, name: "Soup", servings: 2 }] }),
          getMealPlans: vi.fn().mockResolvedValue({ code: 200, entries: [] }),
          createMealPlan,
          updateMealPlan: vi.fn(),
          deleteMealPlan: vi.fn(),
          cookRecipe: vi.fn(),
          generateShoppingListFromMealPlan: vi.fn(),
          showFeedback: vi.fn(),
          showConfirm,
        },
      },
    });
    const doc = realm.document;

    doc.dispatchEvent(new realm.Event("DOMContentLoaded"));
    await flush();

    const dayNodes = doc.querySelectorAll(".mp-day");
    expect(dayNodes.length).toBeGreaterThanOrEqual(28);
    expect(dayNodes.length % 7).toBe(0);
    expect(doc.getElementById("monthLabel").textContent).not.toBe("");
    expect(doc.getElementById("inputMealRecipe").options.length).toBe(1);

    doc.getElementById("inputMealServings").value = "3";
    doc.getElementById("btnAddMeal").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(createMealPlan).toHaveBeenCalledTimes(1);
    expect(createMealPlan.mock.calls[0][0]).toMatchObject({ recipeId: 5, servings: 3 });

    const plannedDate = doc.getElementById("inputMealDate").value;
    captured.renderMealPlanEntries([
      { id: 11, date: plannedDate, recipeId: 5, recipeName: "Soup", servings: 2 },
    ]);
    doc.querySelector("[data-meal-delete]").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    expect(showConfirm).toHaveBeenCalledTimes(1);
    expect(showConfirm.mock.calls[0][0]).toBe("Remove meal");
  });
});
