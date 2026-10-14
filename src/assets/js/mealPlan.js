function updateScrollBounds() {
    const maxScroll = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
    atTop = window.scrollY <= 8;
    atBottom = window.scrollY >= maxScroll - 8;
}

document.addEventListener("DOMContentLoaded", function () {
    const container = document.getElementById("mealPlanGrid");
    if (container) {
        let wheelBuffer = 0;
        let wheelTimer = null;

        updateScrollBounds();

        window.addEventListener("scroll", updateScrollBounds, { passive: true });
        window.addEventListener("resize", updateScrollBounds, { passive: true });

        window.addEventListener("wheel", function (e) {
            if (isNavigating || Date.now() < navCooldownUntil) return;
            updateScrollBounds();
            const deltaModeFactor = e.deltaMode === 1 ? 40 : (e.deltaMode === 2 ? window.innerHeight || 800 : 1);
            const delta = (Number(e.deltaY) || 0) * deltaModeFactor;
            if (!(atBottom || atTop)) {
                wheelBuffer = 0;
                return;
            }
            if (atBottom && delta > 0) {
                wheelBuffer += delta;
                if (wheelTimer) clearTimeout(wheelTimer);
                wheelTimer = setTimeout(function () { wheelBuffer = 0; }, 1500);
                const threshold = 2500;
                if (wheelBuffer >= threshold) {
                    wheelBuffer = 0;
                    navCooldownUntil = Date.now() + 1000;
                    loadNextWindow();
                }
                return;
            }
            if (atTop && delta < 0) {
                wheelBuffer += delta;
                if (wheelTimer) clearTimeout(wheelTimer);
                wheelTimer = setTimeout(function () { wheelBuffer = 0; }, 1500);
                const threshold = 2500;
                if (wheelBuffer <= -threshold) {
                    wheelBuffer = 0;
                    navCooldownUntil = Date.now() + 1000;
                    loadPreviousWindow();
                }
                return;
            }
            wheelBuffer = 0;
        }, { passive: true });
    }
    loadMealPlanRecipes();
    loadMealPlanEntries();
});

document.addEventListener("click", function (e) {
    const target = e.target;
    if (!target || !target.closest) return;

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
        return;
    }
    if (target.closest("#btnPrevMonth")) {
        e.preventDefault();
        loadPreviousWindow();
        return;
    }
    if (target.closest("#btnNextMonth")) {
        e.preventDefault();
        loadNextWindow();
        return;
    }
    if (target.closest("#btnTodayMonth")) {
        e.preventDefault();
        resetToCurrentWindow();
        return;
    }
});

document.addEventListener("dragstart", function (e) {
    if (!e.target || !e.target.closest) return;
    if (e.target.closest("button, select, input")) {
        e.preventDefault();
        return;
    }
    const entry = e.target.closest(".mp-entry");
    if (!entry) return;
    if (e.dataTransfer) {
        e.dataTransfer.effectAllowed = "move";
        e.dataTransfer.setData("text/plain", entry.getAttribute("data-meal-id") || "");
    }
    entry.classList.add("mp-dragging");
});

document.addEventListener("dragend", function (e) {
    const entry = e.target && e.target.closest ? e.target.closest(".mp-entry") : null;
    if (entry) entry.classList.remove("mp-dragging");
    document.querySelectorAll(".mp-day.mp-drop-target").forEach(function (day) {
        day.classList.remove("mp-drop-target");
    });
});

document.addEventListener("dragover", function (e) {
    const day = e.target && e.target.closest ? e.target.closest(".mp-day") : null;
    if (!day) return;
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
    document.querySelectorAll(".mp-day.mp-drop-target").forEach(function (target) {
        if (target !== day) target.classList.remove("mp-drop-target");
    });
    day.classList.add("mp-drop-target");
});

document.addEventListener("dragleave", function (e) {
    const day = e.target && e.target.closest ? e.target.closest(".mp-day") : null;
    if (!day || day.contains(e.relatedTarget)) return;
    day.classList.remove("mp-drop-target");
});

