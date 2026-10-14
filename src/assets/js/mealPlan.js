document.addEventListener("DOMContentLoaded", function () {
    loadMealPlanRecipes();
    loadMealPlanEntries();
});

document.addEventListener("click", function (e) {
    const target = e.target;
    if (!target || !target.closest) return;

    if (target.closest("#btnPrevWeek")) {
        e.preventDefault();
        shiftMealPlanWeek(-7);
        return;
    }
    if (target.closest("#btnNextWeek")) {
        e.preventDefault();
        shiftMealPlanWeek(7);
        return;
    }
    if (target.closest("#btnAddMeal")) {
        e.preventDefault();
        createMealPlanEntry();
        return;
    }
    if (target.closest("#btnGenerateShoppingList") || target.closest("#btnGenerateShoppingListMobile")) {
        e.preventDefault();
        generateShoppingList();
        return;
    }
    const servingsButton = target.closest("[data-meal-servings-action]");
    if (servingsButton) {
        e.preventDefault();
        changeMealServings(servingsButton);
        return;
    }
    const cookButton = target.closest("[data-meal-cook]");
    if (cookButton) {
        e.preventDefault();
        confirmCookPlannedMeal(cookButton.getAttribute("data-meal-cook"), cookButton.getAttribute("data-recipe-name") || "this meal");
        return;
    }
    const deleteButton = target.closest("[data-meal-delete]");
    if (deleteButton) {
        e.preventDefault();
        confirmDeletePlannedMeal(deleteButton.getAttribute("data-meal-delete"), deleteButton.getAttribute("data-recipe-name") || "this meal");
    }
});

document.addEventListener("change", function (e) {
    const target = e.target;
    if (!target || !target.closest) return;
    if (target.matches("[data-meal-move]")) {
        movePlannedMeal(target.getAttribute("data-meal-move"), target.value);
    }
});

let mealPlanWeekStart = startOfWeek(new Date());
let mealPlanRecipes = [];

