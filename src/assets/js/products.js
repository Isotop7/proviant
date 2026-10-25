/* exported changeQty */

const pe = {
    editProduct: null,
    searchQuery: null,
    sortParam: null,
    sortOrder: null,
    locationFilter: null,
    bulkActions: null,
    bulkCount: null,
    mobileBulkActions: null,
    mobileBulkCount: null,
    selectedAll: null,
};
// NOTE: Do not freeze `pe` — its properties are assigned inside the
// DOMContentLoaded handler below. A frozen object makes those assignments
// silent no-ops in sloppy mode and breaks bulk selection entirely.

// A delta the offline outbox could not store is the user's change disappearing
// with nothing to retry, so it has to reach them — a console line in a failed
// enqueue is invisible on a phone. Every other failure keeps its existing
// console-only handling: the delta either reached the server or is queued.
function reportQuantityChangeFailed(response) {
    if (response.queued === false) {
        proviant.showFeedback('error', response.message);
        return;
    }
    console.error(response.message);
}

// readQuantity reads the pack count of a quantity label. The count lives in
// the data attribute; the text is the rendered label ("2 Stk.") and must not
// be parsed, because a locale that does not separate the number from the noun
// would yield NaN.
function readQuantity(element) {
    if (!element) return 0;
    const parsed = Number.parseInt(element.dataset.quantity, 10);
    return Number.isFinite(parsed) ? parsed : 0;
}

function handleCardClickEffect(cardId) {
    const card = document.getElementById(cardId);
    if (card) {
        card.classList.add('card-clicked');
        setTimeout(() => card.classList.remove('card-clicked'), 100);
    }
}

async function handleSelected() {
    const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
    if (pe.editProduct) {
        pe.editProduct.disabled = selectedProducts.length !== 1;
    }
    updateBulkSelected();
}

