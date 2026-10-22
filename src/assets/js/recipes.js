document.addEventListener("DOMContentLoaded", function() {
    void loadRecipeSuggestions();
});

document.addEventListener("click", function(e) {
    if (e.target.closest("#refreshRecipesBtn") || e.target.closest("#recipesRetryBtn")) {
        e.preventDefault();
        void loadRecipeSuggestions(true);
    }
});

const RECIPE_LOADING_HTML = `
    <div class="text-center p-5">
        <div class="spinner-border text-body-secondary" role="status">
            <span class="visually-hidden">${proviant.t("js.recipes.loading")}</span>
        </div>
        <p class="mt-2 text-body-secondary">${proviant.t("js.recipes.loadingSuggestions")}</p>
    </div>`;

async function loadRecipeSuggestions(forceRefresh = false) {
    const container = document.getElementById("recipe-suggestions-container");
    const errorContainer = document.getElementById("recipe-error-container");
    const errorMessage = document.getElementById("error-message");

    // Already-rendered cards stay visible across a reload: a 429 or a provider
    // hiccup must not throw away suggestions the user is still reading. Only
    // the first load of an empty container needs the spinner.
    const hasCards = container.querySelector(".recipe-card") !== null;
    if (!hasCards) {
        container.innerHTML = RECIPE_LOADING_HTML;
    }
    container.classList.remove('d-none');
    errorContainer.classList.add('d-none');

    try {
        const response = await fetch('/api/v1/recipes/suggestions?limit=6' + (forceRefresh ? '&refresh=1' : ''));

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const recipes = await response.json();

        if (recipes.length === 0) {
            container.innerHTML = `
                <div class="text-center py-5">
                    <i class="bi bi-inbox fs-1 text-body-secondary"></i>
                    <p class="mt-3">${proviant.t("js.recipes.emptyTitle")}</p>
                    <p class="text-body-secondary">${proviant.t("js.recipes.emptyDesc")}</p>
                </div>
            `;
            return;
        }

        renderRecipes(recipes);

    } catch (error) {
        if (hasCards) {
            proviant.showFeedback("warning", proviant.t("js.recipes.suggestionsNotUpdatedTitle"), error.message);
            return;
        }
        errorMessage.textContent = error.message;
        container.classList.add('d-none');
        errorContainer.classList.remove('d-none');
    }
}

const renderedSuggestions = [];

function renderRecipes(recipes) {
    const container = document.getElementById("recipe-suggestions-container");

    renderedSuggestions.length = 0;
    recipes.forEach(recipe => renderedSuggestions.push(recipe));

    const html = `
        <div class="row row-cols-1 row-cols-md-2 row-cols-lg-3 g-4">
            ${recipes.map((recipe, index) => {
                const ingredients = recipe.ingredients || [];
                const cookIds = recipe.matchedProductIds || [];
                return `
                <div class="col">
                    <div class="card h-100 recipe-card shadow-sm">
                        ${recipe.imageUrl ? `
                            <img src="${escapeHtml(recipe.imageUrl)}" class="card-img-top" alt="${escapeHtml(recipe.title)}" loading="lazy" style="height: 200px; object-fit: cover;">
                        ` : `
                            <div class="bg-body-tertiary text-center py-4">
                                <i class="bi bi-journal-richtext fs-1 text-body-secondary"></i>
                            </div>
                        `}
                        <div class="card-body d-flex flex-column">
                            <h5 class="card-title">${escapeHtml(recipe.title)}</h5>

                            <div class="mb-2">
                                <div class="progress" style="height: 6px;">
                                    <div class="progress-bar bg-success" role="progressbar" style="width: ${recipe.matchPercent}%" aria-valuenow="${Math.round(recipe.matchPercent)}" aria-valuemin="0" aria-valuemax="100"></div>
                                </div>
                                <small class="text-body-secondary">${proviant.t("js.recipes.ingredientsMatched", recipe.matchedProducts.length, recipe.totalIngredients)}</small>
                            </div>

                            ${ingredients.length > 0 ? `
                            <div class="ingredient-list mt-2">
                                <small class="text-body-secondary mb-1 d-block">${proviant.t("js.recipes.ingredientsLabel")}</small>
                                <div class="d-flex flex-wrap gap-1">
                                    ${ingredients.map(ing => `
                                        <span class="badge badge--${ing.matched ? 'fresh' : 'expired'}"
                                              title="${ing.matched ? proviant.t("js.recipes.youHaveThis") : proviant.t("js.recipes.missingIngredient")}">
                                            ${ing.matched ? '<i class="bi bi-check2"></i> ' : '<i class="bi bi-x"></i> '}
                                            ${escapeHtml(ing.name)}
                                        </span>
                                    `).join('')}
                                </div>
                            </div>
                            ` : ''}


                        </div>
                        <div class="card-footer bg-transparent border-top-0 pb-3 d-flex flex-wrap gap-3 align-items-center">
                            <a href="${escapeHtml(recipe.sourceUrl)}" target="_blank" rel="noopener" class="btn btn-primary btn-sm${cookIds.length ? '' : ' stretched-link'}">
                                ${proviant.t("js.recipes.viewRecipe")} <i class="bi bi-box-arrow-up-right"></i>
                            </a>
                            <button type="button" class="btn btn-outline-secondary btn-sm position-relative z-2 save-suggestion-btn"
                                    data-suggestion-index="${index}"
                                    aria-label="${proviant.t("js.recipes.saveSuggestionLabel", escapeHtml(recipe.title))}">
                                <i class="bi bi-save"></i> ${proviant.t("js.recipes.saveRecipe")}
                            </button>
                            ${cookIds.length ? `
                            <button type="button" class="btn btn-outline-secondary btn-sm position-relative z-2 cook-recipe-btn"
                                    data-product-ids='${JSON.stringify(cookIds)}'
                                    aria-label="${proviant.tn("js.recipes.cookThisLabel", cookIds.length)}">
                                <i class="bi bi-fire"></i> ${proviant.t("js.recipes.cookThis")}
                            </button>
                            ` : ''}
                        </div>
                    </div>
                </div>
            `}).join('')}
        </div>
    `;

    container.innerHTML = html;
}