function escapeMealHtml(text) {
    if (!text) return "";
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

function toISODate(date) {
    const month = `${date.getMonth() + 1}`.padStart(2, "0");
    const day = `${date.getDate()}`.padStart(2, "0");
    return `${date.getFullYear()}-${month}-${day}`;
}

function startOfWeek(date) {
    const start = new Date(date.getFullYear(), date.getMonth(), date.getDate());
    const weekday = (start.getDay() + 6) % 7;
    start.setDate(start.getDate() - weekday);
    return start;
}

function addDays(date, days) {
    const next = new Date(date);
    next.setDate(next.getDate() + days);
    return next;
}

function shiftMealPlanWeek(days) {
    mealPlanWeekStart = addDays(mealPlanWeekStart, days);
    loadMealPlanEntries();
}

function weekDays() {
    const days = [];
    for (let i = 0; i < 7; i++) {
        days.push(addDays(mealPlanWeekStart, i));
    }
    return days;
}

function weekRangeLabel() {
    const days = weekDays();
    const startLabel = days[0].toLocaleDateString(undefined, { month: "short", day: "numeric" });
    const endLabel = days[6].toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
    return `${startLabel} – ${endLabel}`;
}

async function loadMealPlanRecipes() {
    try {
        const res = await proviant.getRecipes();
        mealPlanRecipes = res.code === 200 && Array.isArray(res.recipes) ? res.recipes : [];
    } catch (_error) {
        mealPlanRecipes = [];
    }
    const select = document.getElementById("inputMealRecipe");
    if (!select) return;
    if (mealPlanRecipes.length === 0) {
        select.innerHTML = '<option value="">No saved recipes</option>';
        return;
    }
    select.innerHTML = mealPlanRecipes.map(function (recipe) {
        return `<option value="${recipe.id}">${escapeMealHtml(recipe.name)}</option>`;
    }).join("");
}

function renderMealPlanDaySelect() {
    const select = document.getElementById("inputMealDate");
    if (!select) return;
    select.innerHTML = weekDays().map(function (day) {
        const label = day.toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" });
        return `<option value="${toISODate(day)}">${escapeMealHtml(label)}</option>`;
    }).join("");
}

async function loadMealPlanEntries() {
    const grid = document.getElementById("mealPlanGrid");
    if (!grid) return;
    const label = document.getElementById("weekLabel");
    if (label) label.textContent = weekRangeLabel();
    renderMealPlanDaySelect();
    try {
        const res = await proviant.getMealPlans(toISODate(weekDays()[0]), toISODate(weekDays()[6]));
        if (res.code !== 200) {
            proviant.showFeedback("error", "Meal plan not loaded", res.message || `Request failed with status ${res.code}`);
            renderMealPlanEntries([]);
            return;
        }
        renderMealPlanEntries(res.entries);
    } catch (error) {
        proviant.showFeedback("error", "Meal plan not loaded", error.message);
        renderMealPlanEntries([]);
    }
}

function mealPlanEntryHtml(entry, day) {
    const days = weekDays();
    const moveOptions = days.map(function (candidate) {
        const selected = toISODate(candidate) === toISODate(day) ? " selected" : "";
        const label = candidate.toLocaleDateString(undefined, { weekday: "short" });
        return `<option value="${toISODate(candidate)}"${selected}>${escapeMealHtml(label)}</option>`;
    }).join("");
    return `
        <div class="mp-entry" id="meal-${entry.id}" data-meal-servings="${entry.servings}">
            <div class="mp-entry-name">${escapeMealHtml(entry.recipeName || `Recipe #${entry.recipeId}`)}</div>
            <div class="mp-entry-meta" id="meal-servings-${entry.id}">${entry.servings} serving(s)</div>
            <div class="mp-entry-actions">
                <div class="qty-stepper">
                    <button type="button" data-meal-servings-action="dec" data-meal-id="${entry.id}" aria-label="Decrease servings of ${escapeMealHtml(entry.recipeName)}">−</button>
                    <button type="button" data-meal-servings-action="inc" data-meal-id="${entry.id}" aria-label="Increase servings of ${escapeMealHtml(entry.recipeName)}">+</button>
                </div>
                <select class="form-select form-select-sm" data-meal-move="${entry.id}" aria-label="Move ${escapeMealHtml(entry.recipeName)} to another day">
                    ${moveOptions}
                </select>
                <button type="button" class="btn-icon" data-meal-cook="${entry.id}" data-recipe-name="${escapeMealHtml(entry.recipeName)}" title="Cook" aria-label="Cook ${escapeMealHtml(entry.recipeName)}">
                    <i class="bi bi-fire" aria-hidden="true"></i>
                </button>
                <button type="button" class="btn-icon text-danger" data-meal-delete="${entry.id}" data-recipe-name="${escapeMealHtml(entry.recipeName)}" title="Delete" aria-label="Delete ${escapeMealHtml(entry.recipeName)}">
                    <i class="bi bi-trash3" aria-hidden="true"></i>
                </button>
            </div>
        </div>`;
}

function renderMealPlanEntries(entries) {
    const grid = document.getElementById("mealPlanGrid");
    if (!grid) return;
    const days = weekDays();
    const todayISO = toISODate(new Date());
    grid.innerHTML = days.map(function (day) {
        const dayISO = toISODate(day);
        const dayEntries = (entries || []).filter(function (entry) {
            return entry.date === dayISO;
        });
        const header = day.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });
        return `
            <section class="mp-day" aria-label="${escapeMealHtml(header)}">
                <div class="mp-day-header">
                    <span class="${dayISO === todayISO ? "mp-day-today" : ""}">${escapeMealHtml(header)}</span>
                </div>
                ${dayEntries.length > 0
                    ? dayEntries.map(function (entry) { return mealPlanEntryHtml(entry, day); }).join("")
                    : '<div class="mp-entry-empty">No meals planned</div>'}
            </section>`;
    }).join("");
}

async function createMealPlanEntry() {
    const recipeSelect = document.getElementById("inputMealRecipe");
    const dateSelect = document.getElementById("inputMealDate");
    const servingsInput = document.getElementById("inputMealServings");
    if (!recipeSelect || !dateSelect) return;
    if (!recipeSelect.value) {
        proviant.showFeedback("warning", "No recipe selected", "Save a recipe first, then plan it here.");
        return;
    }
    try {
        const res = await proviant.createMealPlan({
            date: dateSelect.value,
            recipeId: parseInt(recipeSelect.value, 10),
            servings: parseInt(servingsInput.value, 10) || 1,
            notes: "",
        });
        if (res.code !== 201) {
            proviant.showFeedback("error", "Meal not planned", res.message || `Request failed with status ${res.code}`);
            return;
        }
        loadMealPlanEntries();
    } catch (error) {
        proviant.showFeedback("error", "Meal not planned", error.message);
    }
}

async function changeMealServings(button) {
    const mealID = parseInt(button.getAttribute("data-meal-id"), 10);
    const entry = document.getElementById("meal-" + mealID);
    // The entry element carries the last known servings and is bumped
    // optimistically, so rapid ± clicks stack instead of racing each other
    // with a stale pre-render value.
    const current = entry ? (parseInt(entry.getAttribute("data-meal-servings"), 10) || 1) : 1;
    const next = button.getAttribute("data-meal-servings-action") === "inc" ? current + 1 : Math.max(1, current - 1);
    if (next === current) return;
    const label = document.getElementById("meal-servings-" + mealID);
    const revert = function () {
        if (entry) entry.setAttribute("data-meal-servings", String(current));
        if (label) label.textContent = current + " serving(s)";
    };
    if (entry) entry.setAttribute("data-meal-servings", String(next));
    if (label) label.textContent = next + " serving(s)";
    try {
        const res = await proviant.updateMealPlan(mealID, { servings: next });
        if (res.code !== 200) {
            revert();
            proviant.showFeedback("error", "Servings not changed", res.message || `Request failed with status ${res.code}`);
            return;
        }
        loadMealPlanEntries();
    } catch (error) {
        revert();
        proviant.showFeedback("error", "Servings not changed", error.message);
    }
}

async function movePlannedMeal(mealID, date) {
    try {
        const res = await proviant.updateMealPlan(parseInt(mealID, 10), { date: date });
        if (res.code !== 200) {
            proviant.showFeedback("error", "Meal not moved", res.message || `Request failed with status ${res.code}`);
            return;
        }
        loadMealPlanEntries();
    } catch (error) {
        proviant.showFeedback("error", "Meal not moved", error.message);
    }
}

function confirmDeletePlannedMeal(mealID, recipeName) {
    proviant.showConfirm(
        "Remove meal",
        `This will remove "${recipeName}" from the meal plan.`,
        function () { deletePlannedMeal(mealID); },
        "Remove",
        "danger"
    );
}

async function deletePlannedMeal(mealID) {
    try {
        const res = await proviant.deleteMealPlan(parseInt(mealID, 10));
        if (res.code !== 200) {
            proviant.showFeedback("error", "Meal not removed", res.message || `Request failed with status ${res.code}`);
            return;
        }
        loadMealPlanEntries();
    } catch (error) {
        proviant.showFeedback("error", "Meal not removed", error.message);
    }
}

function confirmCookPlannedMeal(mealID, recipeName) {
    proviant.showConfirm(
        "Cook meal",
        `This will consume the linked ingredients of "${recipeName}" from your stock. This cannot be undone.`,
        function () { cookPlannedMeal(mealID); },
        "Cook",
        "warning"
    );
}

async function cookPlannedMeal(mealID) {
    try {
        const listRes = await proviant.getMealPlans(toISODate(weekDays()[0]), toISODate(weekDays()[6]));
        const entry = (listRes.entries || []).find(function (candidate) {
            return candidate.id === parseInt(mealID, 10);
        });
        if (!entry) {
            proviant.showFeedback("error", "Cook failed", "The planned meal no longer exists.");
            return;
        }
        const res = await proviant.cookRecipe(entry.recipeId, entry.servings);
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
            proviant.showFeedback("success", "Meal cooked", `${consumed} product(s) fully consumed, ${partial} partially.`);
        } else {
            proviant.showFeedback("success", "Meal cooked", `${consumed} product(s) marked as fully consumed.`);
        }
    } catch (error) {
        proviant.showFeedback("error", "Cook failed", error.message);
    }
}

async function generateShoppingList() {
    try {
        const res = await proviant.generateShoppingListFromMealPlan(toISODate(weekDays()[0]), toISODate(weekDays()[6]));
        if (res.code !== 200) {
            proviant.showFeedback("error", "Shopping list not generated", res.message || `Request failed with status ${res.code}`);
            return;
        }
        proviant.showFeedback("success", "Shopping list updated", `${res.message || "Items added"}.`);
    } catch (error) {
        proviant.showFeedback("error", "Shopping list not generated", error.message);
    }
}