/* Event delegation for clicks */
document.addEventListener("click", function (event) {
    const target = event.target;

    // Edit product button
    if (target.closest("#edit-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => +checkbox.value);
        if (selectedProducts.length === 1) {
            window.location.href = `/web/products/${selectedProducts[0]}/edit`;
        }
        return;
    }

    // Delete (Wasted) product button
    // Quantity increment button
    if (target.closest(".btn-qty-inc")) {
        event.preventDefault();
        event.stopPropagation();
        const btn = target.closest(".btn-qty-inc");
        const productID = btn.dataset.productId;
        const qtyEl = document.getElementById(`qty-${productID}`);
        proviant.updateProductQuantity(productID, 1).then((response) => {
            if (response.code === 200) {
                if (qtyEl) {
                    qtyEl.dataset.quantity = String(readQuantity(qtyEl) + 1);
                    proviant.renderQuantityLabel(qtyEl);
                    qtyEl.classList.add('qty-updated');
                    setTimeout(() => qtyEl.classList.remove('qty-updated'), 800);
                }
            } else {
                reportQuantityChangeFailed(response);
            }
        }).catch((err) => proviant.showFeedback('error', proviant.t("common.error"), err.message));
        return;
    }

    // Quantity decrement button
    if (target.closest(".btn-qty-dec")) {
        event.preventDefault();
        event.stopPropagation();
        const btn = target.closest(".btn-qty-dec");
        const productID = btn.dataset.productId;
        const qtyEl = document.getElementById(`qty-${productID}`);
        proviant.updateProductQuantity(productID, -1).then((response) => {
            if (response.code === 200) {
                if (response.deleted) {
                    const card = document.getElementById(`card-${productID}`);
                    if (card) card.closest(".col").remove();
                } else {
                    if (qtyEl) {
                        const newVal = Math.max(0, readQuantity(qtyEl) - 1);
                        qtyEl.dataset.quantity = String(newVal);
                        proviant.renderQuantityLabel(qtyEl);
                        qtyEl.classList.add('qty-updated');
                        setTimeout(() => qtyEl.classList.remove('qty-updated'), 800);
                    }
                }
            } else {
                reportQuantityChangeFailed(response);
            }
        }).catch((err) => proviant.showFeedback('error', proviant.t("common.error"), err.message));
        return;
    }

    // Mark as opened button
    if (target.closest(".btn-mark-opened")) {
        event.preventDefault();
        event.stopPropagation();
        const btn = target.closest(".btn-mark-opened");
        const productID = btn.dataset.productId;
        if (!productID) return;
        const sendOpen = (force) => proviant.markProductOpened(productID, force).then((response) => {
            if (response.code === 200) {
                // Reload to re-render card with opened badge and updated effective expiry
                clearBulkSelectionAndReload();
            } else if (response.conflict && response.openedAt) {
                const previous = new Date(response.openedAt);
                const formatted = Number.isNaN(previous.getTime()) ? response.openedAt : previous.toLocaleString();
                proviant.showConfirm(
                    proviant.t("js.products.alreadyOpened"),
                    proviant.t("js.products.resetOpenedConfirm", formatted),
                    () => sendOpen(true),
                    proviant.t("js.products.reset"),
                    'warning'
                );
            } else {
                proviant.showFeedback('error', proviant.t("js.products.markOpenedFailed"), proviant.t("js.products.pleaseTryAgain"));
            }
        }).catch((err) => proviant.showFeedback('error', proviant.t("js.products.markOpenedFailed"), err.message));
        void sendOpen(false);
        return;
    }

    var partialActionBtn = target.closest('.btn-partial-action');
    if (partialActionBtn) {
        event.preventDefault();
        event.stopPropagation();
        void openPartialActionModal(partialActionBtn.dataset.productId, partialActionBtn);
        return;
    }

    // Card click
    const card = target.closest('.card');
    if (card) {
        const cardId = card.id;
        const productId = cardId.split('-')[1];
        const checkbox = document.getElementById(`checkbox-${productId}`);
        if (checkbox) {
            checkbox.checked = !checkbox.checked;
            card.classList.toggle('border-info');
            void handleSelected();
            handleCardClickEffect(cardId);
        }
        return;
    }

    // Search field selector
    var scopeToggle = target.closest(".search-scope__toggle");
    if (scopeToggle) {
        event.preventDefault();
        selectSearchScope(scopeToggle.dataset.scope);
        performSearch();
        return;
    }

    // Clear the search
    if (target.closest("#search-clear")) {
        event.preventDefault();
        if (pe.searchQuery) pe.searchQuery.value = "";
        performSearch();
        return;
    }

    // Add to shopping list button
    var addToListBtn = target.closest('.btn-add-to-shopping-list');
    if (addToListBtn) {
        event.preventDefault();
        var productId = addToListBtn.dataset.productId;
        void proviant.runWithButtonBusyState(
            addToListBtn,
            async function () {
                const res = await fetch('/api/v1/shopping-list', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ productId: Number.parseInt(productId, 10) })
                });
                if (!res.ok) {
                    const body = await res.json().catch(function () { return {}; });
                    throw new Error(body.message || proviant.t("js.products.addToShoppingListFailed"));
                }
            },
            proviant.t("js.products.addedToShoppingList"),
            function (err) {
                proviant.showFeedback('error', proviant.t("js.products.failedToAddToList"), err.message);
            }
        );
        return;
    }

    // Show All button
    if (target.closest("#show-all-btn")) {
        event.preventDefault();
        window.location.href = "/web/products";
        return;
    }

    // Export products CSV
    if (target.closest("#export-products-csv")) {
        event.preventDefault();
        proviant.exportProductsCSV();
        return;
    }

    // Export products JSON
    if (target.closest("#export-products-json")) {
        event.preventDefault();
        proviant.exportProductsJSON();
        return;
    }

    // Export archive CSV
    if (target.closest("#export-archive-csv")) {
        event.preventDefault();
        proviant.exportArchiveCSV();
        return;
    }

    // Export full JSON
    if (target.closest("#export-full-json")) {
        event.preventDefault();
        proviant.exportFullJSON();
        return;
    }

    // Mobile export actions
    if (target.closest("#mobile-export-csv")) {
        event.preventDefault();
        proviant.exportProductsCSV();
        return;
    }
    if (target.closest("#mobile-export-json")) {
        event.preventDefault();
        proviant.exportProductsJSON();
        return;
    }
    if (target.closest("#mobile-archive-csv")) {
        event.preventDefault();
        proviant.exportArchiveCSV();
        return;
    }
    if (target.closest("#mobile-export-full")) {
        event.preventDefault();
        proviant.exportFullJSON();
        return;
    }

    // Bulk action buttons (data-bulk-action attribute)
    var bulkBtn = target.closest('[data-bulk-action]');
    if (bulkBtn) {
        event.preventDefault();
        var ids = getSelectedIDs();
        if (!ids.length) return;
        var action = bulkBtn.dataset.bulkAction;
        if (action === 'cook') {
            void openCookModal(ids, bulkBtn);
            return;
        }
        var opts = action === 'delete' ? { confirm: true, confirmTitle: proviant.t("js.products.markAsWasted"), confirmMsg: proviant.tn("js.products.markWastedBulkConfirm", ids.length) } : {};
        void proviant.bulkAction(action, ids, opts);
        return;
    }

    // Cook modal confirm button
    if (target.closest('#btnCookConfirm')) {
        event.preventDefault();
        void submitCook();
        return;
    }

    if (target.closest('#btnPartialConsumeConfirm')) {
        event.preventDefault();
        void submitPartialAction('consume');
        return;
    }
    if (target.closest('#btnPartialWasteConfirm')) {
        event.preventDefault();
        void submitPartialAction('waste');
        return;
    }

    // List-view qty stepper buttons
    var qtyBtn = target.closest('[data-qty-action]');
    if (qtyBtn) {
        event.preventDefault();
        var id = Number.parseInt(qtyBtn.dataset.productId, 10);
        var delta = qtyBtn.dataset.qtyAction === 'inc' ? 1 : -1;
        changeQty(id, delta);
        return;
    }

    // Focus-target dpeegator (click wrapper div to focus input)
    var focusDiv = target.closest('[data-focus-target]');
    if (focusDiv) {
        var targetEl = document.getElementById(focusDiv.dataset.focusTarget);
        if (targetEl) targetEl.focus();
    }
});