document.addEventListener("drop", function (e) {
    const day = e.target && e.target.closest ? e.target.closest(".mp-day") : null;
    if (!day) return;
    e.preventDefault();
    day.classList.remove("mp-drop-target");
    const mealID = e.dataTransfer ? e.dataTransfer.getData("text/plain") : "";
    const targetDate = day.getAttribute("data-date");
    if (!mealID || !targetDate) return;
    const entry = document.getElementById("meal-" + parseInt(mealID, 10));
    const currentDay = entry && entry.closest ? entry.closest(".mp-day") : null;
    if (currentDay && currentDay.getAttribute("data-date") === targetDate) return;
    movePlannedMeal(parseInt(mealID, 10), targetDate);
});

document.addEventListener("change", function (e) {
    const target = e.target;
    if (!target || !target.closest) return;
    if (target.matches("[data-meal-move]")) {
        movePlannedMeal(target.getAttribute("data-meal-move"), target.value);
    }
});

let currentMonthStart = new Date(new Date().getFullYear(), new Date().getMonth(), 1);
let isNavigating = false;
let navCooldownUntil = 0;
let atTop = true;
let atBottom = false;
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

function addDays(date, days) {
    const next = new Date(date);
    next.setDate(next.getDate() + days);
    return next;
}

function planDays() {
    const days = [];
    const daysInMonth = new Date(currentMonthStart.getFullYear(), currentMonthStart.getMonth() + 1, 0).getDate();
    const firstDay = new Date(currentMonthStart.getFullYear(), currentMonthStart.getMonth(), 1);
    const offset = (firstDay.getDay() + 6) % 7;
    const monthStartGrid = addDays(firstDay, -offset);
    const lastDay = new Date(currentMonthStart.getFullYear(), currentMonthStart.getMonth(), daysInMonth);
    const tailOffset = (7 - lastDay.getDay()) % 7;
    const monthEndGrid = addDays(lastDay, tailOffset);
    const totalDays = Math.round((monthEndGrid - monthStartGrid) / 86400000) + 1;
    for (let i = 0; i < totalDays; i++) {
        days.push(addDays(monthStartGrid, i));
    }
    return days;
}

function planRange() {
    const days = planDays();
    return { from: toISODate(days[0]), to: toISODate(days[days.length - 1]) };
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
    const previousValue = select.value;
    const days = planDays();
    const isoDays = days.map(toISODate);
    select.innerHTML = days.map(function (day, index) {
        const label = day.toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" });
        return `<option value="${isoDays[index]}">${escapeMealHtml(label)}</option>`;
    }).join("");
    const todayISO = toISODate(new Date());
    const firstOfMonthISO = toISODate(currentMonthStart);
    if (previousValue && isoDays.indexOf(previousValue) !== -1) {
        select.value = previousValue;
    } else if (isoDays.indexOf(todayISO) !== -1) {
        select.value = todayISO;
    } else if (isoDays.indexOf(firstOfMonthISO) !== -1) {
        select.value = firstOfMonthISO;
    }
}

async function loadMealPlanEntries() {
    const grid = document.getElementById("mealPlanGrid");
    if (!grid) return;
    const label = document.getElementById("monthLabel");
    if (label) label.textContent = currentMonthStart.toLocaleDateString(undefined, { month: "long", year: "numeric" });
    renderMealPlanDaySelect();
    try {
        const range = planRange();
        const res = await proviant.getMealPlans(range.from, range.to);
        if (res.code !== 200) {
            proviant.showFeedback("error", "Meal plan not loaded", res.message || `Request failed with status ${res.code}`);
            renderMealPlanEntries([]);
            return;
        }
        renderMealPlanEntries(Array.isArray(res.entries) ? res.entries : []);
    } catch (error) {
        proviant.showFeedback("error", "Meal plan not loaded", error.message);
        renderMealPlanEntries([]);
    }
}

