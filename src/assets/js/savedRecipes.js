document.addEventListener("DOMContentLoaded", function () {
    loadSavedRecipes();
});

document.addEventListener("proviant:save-recipe-suggestion", function (e) {
    openEditorFromSuggestion(e.detail);
});

document.addEventListener("click", function (e) {
    const target = e.target;
    if (!target || !target.closest) return;

    if (target.closest("#btnNewRecipe") || target.closest("#btnNewRecipeMobile") || target.closest("#btnNewRecipeEmpty")) {
        e.preventDefault();
        openRecipeEditor(null);
        return;
    }
    if (target.closest("#btnCancelRecipe")) {
        e.preventDefault();
        closeRecipeEditor();
        return;
    }
    if (target.closest("#btnSaveRecipe")) {
        e.preventDefault();
        saveRecipe();
        return;
    }
    if (target.closest("#btnAddIngredient")) {
        e.preventDefault();
        addIngredientRow();
        return;
    }
    const removeButton = target.closest("[data-ingredient-remove]");
    if (removeButton) {
        e.preventDefault();
        const row = removeButton.closest("[data-ingredient-row]");
        if (row) row.remove();
        return;
    }
    const editButton = target.closest("[data-recipe-edit]");
    if (editButton) {
        e.preventDefault();
        editRecipe(parseInt(editButton.getAttribute("data-recipe-edit"), 10));
        return;
    }
    const deleteButton = target.closest("[data-recipe-delete]");
    if (deleteButton) {
        e.preventDefault();
        confirmDeleteRecipe(parseInt(deleteButton.getAttribute("data-recipe-delete"), 10), deleteButton.getAttribute("data-recipe-name") || "this recipe");
        return;
    }
    const cookButton = target.closest("[data-recipe-cook]");
    if (cookButton) {
        e.preventDefault();
        confirmCookRecipe(parseInt(cookButton.getAttribute("data-recipe-cook"), 10), cookButton.getAttribute("data-recipe-name") || "this recipe");
    }
});

let editingRecipeID = null;
let productOptions = null;