document.addEventListener('focusout', function (event) {
    if (event.target.id === 'partialActionQuantity') validatePartialActionQuantity();
});

/* Event delegation for checkbox changes */
document.addEventListener("change", function (event) {
    var target = event.target;
    if (!target.matches('input[type="checkbox"]')) return;

    if (target.id === 'selectAll') {
        var checked = target.checked;
        document.querySelectorAll('.row-checkbox').forEach(function (cb) {
            cb.checked = checked;
        });
        updateBulkSelected();
        return;
    }

    if (target.matches('.row-checkbox')) {
        updateBulkSelected();
        return;
    }

    var cardId = 'card-' + target.id.split('-')[1];
    var card = document.getElementById(cardId);
    var checkbox = document.getElementById('checkbox-' + target.id.split('-')[1]);
    if (card) {
        card.classList.toggle('border-info');
        if (checkbox) {
            checkbox.checked = !checkbox.checked;
        }
    }
});

/* Event delegation for keypress (search input Enter key) */
document.addEventListener("keypress", function (event) {
    const target = event.target;
    if (target.id === "search-query" && event.key === "Enter") {
        event.preventDefault();
        performSearch();
    }
});

// The field selector is a pair of toggles inside the search field; the
// pressed one names the field the query runs against.
function activeSearchScope() {
    const pressed = document.querySelector('.search-scope__toggle[aria-pressed="true"]');
    return pressed ? pressed.dataset.scope : "product_name";
}

function selectSearchScope(scope) {
    document.querySelectorAll('.search-scope__toggle').forEach(function (el) {
        el.setAttribute('aria-pressed', String(el.dataset.scope === scope));
        el.classList.toggle('active', el.dataset.scope === scope);
    });
}

// Function to perform search
function performSearch() {
    const params = {
        queryParam: activeSearchScope(),
        queryValue: pe.searchQuery ? pe.searchQuery.value || "" : "",
        sort: pe.sortParam ? pe.sortParam.value || "created_at" : "created_at",
        order: pe.sortOrder ? pe.sortOrder.value || "asc" : "asc",
    };
    if (pe.locationFilter && pe.locationFilter.value) {
        params.locationId = pe.locationFilter.value;
    }

    showSkeleton();
  window.location.href = `/web/products?${new URLSearchParams(params).toString()}`;
}

