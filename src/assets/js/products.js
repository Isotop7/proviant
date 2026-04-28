/* exported changeQty */

async function bulkAction(action, productIDs) {
    const fn = { delete: proviant.bulkDeleteProducts, restore: proviant.bulkRestoreProducts, archive: proviant.bulkArchiveProducts }[action];
    const response = await fn(productIDs);
    if (response.code !== 200) console.error(response.message);
}

function handleCardClickEffect(cardId) {
    const card = document.getElementById(cardId);
    if (card) {
        card.classList.add('card-clicked');
        setTimeout(() => card.classList.remove('card-clicked'), 100);
    }
}

async function handleSelect() {
    const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
    const editBtn = document.getElementById('edit-product');
    if (editBtn) {
        editBtn.disabled = selectedProducts.length !== 1;
    }
    // Also update bulk selection state (for mobile bulk bar, etc.)
    updateBulkSelection();
}

/* Event delegation for clicks */
document.addEventListener("click", function (event) {
    const target = event.target;

    // Edit product button
    if (target.closest("#edit-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        if (selectedProducts.length === 1) {
            window.location.href = `/web/products/${selectedProducts[0]}/edit`;
        }
        return;
    }

    // Delete product button
    if (target.closest("#delete-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        const count = selectedProducts.length;
        proviant.showConfirm(
            'Delete Products',
            `Delete ${count} selected product${count !== 1 ? 's' : ''}? This cannot be undone.`,
            function () {
                bulkAction('delete', selectedProducts).then(() => location.reload());
            },
            'Delete',
            'danger'
        );
        return;
    }

    // Restore product button
    if (target.closest("#restore-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        bulkAction('restore', selectedProducts).then(() => location.reload());
        return;
    }

    // Archive product button
    if (target.closest("#archive-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        const count = selectedProducts.length;
        proviant.showConfirm(
            'Archive Products',
            `Archive ${count} selected product${count !== 1 ? 's' : ''}?`,
            function () {
                bulkAction('archive', selectedProducts).then(() => location.reload());
            },
            'Archive',
            'warning'
        );
        return;
    }

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

    // Card click
    const card = target.closest('.card');
    if (card) {
        const cardId = card.id;
        const productId = cardId.split('-')[1];
        const checkbox = document.getElementById(`checkbox-${productId}`);
        if (checkbox) {
            checkbox.checked = !checkbox.checked;
            card.classList.toggle('border-info');
            handleSelect();
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
});

/* Event delegation for checkbox changes */
document.addEventListener("change", function (event) {
    const target = event.target;
    if (target.matches('input[type="checkbox"]')) {
        const cardId = `card-${target.id.split('-')[1]}`;
        const card = document.getElementById(cardId);
        const checkbox = document.getElementById(`checkbox-${target.id.split('-')[1]}`);
        if (card) {
            card.classList.toggle('border-info');
            if (checkbox) {
                checkbox.checked = !checkbox.checked;
            }
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
    const queryParam = document.getElementById("search-param");
    const queryValue = document.getElementById("search-query");
    const sortParam = document.getElementById("sort-param");
    const sortOrder = document.getElementById("sort-order");
    // Build query string and redirect to products page
    const locationFilter = document.getElementById("location-filter");
    const params = {
        queryParam: queryParam ? queryParam.value || "product_name" : "product_name",
        queryValue: queryValue ? queryValue.value || "" : "",
        sort: sortParam ? sortParam.value || "created_at" : "created_at",
        order: sortOrder ? sortOrder.value || "asc" : "asc",
    };
    if (locationFilter && locationFilter.value) {
        params.locationId = locationFilter.value;
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
function updateBulkSelection() {
    const checked = document.querySelectorAll('.row-checkbox:checked');
    const bulk = document.getElementById('bulkActions');
    const count = document.getElementById('bulkCount');
    const rows = document.querySelectorAll('.list-row');
    const mobileBulk = document.getElementById('mobileBulkActions');
    const mobileCount = document.getElementById('mobileBulkCount');
    const isMobile = window.innerWidth <= 767;

    rows.forEach(function (row) {
        const cb = row.querySelector('.row-checkbox');
        row.classList.toggle('selected', cb && cb.checked);
    });

    if (checked.length > 0) {
        if (bulk) bulk.style.display = 'flex';
        if (count) count.textContent = checked.length + ' selected';
        if (isMobile && mobileBulk) {
            mobileBulk.style.display = 'block';
            if (mobileCount) mobileCount.textContent = checked.length + ' selected';
        }
    } else {
        if (bulk) bulk.style.display = 'none';
        if (mobileBulk) mobileBulk.style.display = 'none';
    }

    const all = document.getElementById('selectAll');
    const total = document.querySelectorAll('.row-checkbox').length;
    if (all) {
        all.indeterminate = checked.length > 0 && checked.length < total;
        all.checked = checked.length === total && total > 0;
    }
}

function bulkAction(action) {
    const ids = [...document.querySelectorAll('.row-checkbox:checked')].map(function (c) {
        return +c.value;
    });
    if (!ids.length) return;
    if (action === 'delete') {
        if (!confirm('Delete ' + ids.length + ' product(s)? This cannot be undone.')) return;
        proviant.bulkDeleteProducts(ids).then(function () {
            window.location.reload();
        });
    } else if (action === 'archive') {
        proviant.bulkArchiveProducts(ids).then(function () {
            window.location.reload();
        });
    } else if (action === 'restore') {
        proviant.bulkRestoreProducts(ids).then(function () {
            window.location.reload();
        });
    }
}

// ── List view: qty stepper ──────────────────────────────────────
// Inline onclick handlers in list view call this function
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
function hideSkeleton() {
  const productRows = document.getElementById('productRows');
  const skeletonRows = document.getElementById('skeletonRows');
  if (productRows) productRows.classList.remove('d-none');
  if (skeletonRows) skeletonRows.classList.add('d-none');
}

// Show skeleton when navigating via filter, view toggle, or clear-all
document.addEventListener('click', function(event) {
  if (event.target.closest('.filter-pill') || event.target.closest('.btn-view') || event.target.closest('#show-all-btn')) {
    showSkeleton();
  }
});
