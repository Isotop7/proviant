document.addEventListener("DOMContentLoaded", function() {
    loadRecipeSuggestions();
});

document.addEventListener("click", function(e) {
    if (e.target.closest("#refreshRecipesBtn") || e.target.closest("#recipesRetryBtn")) {
        e.preventDefault();
        loadRecipeSuggestions(true);
    }
});

const RECIPE_LOADING_HTML = `
    <div class="text-center p-5">
        <div class="spinner-border text-secondary-custom" role="status">
            <span class="visually-hidden">Loading...</span>
        </div>
        <p class="mt-2 text-secondary-custom">Loading recipe suggestions...</p>
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
                    <i class="bi bi-inbox fs-1 text-secondary-custom"></i>
                    <p class="mt-3">No recipe suggestions found for your expiring products.</p>
                    <p class="text-secondary-custom">Try adding products with expiry dates to get personalized suggestions!</p>
                </div>
            `;
            return;
        }

        renderRecipes(recipes);

    } catch (error) {
        if (hasCards) {
            proviant.showFeedback("warning", "Suggestions not updated", error.message);
            return;
        }
        errorMessage.textContent = error.message;
        container.classList.add('d-none');
        errorContainer.classList.remove('d-none');
    }
}

function renderRecipes(recipes) {
    const container = document.getElementById("recipe-suggestions-container");

    const html = `
        <div class="row row-cols-1 row-cols-md-2 row-cols-lg-3 g-4">
            ${recipes.map(recipe => {
                const ingredients = recipe.ingredients || [];
                const cookIds = recipe.matchedProductIds || [];
                return `
                <div class="col">
                    <div class="card h-100 recipe-card shadow-sm">
                        ${recipe.imageUrl ? `
                            <img src="${escapeHtml(recipe.imageUrl)}" class="card-img-top" alt="${escapeHtml(recipe.title)}" loading="lazy" style="height: 200px; object-fit: cover;">
                        ` : `
                            <div class="bg-body-tertiary text-center py-4">
                                <i class="bi bi-journal-richtext fs-1 text-secondary-custom"></i>
                            </div>
                        `}
                        <div class="card-body d-flex flex-column">
                            <h5 class="card-title">${escapeHtml(recipe.title)}</h5>

                            <div class="mb-2">
                                <div class="progress" style="height: 6px;">
                                    <div class="progress-bar bg-success" role="progressbar" style="width: ${recipe.matchPercent}%" aria-valuenow="${Math.round(recipe.matchPercent)}" aria-valuemin="0" aria-valuemax="100"></div>
                                </div>
                                <small class="text-secondary-custom">${recipe.matchedProducts.length} of ${recipe.totalIngredients} ingredients matched</small>
                            </div>

                            ${ingredients.length > 0 ? `
                            <div class="ingredient-list mt-2">
                                <small class="text-secondary-custom mb-1 d-block">Ingredients:</small>
                                <div class="d-flex flex-wrap gap-1">
                                    ${ingredients.map(ing => `
                                        <span class="badge-status ${ing.matched ? 'badge-fresh' : 'badge-expired'}"
                                              title="${ing.matched ? 'You have this' : 'Missing ingredient'}">
                                            ${ing.matched ? '<i class="bi bi-check2"></i> ' : '<i class="bi bi-x"></i> '}
                                            ${escapeHtml(ing.name)}
                                        </span>
                                    `).join('')}
                                </div>
                            </div>
                            ` : ''}


                        </div>
                        <div class="card-footer bg-transparent border-top-0 pb-3">
                            <a href="${escapeHtml(recipe.sourceUrl)}" target="_blank" rel="noopener" class="btn btn-proviant-primary btn-sm${cookIds.length ? '' : ' stretched-link'}">
                                View Recipe <i class="bi bi-box-arrow-up-right"></i>
                            </a>
                            ${cookIds.length ? `
                            <button type="button" class="btn btn-outline-secondary btn-sm ms-2 cook-recipe-btn"
                                    data-product-ids='${JSON.stringify(cookIds)}'
                                    aria-label="Cook this recipe: mark the ${cookIds.length} matched product(s) as fully consumed">
                                <i class="bi bi-fire"></i> Cook this
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
        "Cook this recipe?",
        `This will mark ${ids.length} matched product${ids.length !== 1 ? "s" : ""} as fully consumed. This cannot be undone.`,
        function() { cookMatchedProducts(btn, ids); },
        "Cook",
        "warning"
    );
});

async function cookMatchedProducts(btn, ids) {
    const original = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Cooking...';
    try {
        const items = ids.map(function(id) { return { productId: id, amount: 0 }; });
        const res = await proviant.cookProducts(items);
        if (res.code !== 200) {
            const failedItems = res.errors || [];
            const detail = failedItems.length > 0 ? `: ${failedItems.join("; ")}` : "";
            proviant.showFeedback("error", "Cook failed", `The cook request failed (HTTP ${res.code})${detail}.`);
            return;
        }
        const consumed = res.consumed || 0;
        const partial = res.partial || 0;
        const itemErrors = res.errors || [];
        if (itemErrors.length > 0) {
            proviant.showFeedback("warning", "Partially cooked",
                `${consumed} product(s) fully consumed, ${partial} partially, ${itemErrors.length} could not be consumed.`);
        } else if (partial > 0) {
            proviant.showFeedback("success", "Products consumed",
                `${consumed} product(s) fully consumed, ${partial} partially.`);
        } else {
            proviant.showFeedback("success", "Products consumed",
                `${consumed} product(s) marked as fully consumed.`);
        }
        if (consumed + partial > 0) {
            loadRecipeSuggestions(true);
        }
    } catch (error) {
        proviant.showFeedback("error", "Cook failed", error.message);
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