/* ── Add-product modal ───────────────────────────────────────────────────────
   Bootstrap is loaded after content scripts, so init must wait for
   DOMContentLoaded before calling bootstrap.Dialog.
─────────────────────────────────────────────────────────────────────────── */
document.addEventListener('DOMContentLoaded', function () {
    const modalEl = document.getElementById('addProductModal');
    if (!modalEl) return;

    const bsModal         = new bootstrap.Dialog(modalEl);
    const barcodeInput    = document.getElementById('barcode');
    const productData     = document.getElementById('productData');
    const inputWrap       = document.querySelector('#modalStepScan .barcode-input-wrap');
    const subtitle        = document.getElementById('addProductModalSubtitle');
    const btnScan         = document.getElementById('btnScan');
    const tabManualInput  = document.getElementById('tabManualInput');
    const tabCamera       = document.getElementById('tabCamera');
    const manualSection   = document.getElementById('manualInputSection');
    const cameraSection   = document.getElementById('cameraSection');
    const editProduct     = document.getElementById('edit-product');
    const searchQuery     = document.getElementById('search-query');
    const sortParam       = document.getElementById('sort-param');
    const sortOrder       = document.getElementById('sort-order');
    const locationFilter  = document.getElementById('location-filter');
    const bulkActions     = document.getElementById('bulkActions');
    const bulkCount       = document.getElementById('bulkCount');
    const mobileBulkActions = document.getElementById('mobileBulkActions');
    const mobileBulkCount = document.getElementById('mobileBulkCount');
    const selectedAll       = document.getElementById('selectAll');

    pe.editProduct = editProduct;
    pe.searchQuery = searchQuery;
    pe.sortParam = sortParam;
    pe.sortOrder = sortOrder;
    pe.locationFilter = locationFilter;
    pe.bulkActions = bulkActions;
    pe.bulkCount = bulkCount;
    pe.mobileBulkActions = mobileBulkActions;
    pe.mobileBulkCount = mobileBulkCount;
    pe.selectedAll = selectedAll;

    let modalIsOpen = false;

    function cap(s) { return s.charAt(0).toUpperCase() + s.slice(1); }

    function isCameraRunning() {
        return btnScan && btnScan.dataset.action === 'stop';
    }

    function stopCameraIfRunning() {
        if (isCameraRunning()) btnScan.click();
    }

    function setActiveTab(mode) {
        const isCamera = mode === 'camera';
        if (tabManualInput) {
            tabManualInput.style.background  = isCamera ? 'transparent' : 'white';
            tabManualInput.style.boxShadow   = isCamera ? 'none' : '0 1px 3px color-mix(in oklab, var(--fg) 10%, transparent)';
            tabManualInput.style.color       = isCamera ? 'var(--meta)' : 'var(--fg)';
        }
        if (tabCamera) {
            tabCamera.style.background  = isCamera ? 'white' : 'transparent';
            tabCamera.style.boxShadow   = isCamera ? '0 1px 3px color-mix(in oklab, var(--fg) 10%, transparent)' : 'none';
            tabCamera.style.color       = isCamera ? 'var(--fg)' : 'var(--meta)';
        }
        if (manualSection) manualSection.style.display = isCamera ? 'none' : 'block';
        if (cameraSection) cameraSection.style.display = isCamera ? 'block' : 'none';

        if (isCamera) {
            if (!isCameraRunning() && btnScan) btnScan.click();
        } else {
            stopCameraIfRunning();
            if (barcodeInput) barcodeInput.focus();
        }
    }

    if (tabManualInput) tabManualInput.addEventListener('click', function () { setActiveTab('manual'); });
    if (tabCamera)      tabCamera.addEventListener('click',      function () { setActiveTab('camera'); });

    function showModalStep(step) {
        const footerEl = document.querySelector('#addProductModal .dialog-footer');
        ['scan', 'form', 'success'].forEach(function (s) {
            document.getElementById('modalStep' + cap(s)).style.display   = s === step ? 'block' : 'none';
            document.getElementById('modalFooter' + cap(s)).style.display = s === step ? 'flex'  : 'none';
        });
        if (footerEl) footerEl.style.display = step === 'success' ? 'none' : '';
        if (step === 'scan')  subtitle.textContent = proviant.t("js.products.modalSubtitleScan");
        if (step === 'form')  subtitle.textContent = proviant.t("js.products.modalSubtitleConfirm");
    }

    function resetToScan() {
        stopCameraIfRunning();
        setActiveTab('manual');
        if (barcodeInput) {
            barcodeInput.value = '';
            barcodeInput.style.backgroundColor = '';
            barcodeInput.style.color = '';
            barcodeInput.classList.remove('border-success');
        }
        if (productData) productData.classList.add('d-none');
        showModalStep('scan');
        if (barcodeInput) barcodeInput.focus();
    }

    // Focus ring on barcode input wrapper
    if (barcodeInput && inputWrap) {
        barcodeInput.addEventListener('focus', function () { inputWrap.classList.add('focused'); });
        barcodeInput.addEventListener('blur',  function () { inputWrap.classList.remove('focused'); });
    }

    function populateScanResult() {
        const barcode    = barcodeInput ? barcodeInput.value : '';
        const nameEl     = document.getElementById('productInfoName');
        const name       = nameEl ? nameEl.innerText.trim() : '';
        const resultName = document.getElementById('scanResultName');
        const resultCode = document.getElementById('scanResultCode');
        if (resultName) resultName.textContent = name || proviant.t("js.products.unknownProduct");
        if (resultCode) resultCode.textContent = barcode + ' · Open Food Facts';
    }

    // Open from header button
    const openBtn = document.getElementById('btnOpenAddProductModal');
    if (openBtn) {
        openBtn.addEventListener('click', function () { bsModal.show(); });
    }

    // Open from empty-state button (noproducts partial)
    document.addEventListener('click', function (event) {
        if (event.target.closest('#btnOpenAddProductModalEmpty')) {
            event.preventDefault();
            bsModal.show();
        }
    });

    modalEl.addEventListener('shown.bs.dialog', function () {
        modalIsOpen = true;
        resetToScan();
    });

    modalEl.addEventListener('hidden.bs.dialog', function () {
        modalIsOpen = false;
        stopCameraIfRunning();
    });

    // Advance to form step when productsCreate.js reveals #productData
    if (productData) {
        new MutationObserver(function () {
            if (modalIsOpen && !productData.classList.contains('d-none')) {
                populateScanResult();
                showModalStep('form');
            }
        }).observe(productData, { attributes: true, attributeFilter: ['class'] });
    }

    const backBtn = document.getElementById('btnModalBack');
    if (backBtn) backBtn.addEventListener('click', resetToScan);

    const scanAnotherBtn = document.getElementById('btnModalScanAnother');
    if (scanAnotherBtn) scanAnotherBtn.addEventListener('click', resetToScan);

    // Intercept success feedback to show inline success step
    if (typeof proviant !== 'undefined' && typeof proviant.showFeedback === 'function') {
        const _origFeedback = proviant.showFeedback.bind(proviant);
        // Forward onClose: callers (bulkAction's failure paths, import's
        // reload) rely on it running when the toast is dismissed — dropping
        // the 4th arg here would leave a stale selection behind on every
        // failed bulk action.
        proviant.showFeedback = function (type, title, msg, onClose) {
            if (type === 'success' && modalIsOpen) {
                const nameEl = document.getElementById('productInfoName');
                const successNameEl = document.getElementById('modalSuccessName');
                if (successNameEl) {
                    successNameEl.textContent = proviant.t("js.products.nameAdded", nameEl ? nameEl.innerText.trim() : '');
                }
                showModalStep('success');
                return;
            }
            _origFeedback(type, title, msg, onClose);
        };
    }
});

