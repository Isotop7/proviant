document.addEventListener("DOMContentLoaded", function () {
    void loadSavedRecipes();
});

document.addEventListener("proviant:save-recipe-suggestion", function (e) {
    void openEditorFromSuggestion(e.detail);
});

document.addEventListener("click", function (e) {
    const target = e.target;
    if (!target || !target.closest) return;

    if (target.closest("#btnNewRecipe") || target.closest("#btnNewRecipeMobile") || target.closest("#btnNewRecipeEmpty")) {
        e.preventDefault();
        void openRecipeEditor(null);
        return;
    }
    if (target.closest("#btnCancelRecipe")) {
        e.preventDefault();
        closeRecipeEditor();
        return;
    }
    if (target.closest("#btnSaveRecipe")) {
        e.preventDefault();
        void saveRecipe();
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
        void editRecipe(Number.parseInt(editButton.getAttribute("data-recipe-edit"), 10));
        return;
    }
    const deleteButton = target.closest("[data-recipe-delete]");
    if (deleteButton) {
        e.preventDefault();
        confirmDeleteRecipe(Number.parseInt(deleteButton.getAttribute("data-recipe-delete"), 10), deleteButton.getAttribute("data-recipe-name") || proviant.t("js.savedRecipes.thisRecipe"));
        return;
    }
    const cookButton = target.closest("[data-recipe-cook]");
    if (cookButton) {
        e.preventDefault();
        confirmCookRecipe(Number.parseInt(cookButton.getAttribute("data-recipe-cook"), 10), cookButton.getAttribute("data-recipe-name") || proviant.t("js.savedRecipes.thisRecipe"));
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

// SUGGESTION_UNITS are the tokens that end the portion of a suggested
// ingredient text ("200 g flour"). The list only tells the parser where the
// name starts; the portion itself is kept as free text.
const SUGGESTION_UNITS = ["g", "kg", "mg", "l", "ml", "el", "tl", "tbsp", "tsp", "cup", "cups", "oz", "lb", "pkg", "pack", "pinch", "pcs", "pc", "piece", "pieces", "bunch", "slice", "slices", "clove", "cloves", "can", "cans", "tin", "bag", "bags", "bottle", "bottles", "handful"];

// parseSuggestionIngredient splits a suggested ingredient text into the
// portion and the name. The portion keeps the wording of the suggestion, so
// nothing about it is converted or rounded.
function parseSuggestionIngredient(raw) {
    const text = (raw || "").trim();
    const amountMatch = text.match(/^(\d+(?:[.,]\d+)?)\s+/);
    if (!amountMatch) {
        return { name: text, portion: "", productId: null };
    }
    const rest = text.slice(amountMatch[0].length);
    const unitMatch = rest.match(/^([A-Za-zäöüß]+)\s+/);
    if (unitMatch && SUGGESTION_UNITS.indexOf(unitMatch[1].toLowerCase()) !== -1) {
        return {
            name: rest.slice(unitMatch[0].length).trim().replace(/^of\s+/i, ""),
            portion: amountMatch[1] + " " + unitMatch[1].toLowerCase(),
            productId: null,
        };
    }
    return { name: rest.trim(), portion: amountMatch[1], productId: null };
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
    proviant.showFeedback("success", proviant.t("js.savedRecipes.prefilledTitle"), proviant.t("js.savedRecipes.prefilledBody"));
}

async function loadProductOptions() {
    if (productOptions !== null) return productOptions;
    try {
        const res = await proviant.getProducts();
        // An entry without an id would build an option that parseInt to NaN at save
        // and silently drop the link the user just picked.
        productOptions = res.code === 200 && Array.isArray(res.products) ? res.products.filter((product) => product && product.id) : [];
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
            <div class="col-12 md:col-4">
                <select class="form-control form-control-sm" data-ingredient-product aria-label="${proviant.t("js.savedRecipes.linkedProduct")}">
                    <option value="">${proviant.t("js.savedRecipes.noProduct")}</option>
                    ${options}
                </select>
            </div>
            <div class="col-6 md:col-3">
                <input type="text" class="form-control form-control-sm" data-ingredient-name maxlength="255" placeholder="${proviant.t("js.savedRecipes.namePlaceholder")}" aria-label="${proviant.t("js.savedRecipes.ingredientName")}" value="${escapeRecipeHtml(ingredient ? ingredient.name : "")}">
            </div>
            <div class="col-6 md:col-4">
                <input type="text" class="form-control form-control-sm" data-ingredient-portion maxlength="40" placeholder="${proviant.t("js.savedRecipes.portion")}" aria-label="${proviant.t("js.savedRecipes.portionLabel")}" value="${escapeRecipeHtml(ingredient ? ingredient.portion : "")}">
            </div>
            <div class="col-12 md:col-1 d-flex align-items-center">
                <button type="button" class="btn-icon fg-danger" data-ingredient-remove aria-label="${proviant.t("js.savedRecipes.removeIngredient")}">
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
    document.getElementById("recipeEditorTitle").textContent = recipe ? proviant.t("js.savedRecipes.editRecipeTitle") : proviant.t("js.savedRecipes.newRecipe");
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
            proviant.showFeedback("error", proviant.t("js.savedRecipes.notLoadedTitle"), res.message || proviant.t("js.savedRecipes.requestFailed", res.code));
            return;
        }
        await openRecipeEditor(res.recipe);
    } catch (error) {
        proviant.showFeedback("error", proviant.t("js.savedRecipes.notLoadedTitle"), error.message);
    }
}

function collectRecipePayload() {
    const editor = document.getElementById("recipeEditor");
    const ingredients = Array.from(document.querySelectorAll("#ingredientRows [data-ingredient-row]")).map(function (row, index) {
        const productRaw = row.querySelector("[data-ingredient-product]").value;
        return {
            productId: productRaw ? Number.parseInt(productRaw, 10) : null,
            name: row.querySelector("[data-ingredient-name]").value.trim(),
            portion: row.querySelector("[data-ingredient-portion]").value.trim(),
            sortOrder: index,
        };
    }).filter(function (ingredient) {
        return ingredient.name !== "";
    });
    return {
        name: document.getElementById("recipeName").value.trim(),
        description: document.getElementById("recipeDescription").value.trim(),
        instructions: document.getElementById("recipeInstructions").value.trim(),
        servings: Number.parseInt(document.getElementById("recipeServings").value, 10) || 1,
        sourceUrl: document.getElementById("recipeSourceUrl").value.trim(),
        imageUrl: editor ? (editor.dataset.imageUrl || "") : "",
        ingredients: ingredients,
    };
}

async function saveRecipe() {
    const payload = collectRecipePayload();
    if (!payload.name) {
        proviant.showFeedback("error", proviant.t("js.savedRecipes.nameRequired"), proviant.t("js.savedRecipes.nameRequiredBody"));
        return;
    }
    try {
        const res = editingRecipeID
            ? await proviant.updateRecipe(editingRecipeID, payload)
            : await proviant.createRecipe(payload);
        if (res.code !== 200 && res.code !== 201) {
            proviant.showFeedback("error", proviant.t("js.savedRecipes.notSavedTitle"), res.message || proviant.t("js.savedRecipes.requestFailed", res.code));
            return;
        }
        proviant.showFeedback("success", proviant.t("js.savedRecipes.savedTitle"), proviant.t("js.savedRecipes.savedBody", payload.name));
        closeRecipeEditor();
        void loadSavedRecipes();
    } catch (error) {
        proviant.showFeedback("error", proviant.t("js.savedRecipes.notSavedTitle"), error.message);
    }
}

function confirmDeleteRecipe(recipeID, recipeName) {
    proviant.showConfirm(
        proviant.t("js.savedRecipes.deleteConfirmTitle"),
        proviant.t("js.savedRecipes.deleteConfirmBody", recipeName),
        function () { void deleteRecipe(recipeID); },
        proviant.t("common.delete"),
        "danger"
    );
}

async function deleteRecipe(recipeID) {
    try {
        const res = await proviant.deleteRecipe(recipeID);
        if (res.code !== 200) {
            proviant.showFeedback("error", proviant.t("js.savedRecipes.notDeletedTitle"), res.message || proviant.t("js.savedRecipes.requestFailed", res.code));
            return;
        }
        proviant.showFeedback("success", proviant.t("js.savedRecipes.deletedTitle"), proviant.t("js.savedRecipes.deletedBody"));
        if (editingRecipeID === recipeID) closeRecipeEditor();
        void loadSavedRecipes();
    } catch (error) {
        proviant.showFeedback("error", proviant.t("js.savedRecipes.notDeletedTitle"), error.message);
    }
}

function confirmCookRecipe(recipeID, recipeName) {
    proviant.showConfirm(
        proviant.t("js.savedRecipes.cookConfirmTitle"),
        proviant.t("js.savedRecipes.cookConfirmBody", recipeName),
        function () { void cookRecipe(recipeID); },
        proviant.t("js.savedRecipes.cook"),
        "warning"
    );
}

async function cookRecipe(recipeID) {
    try {
        const res = await proviant.cookRecipe(recipeID, 0);
        if (res.code !== 200) {
            const itemErrors = res.errors || [];
            const detail = itemErrors.length > 0 ? `: ${itemErrors.join("; ")}` : "";
            proviant.showFeedback("error", proviant.t("js.savedRecipes.cookFailedTitle"), proviant.t("js.savedRecipes.cookRequestFailed", res.code, detail));
            return;
        }
        const consumed = res.consumed || 0;
        const partial = res.partial || 0;
        const itemErrors = res.errors || [];
        if (itemErrors.length > 0) {
            proviant.showFeedback("warning", proviant.t("js.savedRecipes.partiallyCookedTitle"),
                `${proviant.tn("js.savedRecipes.cookConsumed", consumed)}, ${proviant.t("js.savedRecipes.cookPartial", partial)}, ${proviant.tn("js.savedRecipes.cookSkipped", itemErrors.length)}: ${itemErrors.join("; ")}`);
        } else if (partial > 0) {
            proviant.showFeedback("success", proviant.t("js.savedRecipes.cookedTitle"), `${proviant.tn("js.savedRecipes.cookConsumed", consumed)}, ${proviant.t("js.savedRecipes.cookPartial", partial)}.`);
        } else {
            proviant.showFeedback("success", proviant.t("js.savedRecipes.cookedTitle"), proviant.tn("js.savedRecipes.cookConsumedAll", consumed));
        }
    } catch (error) {
        proviant.showFeedback("error", proviant.t("js.savedRecipes.cookFailedTitle"), error.message);
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
                    <div class="empty-title">${proviant.t("js.savedRecipes.loadErrorTitle")}</div>
                    <div class="empty-desc">${escapeRecipeHtml(res.message || proviant.t("js.savedRecipes.requestFailed", res.code))}</div>
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
                <div class="empty-title">${proviant.t("js.savedRecipes.loadErrorTitle")}</div>
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
                <div class="empty-title">${proviant.t("js.savedRecipes.emptyTitle")}</div>
                <div class="empty-desc">${proviant.t("js.savedRecipes.emptyDesc")}</div>
                <button class="btn btn-solid theme-primary" id="btnNewRecipeEmpty" type="button">
                    <i class="bi bi-plus-lg me-1"></i> ${proviant.t("js.savedRecipes.newRecipe")}
                </button>
            </div>`;
        return;
    }
    container.innerHTML = `
        <div class="row row-cols-1 md:row-cols-2 lg:row-cols-3 g-4">
            ${recipes.map(function (recipe) {
                return `
                <div class="col">
                    <div class="card h-100 recipe-card shadow-sm">
                        <div class="card-body d-flex flex-column">
                            <h3 class="h6">${escapeRecipeHtml(recipe.name)}</h3>
                            <p class="mp-entry-meta mb-3">${proviant.tn("js.savedRecipes.servings", recipe.servings)}</p>
                            ${recipe.description ? `<p class="small fg-secondary">${escapeRecipeHtml(recipe.description)}</p>` : ""}
                            <div class="mt-auto d-flex flex-wrap gap-3">
                                <button type="button" class="btn btn-solid theme-primary btn-sm" data-recipe-cook="${recipe.id}" data-recipe-name="${escapeRecipeHtml(recipe.name)}">
                                    <i class="bi bi-fire me-1" aria-hidden="true"></i> ${proviant.t("js.savedRecipes.cook")}
                                </button>
                                <button type="button" class="btn btn-outline theme-secondary btn-sm" data-recipe-edit="${recipe.id}" aria-label="${proviant.t("js.savedRecipes.editLabel", escapeRecipeHtml(recipe.name))}">
                                    <i class="bi bi-pencil" aria-hidden="true"></i> ${proviant.t("common.edit")}
                                </button>
                                <button type="button" class="btn btn-outline theme-secondary btn-sm fg-danger" data-recipe-delete="${recipe.id}" data-recipe-name="${escapeRecipeHtml(recipe.name)}" aria-label="${proviant.t("js.savedRecipes.deleteLabel", escapeRecipeHtml(recipe.name))}">
                                    <i class="bi bi-trash3" aria-hidden="true"></i> ${proviant.t("common.delete")}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>`;
            }).join("")}
        </div>`;
}
