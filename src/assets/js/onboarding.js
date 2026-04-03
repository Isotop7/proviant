// Onboarding wizard — manages the post-signup onboarding flow

(function () {
    "use strict";

    const TOTAL_STEPS = 2;
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


    // --- Step 1: Notifications ---

    function configureNotifications() {
        window.location.href = "/web/user/settings";
    }

    // --- Step 2: Household selection ---

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
            listEl.innerHTML = '<div class="list-group-item text-muted">No households available to join.</div>';
            listEl.classList.remove("d-none");
            return;
        }

        var html = "";
        households.forEach(function (h) {
            html += '<button class="list-group-item list-group-item-action household-join-btn" data-household-id="' + h.id + '">';
            html += '<div class="d-flex w-100 justify-content-between">';
            html += '<h6 class="mb-1">' + escapeHtml(h.name) + '</h6>';
            html += '<small class="text-muted">' + h.memberCount + ' member' + (h.memberCount !== 1 ? 's' : '') + '</small>';
            html += '</div>';
            if (h.description) {
                html += '<p class="mb-1 small text-muted">' + escapeHtml(h.description) + '</p>';
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
                showComplete();
            } else {
                proviant.showFeedback('error', 'Could Not Join', response.message || 'Unknown error');
            }
        } catch (err) {
            proviant.showFeedback('error', 'Network Error', err.message);
        }
    }

    // --- Complete onboarding ---

    async function completeOnboarding() {
        try {
            await proviant.completeOnboarding();
        } catch (err) {
            // Don't block navigation on error
        }
        window.location.href = "/web";
    }

    function skipOnboarding() {
        window.location.href = "/web";
    }

    // --- Restore state from server ---

    async function restoreState() {
        try {
            const response = await proviant.getOnboardingState();
            if (response.code === 200 && response.body) {
                const state = response.body;
                if (state.onboardingCompleted) {
                    window.location.href = "/web";
                    return;
                }
                if (state.notificationsSetup) {
                    if (state.householdStepDone) {
                        showComplete();
                    } else {
                        showStep(2);
                    }
                } else {
                    showStep(1);
                }
            } else {
                showStep(1);
            }
        } catch (err) {
            showStep(1);
        }
    }

    // --- Event delegation ---

    document.addEventListener("click", function (event) {
        const target = event.target;

        if (target.closest("#btnConfigureNotifications")) {
            event.preventDefault();
            configureNotifications();
            return;
        }

        if (target.closest("#btnSkipNotifications")) {
            event.preventDefault();
            showStep(2);
            return;
        }

        if (target.closest("#btnLoadHouseholds")) {
            event.preventDefault();
            if (!householdListLoaded) {
                loadHouseholds();
            }
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

        if (target.closest("#btnSkipHousehold")) {
            event.preventDefault();
            showComplete();
            return;
        }

        if (target.closest("#btnFinishOnboarding")) {
            event.preventDefault();
            completeOnboarding();
            return;
        }

        if (target.closest("#btnSkipOnboarding")) {
            event.preventDefault();
            skipOnboarding();
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