// ── List view: checkbox selection ──────────────────────────────
function updateBulkSelected() {
    const checked = document.querySelectorAll('.row-checkbox:checked');
    const rows = document.querySelectorAll('.list-row');
    const isMobile = window.innerWidth <= 767;

    rows.forEach(function (row) {
        const cb = row.querySelector('.row-checkbox');
        row.classList.toggle('selected', cb && cb.checked);
    });

    if (checked.length > 0) {
        if (pe.bulkActions) pe.bulkActions.style.display = 'flex';
        if (pe.bulkCount) pe.bulkCount.textContent = proviant.t("js.products.selectedCount", checked.length);
        if (isMobile && pe.mobileBulkActions) {
            pe.mobileBulkActions.style.display = 'block';
            if (pe.mobileBulkCount) pe.mobileBulkCount.textContent = proviant.t("js.products.selectedCount", checked.length);
        }
    } else {
        if (pe.bulkActions) pe.bulkActions.style.display = 'none';
        if (pe.mobileBulkActions) pe.mobileBulkActions.style.display = 'none';
    }

    const total = document.querySelectorAll('.row-checkbox').length;
    if (pe.selectedAll) {
        pe.selectedAll.indeterminate = checked.length > 0 && checked.length < total;
        pe.selectedAll.checked = checked.length === total && total > 0;
    }
}

function getSelectedIDs() {
    return [...document.querySelectorAll('.row-checkbox:checked')].map(function (c) { return +c.value; });
}

// ── Cook modal ──────────────────────────────────────────────────
var cookModalInstance = null;

function getCookModal() {
    if (!cookModalInstance) {
        var el = document.getElementById('cookModal');
        if (!el || typeof bootstrap === 'undefined') return null;
        cookModalInstance = new bootstrap.Dialog(el);
        el.addEventListener('hidden.bs.dialog', resetCookModal);
        // Clamp on change/blur (not per keystroke): corrects out-of-range
        // values without fighting the user while typing. Prevents a stray
        // value from falling through to 0, which the backend treats as a
        // full consume; submit-time clamping below is the safety net.
        el.addEventListener('change', function (event) {
            var input = event.target.closest('.cook-item-row input');
            if (!input) return;
            var max = Number.parseInt(input.closest('.cook-item-row').dataset.maxQuantity, 10) || 1;
            var quantity = Number.parseInt(input.value, 10) || 1;
            if (quantity < 1) quantity = 1;
            if (quantity > max) quantity = max;
            input.value = String(quantity);
        });
    }
    return cookModalInstance;
}

function resetCookModal() {
    var container = document.getElementById('cookModalItems');
    if (container) container.innerHTML = '';
    var btn = document.getElementById('btnCookConfirm');
    if (btn) {
        btn.disabled = false;
        btn.classList.remove('loading');
    }
}

