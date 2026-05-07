// Onboarding wizard — manages the post-signup onboarding flow

(function () {
    "use strict";

    const TOTAL_STEPS = 3;
    let currentStep = 1;
    let householdListLoaded = false;

    // --- Step management ---

    function showStep(step) {
        document.querySelectorAll(".onboarding-step").forEach(function (el) {
            el.classList.add("d-none");
        });

        const stepEl = document.getElementById("step" + step);
        if (stepEl) {
            stepEl.classList.remove("d-none");
        }

        for (let i = 1; i <= TOTAL_STEPS; i++) {
            const indicator = document.getElementById("stepIndicator" + i);
            if (!indicator) continue;
            indicator.classList.remove("active", "completed");
            if (i < step) {
                indicator.classList.add("completed");
            } else if (i === step) {
                indicator.classList.add("active");
            }
        }

        const nextBtn = document.getElementById("btnNextStep");
        if (nextBtn) {
            nextBtn.classList.toggle("d-none", step >= TOTAL_STEPS);
        }

        currentStep = step;
    }

    function showComplete() {
        document.querySelectorAll(".onboarding-step").forEach(function (el) {
            el.classList.add("d-none");
        });
        const completeEl = document.getElementById("stepComplete");
        if (completeEl) {
            completeEl.classList.remove("d-none");
        }

        for (let i = 1; i <= TOTAL_STEPS; i++) {
            const indicator = document.getElementById("stepIndicator" + i);
            if (indicator) {
                indicator.classList.remove("active");
                indicator.classList.add("completed");
            }
        }

        const nextBtn = document.getElementById("btnNextStep");
        if (nextBtn) {
            nextBtn.classList.add("d-none");
        }
    }

    // --- Step 1: Profile ---

    async function saveProfile() {
        const input = document.getElementById("inputDisplayName");
        if (!input) return;
        const displayName = input.value.trim();
        if (!displayName) {
            proviant.showFeedback("error", "Required", "Please enter a display name.");
            return;
        }
        try {
            const response = await proviant.updateOnboardingProfile(displayName);
            if (response.code === 200) {
                showStep(2);
            } else {
                proviant.showFeedback("error", "Could Not Save", response.message || "Unknown error");
            }
        } catch (err) {
            proviant.showFeedback("error", "Network Error", err.message);
        }
    }

    // --- Step 2: Household ---

    function switchHouseholdMode(mode) {
        ["browse", "create", "invite"].forEach(function (m) {
            const el = document.getElementById("householdMode" + capitalize(m));
            if (el) {
                el.classList.toggle("d-none", m !== mode);
            }
        });
    }

    function capitalize(str) {
        return str.charAt(0).toUpperCase() + str.slice(1);
    }

    async function loadHouseholds() {
        const listEl = document.getElementById("householdList");
        const loadingEl = document.getElementById("householdListLoading");

        if (loadingEl) loadingEl.classList.remove("d-none");
        if (listEl) listEl.classList.add("d-none");

        try {
            const response = await proviant.getOnboardingHouseholds();
            if (response.code === 200 && Array.isArray(response.body)) {
                renderHouseholdList(response.body);
                householdListLoaded = true;
            } else {
                if (listEl) {
                    listEl.innerHTML = '<div class="alert alert-warning mb-0">Failed to load households.</div>';
                    listEl.classList.remove("d-none");
                }
            }
        } catch (err) {
            if (listEl) {
                listEl.innerHTML = '<div class="alert alert-danger mb-0">Network error: ' + err.message + '</div>';
                listEl.classList.remove("d-none");
            }
        } finally {
            if (loadingEl) loadingEl.classList.add("d-none");
        }
    }

    function renderHouseholdList(households) {
        const listEl = document.getElementById("householdList");
        if (!listEl) return;

        if (households.length === 0) {
            listEl.innerHTML = '<div class="list-group-item text-secondary-custom">No households available to join.</div>';
            listEl.classList.remove("d-none");
            return;
        }

        var html = "";
        households.forEach(function (h) {
            html += '<button class="list-group-item list-group-item-action household-join-btn" data-household-id="' + h.id + '">';
            html += '<div class="d-flex w-100 justify-content-between">';
            html += '<h6 class="mb-1">' + escapeHtml(h.name) + '</h6>';
            html += '<small class="text-secondary-custom">' + h.memberCount + ' member' + (h.memberCount !== 1 ? 's' : '') + '</small>';
            html += '</div>';
            if (h.description) {
                html += '<p class="mb-1 small text-secondary-custom">' + escapeHtml(h.description) + '</p>';
            }
            html += '<small class="text-primary"><i class="bi bi-box-arrow-in-right me-1"></i>Request to Join</small>';
            html += '</button>';
        });

        listEl.innerHTML = html;
        listEl.classList.remove("d-none");
    }

    function escapeHtml(text) {
        var div = document.createElement("div");
        div.appendChild(document.createTextNode(text));
        return div.innerHTML;
    }

    async function applyForHousehold(householdId) {
        try {
            const response = await proviant.applyOnboardingHousehold(householdId);
            if (response.code === 200) {
                showStep(3);
            } else {
                proviant.showFeedback("error", "Could Not Join", response.message || "Unknown error");
            }
        } catch (err) {
            proviant.showFeedback("error", "Network Error", err.message);
        }
    }

    async function createHousehold() {
        const input = document.getElementById("inputNewHouseholdName");
        if (!input) return;
        const name = input.value.trim();
        if (!name) {
            proviant.showFeedback("error", "Required", "Please enter a household name.");
            return;
        }
        try {
            const response = await proviant.createOnboardingHousehold(name);
            if (response.code === 200) {
                showStep(3);
            } else {
                proviant.showFeedback("error", "Could Not Create", response.message || "Unknown error");
            }
        } catch (err) {
            proviant.showFeedback("error", "Network Error", err.message);
        }
    }

    async function joinByInvite() {
        const input = document.getElementById("inputInviteToken");
        if (!input) return;
        const token = input.value.trim();
        if (!token) {
            proviant.showFeedback("error", "Required", "Please enter an invite token.");
            return;
        }
        try {
            const response = await proviant.joinOnboardingByInvite(token);
            if (response.code === 200) {
                showStep(3);
            } else {
                proviant.showFeedback("error", "Could Not Join", response.message || "Unknown error");
            }
        } catch (err) {
            proviant.showFeedback("error", "Network Error", err.message);
        }
    }

    // --- Step 3 (Notifications) redirect ---

    function configureNotifications() {
        window.location.href = "/web/user/settings";
    }

    // --- Complete onboarding ---

    async function completeOnboarding() {
        try {
            await proviant.completeOnboarding();
        } catch (_e) {
            // Don't block navigation on error
        }
        window.location.href = "/web/products";
    }

    // --- Restore state from server ---

    async function restoreState() {
        try {
            const response = await proviant.getOnboardingState();
            if (response.code === 200 && response.body) {
                const state = response.body;
                if (state.onboardingCompleted) {
                    window.location.href = "/web/products";
                    return;
                }
                if (!state.profileStepDone) {
                    showStep(1);
                } else if (!state.householdStepDone) {
                    showStep(2);
                } else if (!state.notificationsSetup) {
                    showStep(3);
                } else {
                    showComplete();
                }
            } else {
                showStep(1);
            }
        } catch (_e) {
            showStep(1);
        }
    }

    // --- Event delegation ---

    document.addEventListener("click", function (event) {
        const target = event.target;

        if (target.closest("#btnSaveProfile")) {
            event.preventDefault();
            saveProfile();
            return;
        }

        if (target.closest("#btnSkipProfile")) {
            event.preventDefault();
            proviant.updateOnboardingProfile("").catch(function () {});
            showStep(2);
            return;
        }

        if (target.closest("#btnHouseholdModeBrowse")) {
            event.preventDefault();
            switchHouseholdMode("browse");
            if (!householdListLoaded) {
                loadHouseholds();
            }
            return;
        }

        if (target.closest("#btnHouseholdModeCreate")) {
            event.preventDefault();
            switchHouseholdMode("create");
            return;
        }

        if (target.closest("#btnHouseholdModeInvite")) {
            event.preventDefault();
            switchHouseholdMode("invite");
            return;
        }

        if (target.closest(".household-join-btn")) {
            event.preventDefault();
            var btn = target.closest(".household-join-btn");
            var householdId = parseInt(btn.getAttribute("data-household-id"), 10);
            if (householdId) {
                applyForHousehold(householdId);
            }
            return;
        }

        if (target.closest("#btnCreateHousehold")) {
            event.preventDefault();
            createHousehold();
            return;
        }

        if (target.closest("#btnJoinByInvite")) {
            event.preventDefault();
            joinByInvite();
            return;
        }

        if (target.closest("#btnConfigureNotifications")) {
            event.preventDefault();
            configureNotifications();
            return;
        }

        if (target.closest("#btnSkipNotifications")) {
            event.preventDefault();
            showComplete();
            return;
        }

        if (target.closest("#btnFinishOnboarding")) {
            event.preventDefault();
            completeOnboarding();
            return;
        }

        if (target.closest("#btnNextStep")) {
            event.preventDefault();
            if (currentStep < TOTAL_STEPS) {
                showStep(currentStep + 1);
            }
            return;
        }
    });

    // --- Initialize ---

    document.addEventListener("DOMContentLoaded", function () {
        restoreState();
    });
})();
