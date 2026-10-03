/* exported changeQty */

const pe = {
    editProduct: null,
    searchParam: null,
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
    // Amount increment button
    if (target.closest(".btn-amount-inc")) {
        event.preventDefault();
        event.stopPropagation();
        const btn = target.closest(".btn-amount-inc");
        const productID = btn.dataset.productId;
        proviant.updateProductAmount(productID, 1).then((response) => {
            if (response.code === 200) {
                const amountEl = document.getElementById(`amount-${productID}`);
                if (amountEl) {
                    amountEl.textContent = parseInt(amountEl.textContent, 10) + 1;
                    amountEl.classList.add('amount-updated');
                    setTimeout(() => amountEl.classList.remove('amount-updated'), 800);
                }
            } else {
                console.error(response.message);
            }
        });
        return;
    }

    // Amount decrement button
    if (target.closest(".btn-amount-dec")) {
        event.preventDefault();
        event.stopPropagation();
        const btn = target.closest(".btn-amount-dec");
        const productID = btn.dataset.productId;
        proviant.updateProductAmount(productID, -1).then((response) => {
            if (response.code === 200) {
                if (response.deleted) {
                    const card = document.getElementById(`card-${productID}`);
                    if (card) card.closest(".col").remove();
                } else {
                    const amountEl = document.getElementById(`amount-${productID}`);
                    if (amountEl) {
                        const newVal = Math.max(0, parseInt(amountEl.textContent, 10) - 1);
                        amountEl.textContent = newVal;
                        amountEl.classList.add('amount-updated');
                        setTimeout(() => amountEl.classList.remove('amount-updated'), 800);
                    }
                }
            } else {
                console.error(response.message);
            }
        });
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
                const formatted = isNaN(previous.getTime()) ? response.openedAt : previous.toLocaleString();
                proviant.showConfirm(
                    'Already opened',
                    `This product was already marked as opened on ${formatted}. Reset the opened date to now?`,
                    () => sendOpen(true),
                    'Reset',
                    'warning'
                );
            } else {
                proviant.showFeedback('error', 'Could not mark as opened', 'Please try again.');
            }
        });
        sendOpen(false);
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
            handleSelected();
            handleCardClickEffect(cardId);
        }
        return;
    }

    // Search button
    if (target.closest("#search-btn")) {
        event.preventDefault();
        performSearch();
        return;
    }

    // Add to shopping list button
    var addToListBtn = target.closest('.btn-add-to-shopping-list');
    if (addToListBtn) {
        event.preventDefault();
        var productId = addToListBtn.dataset.productId;
        proviant.runWithButtonBusyState(
            addToListBtn,
            async function () {
                const res = await fetch('/api/v1/shopping-list', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ productId: parseInt(productId, 10) })
                });
                if (!res.ok) {
                    const body = await res.json().catch(function () { return {}; });
                    throw new Error(body.message || 'Could not add to shopping list');
                }
            },
            'Added to shopping list',
            function (err) {
                proviant.showFeedback('error', 'Failed to add to list', err.message);
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
            openCookModal(ids, bulkBtn);
            return;
        }
        var opts = action === 'delete' ? { confirm: true, confirmTitle: 'Mark as wasted', confirmMsg: 'Mark ' + ids.length + ' product(s) as wasted? This cannot be undone.' } : {};
        proviant.bulkAction(action, ids, opts);
        return;
    }

    // Cook modal confirm button
    if (target.closest('#btnCookConfirm')) {
        event.preventDefault();
        submitCook();
        return;
    }

    // List-view qty stepper buttons
    var qtyBtn = target.closest('[data-qty-action]');
    if (qtyBtn) {
        event.preventDefault();
        var id = parseInt(qtyBtn.dataset.productId, 10);
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

// Function to perform search
function performSearch() {
    const params = {
        queryParam: pe.searchParam ? pe.searchParam.value || "product_name" : "product_name",
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
   DOMContentLoaded before calling bootstrap.Modal.
─────────────────────────────────────────────────────────────────────────── */
document.addEventListener('DOMContentLoaded', function () {
    const modalEl = document.getElementById('addProductModal');
    if (!modalEl) return;

    const bsModal         = new bootstrap.Modal(modalEl);
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
    const searchParam     = document.getElementById('search-param');
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
    pe.searchParam = searchParam;
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
            tabManualInput.style.boxShadow   = isCamera ? 'none' : '0 1px 3px oklch(0 0 0 / 0.10)';
            tabManualInput.style.color       = isCamera ? 'var(--fg-3)' : 'var(--fg)';
        }
        if (tabCamera) {
            tabCamera.style.background  = isCamera ? 'white' : 'transparent';
            tabCamera.style.boxShadow   = isCamera ? '0 1px 3px oklch(0 0 0 / 0.10)' : 'none';
            tabCamera.style.color       = isCamera ? 'var(--fg)' : 'var(--fg-3)';
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
        const footerEl = document.querySelector('#addProductModal .modal-footer');
        ['scan', 'form', 'success'].forEach(function (s) {
            document.getElementById('modalStep' + cap(s)).style.display   = s === step ? 'block' : 'none';
            document.getElementById('modalFooter' + cap(s)).style.display = s === step ? 'flex'  : 'none';
        });
        if (footerEl) footerEl.style.display = step === 'success' ? 'none' : '';
        if (step === 'scan')  subtitle.textContent = 'Scan barcode or enter manually';
        if (step === 'form')  subtitle.textContent = 'Confirm details';
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
        if (resultName) resultName.textContent = name || 'Unknown product';
        if (resultCode) resultCode.textContent = barcode + ' · Open Food Facts';
    }

    // Open from header button
    const openBtn = document.getElementById('btnOpenAddProductModal');
    if (openBtn) {
        openBtn.addEventListener('click', function () { bsModal.show(); });
    }

    modalEl.addEventListener('shown.bs.modal', function () {
        modalIsOpen = true;
        resetToScan();
    });

    modalEl.addEventListener('hidden.bs.modal', function () {
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
        proviant.showFeedback = function (type, title, msg) {
            if (type === 'success' && modalIsOpen) {
                const nameEl = document.getElementById('productInfoName');
                const successNameEl = document.getElementById('modalSuccessName');
                if (successNameEl) {
                    successNameEl.textContent = (nameEl ? nameEl.innerText.trim() : '') + ' added';
                }
                showModalStep('success');
                return;
            }
            _origFeedback(type, title, msg);
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
        if (pe.bulkCount) pe.bulkCount.textContent = checked.length + ' selected';
        if (isMobile && pe.mobileBulkActions) {
            pe.mobileBulkActions.style.display = 'block';
            if (pe.mobileBulkCount) pe.mobileBulkCount.textContent = checked.length + ' selected';
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
        cookModalInstance = new bootstrap.Modal(el);
        el.addEventListener('hidden.bs.modal', resetCookModal);
        // Clamp on change/blur (not per keystroke): corrects out-of-range
        // values without fighting the user while typing. Prevents a stray
        // value from falling through to 0, which the backend treats as a
        // full consume; submit-time clamping below is the safety net.
        el.addEventListener('change', function (event) {
            var input = event.target.closest('.cook-item-row input');
            if (!input) return;
            var max = parseInt(input.closest('.cook-item-row').dataset.maxAmount, 10) || 1;
            var amount = parseInt(input.value, 10) || 1;
            if (amount < 1) amount = 1;
            if (amount > max) amount = max;
            input.value = String(amount);
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
// Page-rendered data-amount attributes go stale when another tab, device or
// a background amount change reduces stock — without this, the modal shows a
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
                    if (p && p.ID) map.set(+p.ID, p);
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
        // Fetch failure must not degrade to stale DOM amounts — cooking
        // against unknown stock can silently archive products.
        proviant.showFeedback('error', 'Cook unavailable', 'Could not load current stock, please try again');
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
        var amount = parseInt(fp.amount, 10) || 0;
        var unit = fp.unit || '';
        var name = fp.productName || ('#' + id);
        if (amount < 1) {
            skipped++;
            return;
        }

        var row = document.createElement('div');
        row.className = 'cook-item-row d-flex align-items-center gap-2 py-1';
        row.dataset.productId = id;
        row.dataset.maxAmount = String(amount);

        var nameSpan = document.createElement('span');
        nameSpan.className = 'cook-item-name flex-grow-1 text-truncate';
        nameSpan.textContent = name;

        var unitSpan = document.createElement('span');
        unitSpan.className = 'text-secondary-custom small flex-shrink-0';
        unitSpan.textContent = unit;

        var input = document.createElement('input');
        input.type = 'number';
        input.className = 'form-control form-control-sm cook-item-input';
        input.min = '1';
        input.max = String(amount);
        input.value = String(amount);
        input.setAttribute('aria-label', 'Amount to cook for ' + name);

        row.appendChild(nameSpan);
        if (unit) row.appendChild(unitSpan);
        row.appendChild(input);
        container.appendChild(row);
        added++;
    });

    if (skipped) {
        var note = document.createElement('div');
        note.className = 'text-secondary-custom small py-1';
        note.textContent = skipped + ' selected product(s) skipped (no stock or not found)';
        container.appendChild(note);
    }

    if (!added) {
        proviant.showFeedback('warning', 'Nothing to cook', 'None of the selected products have stock left');
        return;
    }
    var modal = getCookModal();
    if (!modal) {
        proviant.showFeedback('error', 'Cook unavailable', 'UI components failed to load, please reload the page');
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
        var max = parseInt(row.dataset.maxAmount, 10) || 1;
        var amount = input ? parseInt(input.value, 10) : NaN;
        // An invalid or out-of-range value must never fall through to 0 —
        // the backend clamps 0 to a full consume and would archive the product.
        if (isNaN(amount) || amount > max) amount = max;
        if (amount < 1) amount = 1;
        items.push({ productId: +row.dataset.productId, amount: amount });
    });
    if (!items.length) return;

    btn.disabled = true;
    btn.classList.add('loading');
    try {
        var res = await proviant.cookProducts(items);
        if (res.code !== 200) {
            throw new Error(res.errors && res.errors.length ? res.errors.join('; ') : 'Request failed');
        }
        var msg = res.consumed + ' product' + (res.consumed !== 1 ? 's' : '') + ' fully consumed';
        if (res.partial) msg += ', ' + res.partial + ' partially';
        var modal = getCookModal();
        if (modal) modal.hide();
        if (res.errors.length) {
            // Partial failure: keep the failure details on screen until the
            // user dismisses, then reload — an 800ms auto-reload would hide
            // them before they can be read.
            // Single-line details: #proviantFeedbackMessage is a plain <p>,
            // so \n would collapse into one unreadable run-on line anyway.
            msg += ' (' + res.errors.length + ' failed: ' + res.errors.join('; ') + ')';
            proviant.showFeedback('warning', 'Cooked with errors', msg, clearBulkSelectionAndReload);
            return;
        }
        proviant.showFeedback('success', 'Cooked', msg);
        setTimeout(clearBulkSelectionAndReload, 800);
    } catch (err) {
        btn.disabled = false;
        btn.classList.remove('loading');
        proviant.showFeedback('error', 'Cook failed', err.message);
    }
}

// ── List view: qty stepper ──────────────────────────────────────
function changeQty(id, delta) {
    proviant.updateProductAmount(id, delta).then((response) => {
        if (response.code === 200) {
            if (response.deleted) {
                const row = document.querySelector(`.list-row[data-id="${id}"]`);
                if (row) row.remove();
            } else {
                const span = document.getElementById('qty-' + id);
                if (span) {
                    span.textContent = Math.max(0, parseInt(span.textContent, 10) + delta);
                    span.classList.add('amount-updated');
                    setTimeout(() => span.classList.remove('amount-updated'), 800);
                }
            }
        } else {
            console.error(response.message);
        }
    });
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