// Fetches fresh stock for the selected products before opening the modal.
// Page-rendered data-quantity attributes go stale when another tab, device or
// a background stock change reduces it — without this, the modal shows a
// max above real stock and an overshoot submit silently archives the product.
// Fetches only the selected IDs via the ?ids= filter. Any failure (server
// error or network) blocks cooking: cooking against unknown stock can
// silently archive products, and a failed GET means the cook POST would
// fail anyway, so a stale fallback would only ever mask the failure.
function fetchFreshProducts(ids) {
    return fetch('/api/v1/products?ids=' + encodeURIComponent(ids.join(',')))
        .then(function (res) {
            if (!res.ok) return null;
            return res.json().then(function (products) {
                var map = new Map();
                (Array.isArray(products) ? products : []).forEach(function (p) {
                    if (p && p.id) map.set(+p.id, p);
                });
                return map;
            });
        })
        .catch(function () {
            return null;
        });
}

async function openCookModal(ids, triggerBtn) {
    var container = document.getElementById('cookModalItems');
    if (!container) return;

    // Fetching fresh stock can exceed the 500ms feedback threshold — show
    // the loading state on the triggering button until every early return
    // (skipped items, modal unavailable, or success) is done.
    var setTriggerLoading = function (loading) {
        if (!triggerBtn) return;
        triggerBtn.disabled = loading;
        triggerBtn.classList.toggle('loading', loading);
    };
    setTriggerLoading(true);
    try {
        await openCookModalInner(ids, container);
    } finally {
        setTriggerLoading(false);
    }
}

async function openCookModalInner(ids, container) {
    var freshMap = await fetchFreshProducts(ids);
    if (!freshMap) {
        // Fetch failure must not degrade to stale DOM counts — cooking
        // against unknown stock can silently archive products.
        proviant.showFeedback('error', proviant.t("js.products.cookUnavailable"), proviant.t("js.products.cookStockLoadFailed"));
        return;
    }

    container.innerHTML = '';
    var added = 0;
    var skipped = 0;
    ids.forEach(function (id) {
        var fp = freshMap.get(id);
        if (!fp) {
            // Fresh fetch succeeded but the product is missing from it —
            // it was deleted or archived elsewhere. Never fall back to the
            // stale DOM row, which would offer cooking a gone product.
            skipped++;
            return;
        }
        var quantity = Number.parseInt(fp.quantity, 10) || 0;
        var name = fp.productName || ('#' + id);
        if (quantity < 1) {
            skipped++;
            return;
        }

        var row = document.createElement('div');
        row.className = 'cook-item-row d-flex align-items-center gap-3 py-1';
        row.dataset.productId = id;
        row.dataset.maxQuantity = String(quantity);

        var nameSpan = document.createElement('span');
        nameSpan.className = 'cook-item-name flex-grow-1 text-truncate';
        nameSpan.textContent = name;

        var input = document.createElement('input');
        input.type = 'number';
        input.className = 'form-control form-control-sm cook-item-input';
        input.min = '1';
        input.max = String(quantity);
        input.value = String(quantity);
        input.setAttribute('aria-label', proviant.t("js.products.cookQuantityAria", name));

        row.appendChild(nameSpan);
        row.appendChild(input);
        container.appendChild(row);
        added++;
    });

    if (skipped) {
        var note = document.createElement('div');
        note.className = 'fg-secondary small py-1';
        note.textContent = proviant.tn("js.products.cookSkippedNote", skipped);
        container.appendChild(note);
    }

    if (!added) {
        proviant.showFeedback('warning', proviant.t("js.products.nothingToCook"), proviant.t("js.products.noStockLeft"));
        return;
    }
    var modal = getCookModal();
    if (!modal) {
        proviant.showFeedback('error', proviant.t("js.products.cookUnavailable"), proviant.t("js.products.uiComponentsFailed"));
        return;
    }
    modal.show();
}