function mealPlanEntryHtml(entry, day) {
    const days = planDays();
    const moveOptions = days.map(function (candidate) {
        const selected = toISODate(candidate) === toISODate(day) ? " selected" : "";
        const label = candidate.toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" });
        return `<option value="${toISODate(candidate)}"${selected}>${escapeMealHtml(label)}</option>`;
    }).join("");
    return `
        <div class="mp-entry" id="meal-${entry.id}" data-meal-id="${entry.id}" data-meal-servings="${entry.servings}" draggable="true">
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
    const allDays = planDays();
    const todayISO = toISODate(new Date());
    const weeks = [];
    for (let offset = 0; offset < allDays.length; offset += 7) {
        weeks.push(allDays.slice(offset, offset + 7));
    }
    grid.innerHTML = weeks.map(function (weekDays, weekIndex) {
        const weekStart = weekDays[0];
        const weekEnd = weekDays[weekDays.length - 1];
        const weekLabel = `${weekStart.toLocaleDateString(undefined, { month: "short", day: "numeric" })} – ${weekEnd.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}`;
        const dayCards = weekDays.map(function (day) {
            const dayISO = toISODate(day);
            const dayEntries = (entries || []).filter(function (entry) {
                return entry.date === dayISO;
            });
            const header = day.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });
            const isToday = dayISO === todayISO;
            const isPast = dayISO < todayISO;
            const isOtherMonth = day.getMonth() !== currentMonthStart.getMonth() || day.getFullYear() !== currentMonthStart.getFullYear();
            const isWeekend = day.getDay() === 0 || day.getDay() === 6;
            return `
                <section class="mp-day${isToday ? " mp-day-current" : ""}${isPast || isOtherMonth ? " mp-day-past" : ""}${isWeekend ? " mp-day-weekend" : ""}" data-date="${dayISO}" aria-label="${escapeMealHtml(header)}">
                    <div class="mp-day-header">
                        <span class="${isToday ? "mp-day-today" : ""}">${escapeMealHtml(header)}</span>
                    </div>
                    ${dayEntries.length > 0
                        ? dayEntries.map(function (entry) { return mealPlanEntryHtml(entry, day); }).join("")
                        : '<div class="mp-entry-empty">—</div>'}
                </section>`;
        }).join("");
        return `
            <section class="mp-week${weekIndex % 2 === 1 ? " mp-week-alt" : ""}" aria-label="${escapeMealHtml(weekLabel)}">
                <h2 class="mp-week-header">${escapeMealHtml(weekLabel)}</h2>
                <div class="mp-grid">
                    ${dayCards}
                </div>
            </section>`;
    }).join("");
    updateScrollBounds();
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

function resetMonthView() {
    if (!/jsdom/i.test((navigator && navigator.userAgent) || "")) {
        window.scrollTo(0, 0);
    }
    updateScrollBounds();
}

async function loadPreviousWindow() {
    if (isNavigating) return;
    isNavigating = true;
    try {
        currentMonthStart = new Date(currentMonthStart.getFullYear(), currentMonthStart.getMonth() - 1, 1);
        await loadMealPlanEntries();
        resetMonthView();
    } finally {
        isNavigating = false;
    }
}

async function loadNextWindow() {
    if (isNavigating) return;
    isNavigating = true;
    try {
        currentMonthStart = new Date(currentMonthStart.getFullYear(), currentMonthStart.getMonth() + 1, 1);
        await loadMealPlanEntries();
        resetMonthView();
    } finally {
        isNavigating = false;
    }
}

async function resetToCurrentWindow() {
    if (isNavigating) return;
    isNavigating = true;
    try {
        currentMonthStart = new Date(new Date().getFullYear(), new Date().getMonth(), 1);
        await loadMealPlanEntries();
        resetMonthView();
    } finally {
        isNavigating = false;
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
        const range = planRange();
        const listRes = await proviant.getMealPlans(range.from, range.to);
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
        const firstDayISO = toISODate(currentMonthStart);
        const lastDay = new Date(currentMonthStart.getFullYear(), currentMonthStart.getMonth() + 1, 0);
        const lastDayISO = toISODate(lastDay);
        const todayISO = toISODate(new Date());
        const fromISO = firstDayISO < todayISO && todayISO <= lastDayISO ? todayISO : firstDayISO;
        const res = await proviant.generateShoppingListFromMealPlan(fromISO, lastDayISO);
        if (res.code !== 200) {
            proviant.showFeedback("error", "Shopping list not generated", res.message || `Request failed with status ${res.code}`);
            return;
        }
        proviant.showFeedback("success", "Shopping list updated", `${res.message || "Items added"}.`);
    } catch (error) {
        proviant.showFeedback("error", "Shopping list not generated", error.message);
    }
}