document.addEventListener("click", function(e) {
    const saveBtn = e.target.closest(".save-suggestion-btn");
    if (saveBtn) {
        e.preventDefault();
        const suggestion = renderedSuggestions[Number.parseInt(saveBtn.getAttribute("data-suggestion-index"), 10)];
        if (suggestion) {
            document.dispatchEvent(new CustomEvent("proviant:save-recipe-suggestion", { detail: suggestion }));
        }
        return;
    }

    const btn = e.target.closest(".cook-recipe-btn");
    if (!btn) return;
    e.preventDefault();

    let ids = [];
    try {
        ids = JSON.parse(btn.dataset.productIds || "[]");
    } catch (error) {
        console.error("Invalid product ID data on cook button", error);
        return;
    }
    if (!Array.isArray(ids) || ids.length === 0) return;

    proviant.showConfirm(
        proviant.t("js.recipes.cookConfirmTitle"),
        proviant.tn("js.recipes.cookConfirmBody", ids.length),
        function() { void cookMatchedProducts(btn, ids); },
        proviant.t("js.recipes.cook"),
        "warning"
    );
});

async function cookMatchedProducts(btn, ids) {
    const original = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> ' + proviant.t("js.recipes.cooking");
    try {
        // One pack per matched product: the stock is counted in packs, and a
        // recipe states what a dish needs, not how many packs of it are in
        // the cupboard.
        const items = ids.map(function(id) { return { productId: id, quantity: 1 }; });
        const res = await proviant.cookProducts(items);
        if (res.code !== 200) {
            const failedItems = res.errors || [];
            const detail = failedItems.length > 0 ? `: ${failedItems.join("; ")}` : "";
            proviant.showFeedback("error", proviant.t("js.recipes.cookFailedTitle"), proviant.t("js.recipes.cookRequestFailed", res.code, detail));
            return;
        }
        const consumed = res.consumed || 0;
        const partial = res.partial || 0;
        const itemErrors = res.errors || [];
        if (itemErrors.length > 0) {
            proviant.showFeedback("warning", proviant.t("js.recipes.partiallyCookedTitle"),
                `${proviant.tn("js.recipes.cookConsumed", consumed)}, ${proviant.t("js.recipes.cookPartial", partial)}, ${proviant.tn("js.recipes.cookNotConsumed", itemErrors.length)}.`);
        } else if (partial > 0) {
            proviant.showFeedback("success", proviant.t("js.recipes.productsConsumedTitle"),
                `${proviant.tn("js.recipes.cookConsumed", consumed)}, ${proviant.t("js.recipes.cookPartial", partial)}.`);
        } else {
            proviant.showFeedback("success", proviant.t("js.recipes.productsConsumedTitle"),
                proviant.tn("js.recipes.cookConsumedAll", consumed));
        }
        if (consumed + partial > 0) {
            void loadRecipeSuggestions(true);
        }
    } catch (error) {
        proviant.showFeedback("error", proviant.t("js.recipes.cookFailedTitle"), error.message);
    } finally {
        btn.disabled = false;
        btn.innerHTML = original;
    }
}

// Simple HTML escaping to prevent XSS, including attribute breakout via quotes
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