async function submitCook() {
    var btn = document.getElementById('btnCookConfirm');
    if (!btn || btn.disabled) return;

    var items = [];
    document.querySelectorAll('#cookModalItems .cook-item-row').forEach(function (row) {
        var input = row.querySelector('input');
        var max = Number.parseInt(row.dataset.maxQuantity, 10) || 1;
        var quantity = input ? Number.parseInt(input.value, 10) : Number.NaN;
        // An invalid or out-of-range value must never fall through to 0 —
        // the backend clamps 0 to a full consume and would archive the product.
        if (Number.isNaN(quantity) || quantity > max) quantity = max;
        if (quantity < 1) quantity = 1;
        items.push({ productId: +row.dataset.productId, quantity: quantity });
    });
    if (!items.length) return;

    btn.disabled = true;
    btn.classList.add('loading');
    try {
        var res = await proviant.cookProducts(items);
        if (res.code !== 200) {
            throw new Error(res.errors && res.errors.length ? res.errors.join('; ') : proviant.t("js.products.requestFailed"));
        }
        var msg = proviant.tn("js.products.cookConsumed", res.consumed);
        if (res.partial) msg += proviant.t("js.products.cookPartialSuffix", res.partial);
        var modal = getCookModal();
        if (modal) modal.hide();
        if (res.errors.length) {
            // Partial failure: the toast stays up for six seconds, then the
            // list reloads. An 800ms auto-reload would hide the details
            // before they can be read.
            msg += proviant.t("js.products.cookErrorsSuffix", res.errors.length, res.errors.join('; '));
            proviant.showFeedback('warning', proviant.t("js.products.cookedWithErrors"), msg, clearBulkSelectionAndReload);
            return;
        }
        proviant.showFeedback('success', proviant.t("js.products.cooked"), msg);
        setTimeout(clearBulkSelectionAndReload, 800);
    } catch (err) {
        btn.disabled = false;
        btn.classList.remove('loading');
        proviant.showFeedback('error', proviant.t("js.products.cookFailed"), err.message);
    }
}

// ── Partial consume / waste ─────────────────────────────────────
var partialActionModalInstance = null;
var partialActionState = { productID: 0, maxQuantity: 0, busy: false, openToken: 0 };

function getPartialActionModal() {
    if (!partialActionModalInstance) {
        var element = document.getElementById('partialActionModal');
        if (!element || typeof bootstrap === 'undefined') return null;
        partialActionModalInstance = new bootstrap.Dialog(element);
    }
    return partialActionModalInstance;
}

function validatePartialActionQuantity() {
    var input = document.getElementById('partialActionQuantity');
    var error = document.getElementById('partialActionQuantityError');
    if (!input) return null;
    var quantity = Number(input.value);
    var valid = Number.isInteger(quantity) && quantity >= 1 && quantity <= partialActionState.maxQuantity;
    input.classList.toggle('is-invalid', !valid);
    input.setAttribute('aria-invalid', valid ? 'false' : 'true');
    if (error) {
        error.textContent = valid ? '' : proviant.t('js.products.partialActionQuantityInvalid', partialActionState.maxQuantity);
    }
    return valid ? quantity : null;
}

function setPartialActionBusy(busy, action) {
    partialActionState.busy = busy;
    [
        { id: 'btnPartialConsumeConfirm', action: 'consume' },
        { id: 'btnPartialWasteConfirm', action: 'waste' },
    ].forEach(function (entry) {
        var button = document.getElementById(entry.id);
        if (!button) return;
        button.disabled = busy;
        button.classList.toggle('loading', busy && entry.action === action);
        if (busy) button.setAttribute('aria-busy', 'true');
        else button.removeAttribute('aria-busy');
    });
    var input = document.getElementById('partialActionQuantity');
    if (input) input.disabled = busy;
    document.querySelectorAll('#partialActionModal [data-bs-dismiss="dialog"]').forEach(function (button) {
        button.disabled = busy;
    });
}

async function openPartialActionModal(productID, triggerButton) {
    var id = Number.parseInt(productID, 10);
    if (!Number.isInteger(id) || id < 1 || partialActionState.busy) return;
    // Two products clicked in quick succession would race: the slower fetch
    // would land last and put its product into the dialog the faster one
    // opened. The token makes the newer request win and the older one
    // harmless.
    var token = ++partialActionState.openToken;
    var triggerHTML = triggerButton ? triggerButton.innerHTML : '';
    if (triggerButton) {
        triggerButton.disabled = true;
        triggerButton.classList.add('loading');
        triggerButton.setAttribute('aria-busy', 'true');
        triggerButton.innerHTML = '<i class="bi bi-hourglass-split" aria-hidden="true"></i>';
    }
    try {
        var freshMap = await fetchFreshProducts([id]);
        if (token !== partialActionState.openToken) return;
        if (!freshMap) {
            proviant.showFeedback('error', proviant.t('js.products.partialActionUnavailable'), proviant.t('js.products.partialActionStockLoadFailed'));
            return;
        }
        var product = freshMap.get(id);
        var quantity = product ? Number.parseInt(product.quantity, 10) : 0;
        if (!product || !Number.isInteger(quantity) || quantity < 1) {
            proviant.showFeedback('warning', proviant.t('js.products.partialActionUnavailable'), proviant.t('js.products.partialActionNoStock'));
            return;
        }

        partialActionState.productID = id;
        partialActionState.maxQuantity = quantity;
        var name = document.getElementById('partialActionProductName');
        if (name) name.textContent = product.productName || ('#' + id);
        var stock = document.getElementById('partialActionStock');
        if (stock) stock.textContent = proviant.tn('js.products.partialActionStock', quantity);
        var input = document.getElementById('partialActionQuantity');
        if (input) {
            input.max = String(quantity);
            input.value = '1';
            input.classList.remove('is-invalid');
            input.setAttribute('aria-invalid', 'false');
        }
        var error = document.getElementById('partialActionQuantityError');
        if (error) error.textContent = '';

        var modal = getPartialActionModal();
        if (!modal) {
            proviant.showFeedback('error', proviant.t('js.products.partialActionUnavailable'), proviant.t('js.products.uiComponentsFailed'));
            return;
        }
        modal.show();
    } finally {
        if (triggerButton) {
            triggerButton.disabled = false;
            triggerButton.classList.remove('loading');
            triggerButton.removeAttribute('aria-busy');
            triggerButton.innerHTML = triggerHTML;
        }
    }
}