function escapeRecipeHtml(text) {
    if (!text) return "";
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

const SUGGESTION_UNITS = ["g", "kg", "mg", "l", "ml", "el", "tl", "tbsp", "tsp", "cup", "cups", "oz", "lb", "pkg", "pack", "pinch", "pcs", "pc", "piece", "pieces", "bunch", "slice", "slices", "clove", "cloves", "can", "cans", "tin", "bag", "bags", "bottle", "bottles", "handful"];

function parseSuggestionIngredient(raw) {
    const text = (raw || "").trim();
    const match = text.match(/^(\d+(?:[.,]\d+)?)\s*([A-Za-zäöüß]+)?\s+(.+)$/);
    if (!match) {
        return { name: text, amount: 0, unit: "", productId: null };
    }
    const amount = Math.round(parseFloat(match[1].replace(",", "."))) || 0;
    const unit = (match[2] || "").toLowerCase();
    if (unit && SUGGESTION_UNITS.indexOf(unit) !== -1) {
        return { name: match[3].trim().replace(/^of\s+/i, ""), amount: amount, unit: unit, productId: null };
    }
    return { name: ((match[2] || "") + " " + match[3]).trim(), amount: amount, unit: "", productId: null };
}

async function openEditorFromSuggestion(suggestion) {
    if (!suggestion) return;
    const ingredients = (suggestion.ingredients || [])
        .map((ingredient) => parseSuggestionIngredient(ingredient.name))
        .filter((ingredient) => ingredient.name !== "");
    await openRecipeEditor({
        name: suggestion.title || "",
        servings: 2,
        description: "",
        instructions: "",
        sourceUrl: suggestion.sourceUrl || "",
        imageUrl: suggestion.imageUrl || "",
        ingredients: ingredients,
    });
    proviant.showFeedback("success", "Recipe prefilled", "Review the imported ingredients, then save.");
}

async function loadProductOptions() {
    if (productOptions !== null) return productOptions;
    try {
        const res = await proviant.getProducts();
        productOptions = res.code === 200 && Array.isArray(res.products) ? res.products : [];
    } catch (_error) {
        productOptions = [];
    }
    return productOptions;
}

function ingredientRowHtml(ingredient, products) {
    const options = products.map(function (product) {
        const selected = ingredient && ingredient.productId === product.id ? " selected" : "";
        return `<option value="${product.id}"${selected}>${escapeRecipeHtml(product.productName)}</option>`;
    }).join("");
    return `
        <div class="row g-2 mb-1" data-ingredient-row>
            <div class="col-12 col-md-4">
                <select class="form-select form-select-sm" data-ingredient-product aria-label="Linked product">
                    <option value="">No product</option>
                    ${options}
                </select>
            </div>
            <div class="col-6 col-md-3">
                <input type="text" class="form-control form-control-sm" data-ingredient-name maxlength="255" placeholder="Name" aria-label="Ingredient name" value="${escapeRecipeHtml(ingredient ? ingredient.name : "")}">
            </div>
            <div class="col-3 col-md-2">
                <input type="number" class="form-control form-control-sm" data-ingredient-amount min="0" placeholder="Amount" aria-label="Amount" value="${ingredient ? ingredient.amount : ""}">
            </div>
            <div class="col-3 col-md-2">
                <input type="text" class="form-control form-control-sm" data-ingredient-unit maxlength="20" placeholder="Unit" aria-label="Unit" value="${escapeRecipeHtml(ingredient ? ingredient.unit : "")}">
            </div>
            <div class="col-12 col-md-1 d-flex align-items-center">
                <button type="button" class="btn-icon text-danger" data-ingredient-remove aria-label="Remove ingredient">
                    <i class="bi bi-trash3" aria-hidden="true"></i>
                </button>
            </div>
        </div>`;
}

function addIngredientRow() {
    const rows = document.getElementById("ingredientRows");
    if (!rows) return;
    rows.insertAdjacentHTML("beforeend", ingredientRowHtml(null, productOptions || []));
}

async function openRecipeEditor(recipe) {
    const editor = document.getElementById("recipeEditor");
    if (!editor) return;
    editingRecipeID = recipe ? recipe.id : null;
    editor.dataset.imageUrl = recipe && recipe.imageUrl ? recipe.imageUrl : "";
    document.getElementById("recipeEditorTitle").textContent = recipe ? "Edit recipe" : "New recipe";
    document.getElementById("recipeName").value = recipe ? recipe.name : "";
    document.getElementById("recipeServings").value = recipe && recipe.servings ? recipe.servings : 2;
    document.getElementById("recipeSourceUrl").value = recipe ? (recipe.sourceUrl || "") : "";
    document.getElementById("recipeDescription").value = recipe ? (recipe.description || "") : "";
    document.getElementById("recipeInstructions").value = recipe ? (recipe.instructions || "") : "";
    const products = await loadProductOptions();
    const ingredients = recipe && Array.isArray(recipe.ingredients) && recipe.ingredients.length > 0 ? recipe.ingredients : [null];
    document.getElementById("ingredientRows").innerHTML = ingredients.map(function (ingredient) {
        return ingredientRowHtml(ingredient, products);
    }).join("");
    editor.classList.remove("d-none");
    document.getElementById("recipeName").focus();
}

function closeRecipeEditor() {
    const editor = document.getElementById("recipeEditor");
    if (!editor) return;
    editor.classList.add("d-none");
    editingRecipeID = null;
}

async function editRecipe(recipeID) {
    try {
        const res = await proviant.getRecipe(recipeID);
        if (res.code !== 200 || !res.recipe) {
            proviant.showFeedback("error", "Recipe not loaded", res.message || `Request failed with status ${res.code}`);
            return;
        }
        await openRecipeEditor(res.recipe);
    } catch (error) {
        proviant.showFeedback("error", "Recipe not loaded", error.message);
    }
}

function collectRecipePayload() {
    const editor = document.getElementById("recipeEditor");
    const ingredients = Array.from(document.querySelectorAll("#ingredientRows [data-ingredient-row]")).map(function (row, index) {
        const productRaw = row.querySelector("[data-ingredient-product]").value;
        const amount = parseInt(row.querySelector("[data-ingredient-amount]").value, 10);
        return {
            productId: productRaw ? parseInt(productRaw, 10) : null,
            name: row.querySelector("[data-ingredient-name]").value.trim(),
            amount: Number.isFinite(amount) && amount > 0 ? amount : 0,
            unit: row.querySelector("[data-ingredient-unit]").value.trim(),
            sortOrder: index,
        };
    }).filter(function (ingredient) {
        return ingredient.name !== "";
    });
    return {
        name: document.getElementById("recipeName").value.trim(),
        description: document.getElementById("recipeDescription").value.trim(),
        instructions: document.getElementById("recipeInstructions").value.trim(),
        servings: parseInt(document.getElementById("recipeServings").value, 10) || 1,
        sourceUrl: document.getElementById("recipeSourceUrl").value.trim(),
        imageUrl: editor ? (editor.dataset.imageUrl || "") : "",
        ingredients: ingredients,
    };
}

async function saveRecipe() {
    const payload = collectRecipePayload();
    if (!payload.name) {
        proviant.showFeedback("error", "Name required", "Please give the recipe a name.");
        return;
    }
    try {
        const res = editingRecipeID
            ? await proviant.updateRecipe(editingRecipeID, payload)
            : await proviant.createRecipe(payload);
        if (res.code !== 200 && res.code !== 201) {
            proviant.showFeedback("error", "Recipe not saved", res.message || `Request failed with status ${res.code}`);
            return;
        }
        proviant.showFeedback("success", "Recipe saved", `${payload.name} was saved.`);
        closeRecipeEditor();
        loadSavedRecipes();
    } catch (error) {
        proviant.showFeedback("error", "Recipe not saved", error.message);
    }
}

function confirmDeleteRecipe(recipeID, recipeName) {
    proviant.showConfirm(
        "Delete recipe",
        `This will permanently delete "${recipeName}". This cannot be undone.`,
        function () { deleteRecipe(recipeID); },
        "Delete",
        "danger"
    );
}

async function deleteRecipe(recipeID) {
    try {
        const res = await proviant.deleteRecipe(recipeID);
        if (res.code !== 200) {
            proviant.showFeedback("error", "Recipe not deleted", res.message || `Request failed with status ${res.code}`);
            return;
        }
        proviant.showFeedback("success", "Recipe deleted", "The recipe was deleted.");
        if (editingRecipeID === recipeID) closeRecipeEditor();
        loadSavedRecipes();
    } catch (error) {
        proviant.showFeedback("error", "Recipe not deleted", error.message);
    }
}

function confirmCookRecipe(recipeID, recipeName) {
    proviant.showConfirm(
        "Cook recipe",
        `This will consume the linked ingredients of "${recipeName}" from your stock. This cannot be undone.`,
        function () { cookRecipe(recipeID); },
        "Cook",
        "warning"
    );
}

async function cookRecipe(recipeID) {
    try {
        const res = await proviant.cookRecipe(recipeID, 0);
        if (res.code !== 200) {
            const itemErrors = res.errors || [];
            const detail = itemErrors.length > 0 ? `: ${itemErrors.join("; ")}` : "";
            proviant.showFeedback("error", "Cook failed", `The cook request failed (HTTP ${res.code})${detail}.`);
            return;
        }
        const consumed = res.consumed || 0;
        const partial = res.partial || 0;
        const itemErrors = res.errors || [];
        if (itemErrors.length > 0) {
            proviant.showFeedback("warning", "Partially cooked",
                `${consumed} product(s) fully consumed, ${partial} partially, ${itemErrors.length} ingredient(s) skipped: ${itemErrors.join("; ")}`);
        } else if (partial > 0) {
            proviant.showFeedback("success", "Recipe cooked", `${consumed} product(s) fully consumed, ${partial} partially.`);
        } else {
            proviant.showFeedback("success", "Recipe cooked", `${consumed} product(s) marked as fully consumed.`);
        }
    } catch (error) {
        proviant.showFeedback("error", "Cook failed", error.message);
    }
}

async function loadSavedRecipes() {
    const container = document.getElementById("savedRecipesContainer");
    if (!container) return;
    try {
        const res = await proviant.getRecipes();
        if (res.code !== 200) {
            container.innerHTML = `
                <div class="empty">
                    <div class="empty-icon empty-icon-danger">
                        <i class="bi bi-exclamation-triangle"></i>
                    </div>
                    <div class="empty-title">Unable to load your recipes</div>
                    <div class="empty-desc">${escapeRecipeHtml(res.message || `Request failed with status ${res.code}`)}</div>
                </div>`;
            return;
        }
        renderSavedRecipes(res.recipes);
    } catch (error) {
        container.innerHTML = `
            <div class="empty">
                <div class="empty-icon empty-icon-danger">
                    <i class="bi bi-exclamation-triangle"></i>
                </div>
                <div class="empty-title">Unable to load your recipes</div>
                <div class="empty-desc">${escapeRecipeHtml(error.message)}</div>
            </div>`;
    }
}

function renderSavedRecipes(recipes) {
    const container = document.getElementById("savedRecipesContainer");
    if (recipes.length === 0) {
        container.innerHTML = `
            <div class="empty">
                <div class="empty-icon">
                    <i class="bi bi-journal-richtext"></i>
                </div>
                <div class="empty-title">No saved recipes yet</div>
                <div class="empty-desc">Save your favourite recipes to plan meals and cook them with one click.</div>
                <button class="btn btn-proviant-primary" id="btnNewRecipeEmpty" type="button">
                    <i class="bi bi-plus-lg me-1"></i> New recipe
                </button>
            </div>`;
        return;
    }
    container.innerHTML = `
        <div class="row row-cols-1 row-cols-md-2 row-cols-lg-3 g-4">
            ${recipes.map(function (recipe) {
                return `
                <div class="col">
                    <div class="card h-100 recipe-card shadow-sm">
                        <div class="card-body d-flex flex-column">
                            <h3 class="h6">${escapeRecipeHtml(recipe.name)}</h3>
                            <p class="mp-entry-meta mb-2">${recipe.servings} serving(s)</p>
                            ${recipe.description ? `<p class="small text-secondary-custom">${escapeRecipeHtml(recipe.description)}</p>` : ""}
                            <div class="mt-auto d-flex flex-wrap gap-2">
                                <button type="button" class="btn btn-proviant-primary btn-sm" data-recipe-cook="${recipe.id}" data-recipe-name="${escapeRecipeHtml(recipe.name)}">
                                    <i class="bi bi-fire me-1" aria-hidden="true"></i> Cook
                                </button>
                                <button type="button" class="btn btn-proviant-secondary btn-sm" data-recipe-edit="${recipe.id}" aria-label="Edit ${escapeRecipeHtml(recipe.name)}">
                                    <i class="bi bi-pencil" aria-hidden="true"></i> Edit
                                </button>
                                <button type="button" class="btn btn-proviant-secondary btn-sm text-danger" data-recipe-delete="${recipe.id}" data-recipe-name="${escapeRecipeHtml(recipe.name)}" aria-label="Delete ${escapeRecipeHtml(recipe.name)}">
                                    <i class="bi bi-trash3" aria-hidden="true"></i> Delete
                                </button>
                            </div>
                        </div>
                    </div>
                </div>`;
            }).join("")}
        </div>`;
}
