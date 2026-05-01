document.addEventListener("DOMContentLoaded", function() {
    loadRecipeSuggestions();
});

document.addEventListener("click", function(e) {
    if (e.target.closest("#refreshRecipesBtn")) {
        e.preventDefault();
        loadRecipeSuggestions(true);
    }
});

async function loadRecipeSuggestions(forceRefresh = false) {
    const container = document.getElementById("recipe-suggestions-container");
    const errorContainer = document.getElementById("recipe-error-container");
    const errorMessage = document.getElementById("error-message");

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
                return `
                <div class="col">
                    <div class="card h-100 recipe-card shadow-sm">
                        ${recipe.imageUrl ? `
                            <img src="${recipe.imageUrl}" class="card-img-top" alt="${escapeHtml(recipe.title)}" loading="lazy" style="height: 200px; object-fit: cover;">
                        ` : `
                            <div class="bg-light text-center py-4">
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
                                        <span class="badge ${ing.matched ? 'bg-success' : 'bg-warning text-dark'}" 
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
                            <a href="${escapeHtml(recipe.sourceUrl)}" target="_blank" rel="noopener" class="btn btn-outline-primary btn-sm stretched-link">
                                View Recipe <i class="bi bi-box-arrow-up-right"></i>
                            </a>
                        </div>
                    </div>
                </div>
            `}).join('')}
        </div>
    `;

    container.innerHTML = html;
}

// Simple HTML escaping to prevent XSS
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