async function submitPartialAction(action) {
    if (partialActionState.busy) return;
    var quantity = validatePartialActionQuantity();
    if (quantity === null) return;
    var productID = partialActionState.productID;
    setPartialActionBusy(true, action);
    try {
        var send = action === 'consume' ? proviant.consumeProduct : proviant.wasteProduct;
        var response = await send(productID, quantity);
        var modal = getPartialActionModal();
        if (response.queued && response.code === 202) {
            if (modal) modal.hide();
            proviant.showFeedback('info', proviant.t('js.products.productQueuedOffline'), proviant.t('js.products.partialActionQueued'));
            return;
        }
        if (response.code !== 200) {
            throw new Error(response.message || proviant.t('js.products.requestFailed'));
        }
        if (modal) modal.hide();
        // The server clamps the request to the stock that is actually left, so
        // the reported count is the one that left the pantry.
        var removed = typeof response.removed === 'number' ? response.removed : quantity;
        var successTitle = action === 'consume' ? proviant.t('js.products.consumedButton') : proviant.t('js.products.wastedButton');
        var messageKey = action === 'consume' ? 'js.products.partialConsumeSuccess' : 'js.products.partialWasteSuccess';
        proviant.showFeedback('success', successTitle, proviant.tn(messageKey, removed));
        setTimeout(clearBulkSelectionAndReload, 800);
    } catch (error) {
        var errorTitle = action === 'consume' ? proviant.t('js.products.consumeFailed') : proviant.t('js.products.wastedFailed');
        proviant.showFeedback('error', errorTitle, error.message);
    } finally {
        setPartialActionBusy(false, null);
    }
}

// ── List view: qty stepper ──────────────────────────────────────
function changeQty(id, delta) {
    const span = document.getElementById('qty-' + id);
    proviant.updateProductQuantity(id, delta).then((response) => {
        if (response.code === 200) {
            if (response.deleted) {
                const row = document.querySelector(`.list-row[data-id="${id}"]`);
                if (row) row.remove();
            } else {
                if (span) {
                    span.dataset.quantity = String(Math.max(0, readQuantity(span) + delta));
                    proviant.renderQuantityLabel(span);
                    span.classList.add('qty-updated');
                    setTimeout(() => span.classList.remove('qty-updated'), 800);
                }
            }
        } else {
            reportQuantityChangeFailed(response);
        }
    }).catch((err) => proviant.showFeedback('error', proviant.t("common.error"), err.message));
}

// Skeleton loading functions for filter/search operations
function showSkeleton() {
  const productRows = document.getElementById('productRows');
  const skeletonRows = document.getElementById('skeletonRows');
  if (productRows) productRows.classList.add('d-none');
  if (skeletonRows) skeletonRows.classList.remove('d-none');
}

// Show skeleton when navigating via filter, view toggle, or clear-all
document.addEventListener('click', function(event) {
  if (event.target.closest('.filter-pill') || event.target.closest('.btn-view') || event.target.closest('#show-all-btn')) {
    showSkeleton();
  }
});

// ── Image fallback handling ─────────────────────────────────────
document.addEventListener('DOMContentLoaded', function () {
    function findFallback(img) {
        var listThumb = img.closest('.list-thumb');
        if (listThumb) {
            return listThumb.querySelector('.list-thumb-fallback');
        }
        var productImg = img.closest('.product-img');
        if (productImg) {
            return productImg.querySelector('.product-img-fallback');
        }
        return null;
    }

    document.querySelectorAll('.list-thumb-img, .product-card .product-img img').forEach(function (img) {
        img.addEventListener('load', function () {
            img.style.opacity = '1';
            var fb = findFallback(img);
            if (fb) fb.style.display = 'none';
        });
        img.addEventListener('error', function () {
            var fb = findFallback(img);
            if (fb) fb.style.display = 'flex';
        });
    });
});
