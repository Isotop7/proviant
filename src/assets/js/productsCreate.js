const el = Object.freeze({
    foundBarcodeWrapper: document.getElementById('foundBarcode'),
    productDataWrapper: document.getElementById('productData'),
    inputBarcode: document.getElementById('barcode'),
    inputExpireAt: document.getElementById('expireAt'),
    productInfoImage: document.getElementById('productInfoImage'),
    productInfoName: document.getElementById('productInfoName'),
    productInfoGenericName: document.getElementById('productInfoGenericName'),
    storageHint: document.getElementById('storageHint'),
    btnScan: document.getElementById('btnScan'),
    btnAddProduct: document.getElementById('btnAddProduct'),
    btnShowProduct: document.getElementById('btnShowProduct'),
    btnDeleteProductModal: document.getElementById('btnDeleteProductModal'),
    btnArchiveProductModal: document.getElementById('btnArchiveProductModal'),
    btnShowProducts: document.getElementById('btnShowProducts'),
    instanceDropdown: document.getElementById('instanceDropdown'),
    barcodeLookupLoading: document.getElementById('barcodeLookupLoading'),
    barcodeReaderWrapper: document.getElementById('barcode-reader-wrapper'),
    quantity: document.getElementById('quantity'),
    quantityDisplay: document.getElementById('quantityDisplay'),
    modalStorageLocation: document.getElementById('modalStorageLocation'),
    btnQuantityDec: document.getElementById('btnQuantityDec'),
    btnQuantityInc: document.getElementById('btnQuantityInc'),
    btnExpireAdd3: document.getElementById('btnExpireAdd3'),
    btnExpireAdd7: document.getElementById('btnExpireAdd7'),
    btnExpireAdd1m: document.getElementById('btnExpireAdd1m'),
    // The household's category list and the hint that names the shelf life of
    // the category — both only present on the pages that render them.
    productCategory: document.getElementById('productCategory'),
    expirySuggestion: document.getElementById('expirySuggestion'),
    // The staples the household buys repeatedly — only present on the pages
    // that render the add-product dialog.
    frequentProducts: document.getElementById('frequentProducts'),
    frequentProductsList: document.getElementById('frequentProductsList'),
    // The name the household calls the product and the size of one pack.
    productName: document.getElementById('productName'),
    packSize: document.getElementById('packSize'),
    // Batch scan mode — only present on the scan page, every use is guarded
    batchStartPanel: document.getElementById('batchStartPanel'),
    batchConfirmPanel: document.getElementById('batchConfirmPanel'),
    batchTally: document.getElementById('batchTally'),
    batchStorageLocation: document.getElementById('batchStorageLocation'),
    batchProductName: document.getElementById('batchProductName'),
    batchProductBarcode: document.getElementById('batchProductBarcode'),
    batchQuantity: document.getElementById('batchQuantity'),
    batchQuantityDisplay: document.getElementById('batchQuantityDisplay'),
    batchExpireAt: document.getElementById('batchExpireAt'),
});
const html5QrCode = new Html5Qrcode('barcode-reader',
    { formatsToSupport: [Html5QrcodeSupportedFormats.EAN_13] }
);

const ProductState = Object.freeze({
    INVALID: 'invalid',
    NEW: 'new',
    PRESENT: 'present'
});

// UI functions
function showError(error) {
    if (el.inputBarcode) {
        el.inputBarcode.value = '';
        el.inputBarcode.style.backgroundColor = 'var(--bs-warning)';
        el.inputBarcode.style.color = 'var(--bs-warning-text)';
    }
    console.error(error);
}
function showBarcode(barcode) {
    if (el.inputBarcode) {
        el.inputBarcode.value = barcode;
        el.inputBarcode.classList.add('border-success');
    }
}
function showAlert(isSuccess, message) {
    if (isSuccess) {
        proviant.showFeedback('success', proviant.t("js.products.productCreatedTitle"), message || proviant.t("js.products.productCreatedMessage"));
    } else {
        proviant.showFeedback('error', proviant.t("common.error"), message || proviant.t("js.products.createFailedMessage"));
    }
}
// The product's picture. An empty src queues an error event, and that event
// outlives the image that follows it: the next lookup's picture would be
// hidden by the previous one's failure. Removing the attribute asks for
// nothing and queues nothing.
function setProductImage(url, alt) {
    const img = el.productInfoImage;
    if (!img) return;
    img.alt = alt || '';
    if (!url) {
        img.removeAttribute('src');
        img.classList.add('d-none');
        return;
    }
    img.src = url;
    img.classList.remove('d-none');
}

function clearProductInfo() {
    setProductImage('', '');
    if (el.productInfoName) el.productInfoName.textContent = '';
    if (el.productInfoGenericName) el.productInfoGenericName.textContent = '';
    if (el.storageHint) {
        el.storageHint.textContent = '';
        el.storageHint.classList.add('d-none');
    }
}

// Loading state helper for barcode lookup
function setBarcodeLoading(loading) {
  if (el.barcodeLookupLoading) {
    if (loading) {
      el.barcodeLookupLoading.classList.remove('d-none');
    } else {
      el.barcodeLookupLoading.classList.add('d-none');
    }
  }
}
function showProductData(product) {
    setProductImage(product.imageUrl, product.productName || proviant.t("js.products.productImageAlt"));
    if (el.productInfoName) el.productInfoName.textContent = product.productName;
    // The name and the pack size are editable fields, so the lookup fills
    // them: the user can correct what the barcode answered.
    if (el.productName) el.productName.value = product.productName || '';
    if (el.packSize && product.packSize) el.packSize.value = product.packSize;
    if (product.categories != 'undefined' && product.categories != null) {
        if (el.productInfoGenericName) el.productInfoGenericName.textContent = product.categories;
    }
    if (el.storageHint) {
        if (product.storageHint) {
            el.storageHint.textContent = product.storageHint;
            el.storageHint.classList.remove('d-none');
        } else {
            el.storageHint.classList.add('d-none');
        }
    }
    selectResolvedCategory(product.categories);
    if (el.productDataWrapper) el.productDataWrapper.classList.remove('d-none');
}

// The household's categories, read from the rendered list. The list carries
// the shelf life of every category, so the suggestion works without a
// request — and therefore offline.
function householdCategories() {
    const select = el.productCategory;
    if (!select) return [];
    return Array.from(select.options)
        .filter((option) => option.value !== '')
        .map((option) => ({
            categoryKey: option.value,
            displayName: (option.textContent || '').trim(),
            defaultShelfLifeDays: option.dataset.shelfLifeDays
                ? Number.parseInt(option.dataset.shelfLifeDays, 10)
                : null,
        }));
}

// Picks the category the looked-up product belongs to and fills the expiry
// date the household gave it.
function selectResolvedCategory(categoriesText) {
    const select = el.productCategory;
    if (!select) return;
    const categories = householdCategories();
    const categoryKey = proviant.resolveCategoryKey(categoriesText, categories);
    select.value = categoryKey && categories.some((category) => category.categoryKey === categoryKey)
        ? categoryKey
        : '';
    applyCategorySuggestion();
}

// The shelf life the household gave the selected category, if it has one.
function expiryShelfLife() {
    const select = el.productCategory;
    if (!select) return null;
    const categories = householdCategories();
    const categoryKey = select.value;
    const days = proviant.categoryShelfLifeDays(categories, categoryKey);
    if (days === null) return null;
    const category = proviant.categoryByKey(categories, categoryKey);
    return {
        categories: categories,
        categoryKey: categoryKey,
        days: days,
        displayName: category ? category.displayName : categoryKey,
    };
}

// Names the shelf life the household gave the category. The date is left to
// the user: how long this pack lasts is not something the household's history
// can answer, so the tap must not answer it either.
function showExpiryShelfLife() {
    const hint = el.expirySuggestion;
    if (!hint) return;
    const shelfLife = expiryShelfLife();
    // A date the user set is theirs: the shelf life is not needed to explain
    // it, and the hint would only tell them to set what they already set.
    if (!shelfLife || (el.inputExpireAt && el.inputExpireAt.dataset.userEdited === '1')) {
        hint.classList.add('d-none');
        return;
    }
    hint.textContent = proviant.t("js.products.expiryShelfLife", shelfLife.displayName, shelfLife.days);
    hint.classList.remove('d-none');
}

// Fills the expiry date with the shelf life of the selected category. A date
// the user set is theirs and is never overwritten.
function applyCategorySuggestion() {
    const hint = el.expirySuggestion;
    if (!el.productCategory) return;
    const shelfLife = expiryShelfLife();
    if (!shelfLife) {
        if (hint) hint.classList.add('d-none');
        return;
    }
    // The hint names the source of the date, so it may only show when the
    // suggestion really wrote the date: a date the user set is theirs.
    const applied = proviant.applyExpirySuggestion(el.inputExpireAt, shelfLife.categories, shelfLife.categoryKey);
    if (!applied) {
        if (hint) hint.classList.add('d-none');
        return;
    }
    if (hint) {
        hint.textContent = proviant.t("js.products.expirySuggested", shelfLife.displayName, shelfLife.days);
        hint.classList.remove('d-none');
    }
}

// ── Frequently bought ─────────────────────────────────────────────────────────

// The staples the household buys repeatedly. The list is a server response, so
// it is empty when the server is not there — the form it fills keeps working
// either way.
async function loadFrequentProducts() {
    if (!el.frequentProducts || !el.frequentProductsList) return;
    let products = [];
    try {
        const response = await proviant.getFrequentlyBoughtProducts();
        if (response && response.code === 200 && Array.isArray(response.message)) {
            products = response.message;
        }
    } catch (_) {
        products = [];
    }
    renderFrequentProducts(products);
}

// The barcode field decides which barcodes the form accepts, so the strip
// offers exactly those. A chip the form would reject fills a field that
// blocks the submit, and the household is left with a button that does
// nothing.
//
// The browser applies the rule: the pattern attribute is HTML's "v" dialect,
// which a plain RegExp does not speak.
function barcodeProbe() {
    const input = el.inputBarcode;
    if (!input) return null;
    const pattern = input.getAttribute('pattern');
    if (!pattern) return null;
    const probe = document.createElement('input');
    probe.setAttribute('pattern', pattern);
    return probe;
}

function acceptsBarcode(probe, barcode) {
    if (!probe) return true;
    probe.value = barcode;
    return probe.checkValidity();
}

function renderFrequentProducts(products) {
    if (!el.frequentProducts || !el.frequentProductsList) return;
    el.frequentProductsList.innerHTML = '';
    if (!products || products.length === 0) {
        el.frequentProducts.classList.add('d-none');
        return;
    }
    const probe = barcodeProbe();
    products.forEach((product) => {
        const barcode = product.barcode || '';
        if (!barcode || !acceptsBarcode(probe, barcode)) return;
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'frequent-product';
        button.dataset.barcode = barcode;
        button.dataset.productName = product.productName || '';
        button.dataset.packSize = product.packSize || '';
        button.dataset.categoryKey = product.categoryKey || '';
        button.dataset.storageLocationId = product.storageLocationId || '';
        button.dataset.imageUrl = product.imageUrl || '';
        button.textContent = product.productName || barcode;
        el.frequentProductsList.appendChild(button);
    });
    if (el.frequentProductsList.childElementCount === 0) {
        el.frequentProducts.classList.add('d-none');
        return;
    }
    el.frequentProducts.classList.remove('d-none');
}

// Picks the category the household used for this product. A category the
// household has since removed is not in the list, so the form is left without
// one rather than with the previous product's.
function selectStoredCategory(categoryKey) {
    const select = el.productCategory;
    if (!select) return;
    const known = categoryKey && Array.from(select.options).some((option) => option.value === categoryKey);
    select.value = known ? categoryKey : '';
}

// Picks the storage location the household used for this product. A location
// the household has since removed is not in the list, so the form is left
// without one rather than with the previous product's.
function selectStoredLocation(locationID) {
    const select = el.modalStorageLocation;
    if (!select) return;
    const known = locationID && Array.from(select.options).some((option) => option.value === String(locationID));
    select.value = known ? String(locationID) : '';
}

// Fills the form with what the household already knows about the product.
function applyFrequentProduct(entry) {
    if (!entry || !entry.barcode) return;
    // The banner belongs to one product: the previous one must not leave its
    // categories and its storage hint under the new name.
    clearProductInfo();
    showBarcode(entry.barcode);
    if (el.productName) el.productName.value = entry.productName || '';
    if (el.packSize) el.packSize.value = entry.packSize || '';
    if (el.productInfoName) el.productInfoName.textContent = entry.productName || '';
    setProductImage(entry.imageUrl, entry.productName || '');
    selectStoredCategory(entry.categoryKey);
    selectStoredLocation(entry.storageLocationId);
    // The suggestion carries no date: how long a pack lasts is not something
    // the household's history can answer. The category does — the household
    // gave it a shelf life — so the hint names it and the user sets the date.
    // A hand-picked category fills the date instead; the tap must not.
    showExpiryShelfLife();
    // Revealing the product data is what advances the dialog to the form
    // step, and the barcode lookup tells the form whether the household
    // already holds the product.
    storeBarcode(entry.barcode);
    checkBarcode(entry.barcode);
}

// UI toggle functions
function toggleBtnScan(state) {
    if (el.btnScan) {
        if (state) {
            el.btnScan.dataset.action = 'scan';
            el.btnScan.innerHTML = '<i class="bi bi-upc-scan px-3"></i>' + proviant.t("js.scan.scanButton");
        } else {
            el.btnScan.dataset.action = 'stop';
            el.btnScan.innerHTML = '<i class="bi bi-stop-circle px-3"></i>' + proviant.t("js.scan.stopButton");
        }
    }
}
function toggleBtnAddProduct(state) {
    if (el.btnAddProduct) {
        if (state) {
            el.btnAddProduct.disabled = false;
        } else {
            el.btnAddProduct.disabled = true;
        }
    }
}
function togglePlaceholders(state) {
    const placeholders = document.querySelectorAll('.placeholder');
    if (state) {
        placeholders.forEach(element => {
            element.classList.remove('d-none');
        });
    } else {
        placeholders.forEach(element => {
            element.classList.add('d-none');
        });
    }
}
function toggleGrowers(state) {
    if (state) {
        document.querySelectorAll('.spinner-grow.spinner-sm.fg-secondary').forEach((elem) => {
            elem.style.display = '';
        });
    } else {
        document.querySelectorAll('.spinner-grow.spinner-sm.fg-secondary').forEach((elem) => {
            elem.style.display = 'none';
        });
    }
}
function clearScanUI() {
    toggleGrowers(false);
    toggleBtnScan(true);
    toggleBtnAddProduct(true);
    togglePlaceholders(false);
    if (el.barcodeReaderWrapper) el.barcodeReaderWrapper.style.display = 'none';
    if (html5QrCode.getState() == Html5QrcodeScannerState.SCANNING) {
        html5QrCode.stop();
        html5QrCode.clear();
    }
};
function setProductOptionsState(productState, products) {
    const btnAdd = el.btnAddProduct;
    const btnShow = el.btnShowProduct;
    const btnDeleteModal = el.btnDeleteProductModal;
    const btnArchiveModal = el.btnArchiveProductModal;
    const btnShowAll = el.btnShowProducts;
    const instanceDropdown = el.instanceDropdown;

    // Clear all states
    btnAdd.disabled = true;
    btnShow.disabled = true;
    btnDeleteModal.disabled = true;
    btnArchiveModal.disabled = true;
    btnShowAll.disabled = true;
    instanceDropdown.classList.add('d-none');
    instanceDropdown.innerHTML = "";

    switch (productState) {
        case ProductState.NEW:
            btnAdd.disabled = false;
            btnShow.disabled = true;
            btnDeleteModal.disabled = true;
            btnArchiveModal.disabled = true;
            btnShowAll.disabled = true;
            instanceDropdown.classList.add('d-none');
            break;
        case ProductState.PRESENT:
            btnAdd.disabled = false;
            btnShow.disabled = false;
            btnDeleteModal.disabled = false;
            btnArchiveModal.disabled = false;
            btnShowAll.disabled = false;
            instanceDropdown.classList.remove('d-none');
            // Add new options based on the array
            products.forEach(product => {
                const option = document.createElement("option");
                option.value = product.id; // Set the value to the instance ID
                option.textContent = product.label;
                instanceDropdown.appendChild(option);
            });
            break;
        default:
            btnAdd.disabled = false;
            btnShow.disabled = true;
            btnDeleteModal.disabled = true;
            btnArchiveModal.disabled = true;
            btnShowAll.disabled = true;
            instanceDropdown.classList.add('d-none');
            console.error(`Invalid product state '${productState}'`);
            break;
    }
}
function parseInputDate(value) {
    const [year, month, day] = value.split('-').map(Number);
    return new Date(year, month - 1, day);
}

function formatInputDate(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
}

function shiftExpiry(days, months) {
    const inputEl = el.inputExpireAt;
    if (!inputEl) return;
    const base = inputEl.value ? parseInputDate(inputEl.value) : new Date();
    if (days !== 0) base.setDate(base.getDate() + days);
    if (months !== 0) base.setMonth(base.getMonth() + months);
    inputEl.value = formatInputDate(base);
    // A quick adjustment is the user's own date: a later suggestion must not
    // overwrite it.
    inputEl.dataset.userEdited = '1';
    inputEl.classList.remove('date-updated');
    void inputEl.offsetWidth;
    inputEl.classList.add('date-updated');
    inputEl.addEventListener('animationend', () => inputEl.classList.remove('date-updated'), { once: true });
}

// ── Batch scan mode ──────────────────────────────────────────────────────────

const BATCH_QUEUE_KEY = 'proviant_batch_scan_queue';

let batchMode = false;
let batchAwaitingConfirm = false;
// True while the bulk POST is in flight. The indices in the server's error
// list refer to the snapshot submitted with that request, so Add, Undo, and
// new decodes are blocked until the response arrives.
let batchSubmitting = false;
let batchPendingBarcode = '';
let batchPendingName = '';
let batchLocationId = null;
let batchQueue = [];
// In-flight html5QrCode.start() promise. getState() stays UNKNOWN until the
// start resolves, so clearScanUI/stop() cannot see a camera that is still
// starting; this promise is what makes that window safe to exit.
let batchStartPromise = null;

function saveBatchQueue() {
    try {
        sessionStorage.setItem(BATCH_QUEUE_KEY, JSON.stringify(batchQueue));
    } catch (error) {
        console.error('Could not persist batch queue', error);
    }
}

function restoreBatchQueue() {
    try {
        const stored = sessionStorage.getItem(BATCH_QUEUE_KEY);
        if (!stored) return;
        const parsed = JSON.parse(stored);
        if (Array.isArray(parsed)) {
            batchQueue = parsed.filter((item) => item && item.barcode);
            updateBatchTally();
        }
    } catch (error) {
        // Corrupt storage must not break the page; drop the queue instead.
        console.error('Could not restore batch queue', error);
        try { sessionStorage.removeItem(BATCH_QUEUE_KEY); } catch (_) { /* noop */ }
    }
}

function updateBatchTally() {
    if (!el.batchTally) return;
    const countEl = document.getElementById('batchTallyCount');
    const lastEl = document.getElementById('batchTallyLast');
    if (batchQueue.length > 0) {
        if (countEl) countEl.textContent = proviant.tn("js.scan.batchQueued", batchQueue.length);
        if (lastEl) lastEl.textContent = batchQueue[batchQueue.length - 1].productName || batchQueue[batchQueue.length - 1].barcode;
        el.batchTally.classList.remove('d-none');
    } else {
        if (lastEl) lastEl.textContent = '';
        el.batchTally.classList.add('d-none');
    }
}

function setBatchToggleLabel() {
    const btn = document.getElementById('btnBatchToggle');
    if (!btn) return;
    if (batchMode) {
        btn.innerHTML = '<i class="bi bi-x-circle" aria-hidden="true"></i>' + proviant.t("js.scan.exitBatchMode");
    } else {
        btn.innerHTML = '<i class="bi bi-stack" aria-hidden="true"></i>' + proviant.t("js.scan.batchScanButton");
    }
}

function setBatchScanLine(active) {
    const scanLine = document.getElementById('scanLine');
    if (!scanLine) return;
    if (active) {
        scanLine.classList.add('active');
    } else {
        scanLine.classList.remove('active');
    }
}

async function loadBatchStorageLocations() {
    const select = el.batchStorageLocation;
    if (!select) return;
    try {
        const response = await proviant.getStorageLocations();
        if (response.code === 200 && Array.isArray(response.message)) {
            response.message.forEach((location) => {
                const option = document.createElement('option');
                option.value = location.id;
                option.textContent = location.name || location.Name;
                select.appendChild(option);
            });
        }
    } catch (error) {
        console.error('Could not load storage locations', error);
    }
}

function handleBatchToggle() {
    if (batchMode) {
        exitBatchMode();
        return;
    }
    if (!el.batchStartPanel) return;
    el.batchStartPanel.classList.toggle('d-none');
    if (!el.batchStartPanel.classList.contains('d-none') && el.batchStorageLocation && el.batchStorageLocation.options.length <= 1) {
        void loadBatchStorageLocations();
    }
}

function handleBatchStart() {
    if (!el.batchStartPanel || !el.batchStorageLocation) return;
    batchLocationId = el.batchStorageLocation.value ? Number.parseInt(el.batchStorageLocation.value, 10) : null;
    batchMode = true;
    batchAwaitingConfirm = false;
    el.batchStartPanel.classList.add('d-none');
    const scanActions = document.querySelector('.scan-actions');
    if (scanActions) scanActions.classList.add('d-none');
    if (el.productDataWrapper) el.productDataWrapper.classList.add('d-none');
    setBatchToggleLabel();
    startBatchCamera();
}

function startBatchCamera() {
    if (batchStartPromise) return;
    if (html5QrCode.getState() === Html5QrcodeScannerState.SCANNING) {
        // The camera is live but may still carry the single-flow decode
        // callback (user tapped "Scan barcode" before switching to batch).
        // Restart it so decodes reach the batch confirm panel.
        restartBatchCamera();
        return;
    }
    toggleGrowers(true);
    if (el.barcodeReaderWrapper) el.barcodeReaderWrapper.style.display = '';
    setBatchScanLine(true);
    batchStartPromise = html5QrCode.start(
        { facingMode: 'environment' },
        {
            fps: 10,
            qrbox: { width: 240, height: 100 }
        },
        (decodedText) => handleBatchDecode(decodedText)
    ).then(() => {
        batchStartPromise = null;
    }).catch((error) => {
        batchStartPromise = null;
        if (!batchMode) return;
        proviant.showFeedback('error', proviant.t("js.scan.cameraError"), String(error));
        exitBatchMode();
    });
}

function restartBatchCamera() {
    setBatchScanLine(false);
    html5QrCode.stop().then(() => {
        html5QrCode.clear();
        if (batchMode) startBatchCamera();
    }).catch(() => {
        // A failed stop means the camera was not really running; a fresh
        // start is still the right recovery.
        if (batchMode) startBatchCamera();
    });
}

function exitBatchCamera() {
    if (!batchStartPromise) return;
    const pending = batchStartPromise;
    batchStartPromise = null;
    // The pending start() resolves after this function returns, so the camera
    // only becomes visible (and decodable) afterwards — stop it there.
    pending.finally(() => {
        if (html5QrCode.getState() === Html5QrcodeScannerState.SCANNING) {
            html5QrCode.stop().then(() => html5QrCode.clear()).catch(() => {});
        }
    });
}

function exitBatchMode() {
    batchMode = false;
    batchAwaitingConfirm = false;
    batchPendingBarcode = '';
    batchPendingName = '';
    setBatchScanLine(false);
    exitBatchCamera();
    // clearScanUI stops the camera but leaves the queue and tally intact so
    // re-entering batch mode resumes where the user left off.
    clearScanUI();
    if (el.batchConfirmPanel) el.batchConfirmPanel.classList.add('d-none');
    if (el.batchStartPanel) el.batchStartPanel.classList.add('d-none');
    const scanActions = document.querySelector('.scan-actions');
    if (scanActions) scanActions.classList.remove('d-none');
    setBatchToggleLabel();
}

function handleBatchDecode(barcode) {
    if (!batchMode || batchAwaitingConfirm || batchSubmitting) return;
    batchAwaitingConfirm = true;
    batchPendingBarcode = barcode;
    openBatchConfirmPanel(barcode);
}

function openBatchConfirmPanel(barcode) {
    if (!el.batchConfirmPanel) return;
    batchPendingName = '';
    if (el.batchProductName) el.batchProductName.textContent = proviant.t("js.scan.lookingUp");
    if (el.batchProductBarcode) el.batchProductBarcode.textContent = barcode;
    if (el.batchQuantity) el.batchQuantity.value = 1;
    if (el.batchQuantityDisplay) el.batchQuantityDisplay.textContent = '1';
    if (el.batchExpireAt) {
        el.batchExpireAt.value = formatInputDate(new Date());
        el.batchExpireAt.classList.remove('is-invalid');
    }
    el.batchConfirmPanel.classList.remove('d-none');
    el.batchConfirmPanel.scrollIntoView({ behavior: 'smooth', block: 'nearest' });

    // The lookup is non-blocking: the panel opens immediately with a
    // placeholder and the name fills in when Open Food Facts answers. A miss
    // is fine — the server re-resolves the name when the batch is saved.
    proviant.getOpenFoodFactsData(barcode).then((response) => {
        if (!batchAwaitingConfirm || batchPendingBarcode !== barcode) return;
        if (response.code === 200 && response.message.productName) {
            batchPendingName = response.message.productName;
            if (el.batchProductName) el.batchProductName.textContent = batchPendingName;
        } else if (el.batchProductName) {
            el.batchProductName.textContent = barcode;
        }
    }).catch(() => {
        if (batchAwaitingConfirm && batchPendingBarcode === barcode && el.batchProductName) {
            el.batchProductName.textContent = barcode;
        }
    });
}

function closeBatchConfirmPanel() {
    batchAwaitingConfirm = false;
    batchPendingBarcode = '';
    batchPendingName = '';
    if (el.batchConfirmPanel) el.batchConfirmPanel.classList.add('d-none');
    // The scanner must be back in SCANNING before the next decode; if its
    // state drifted (e.g. a stop raced the panel), revive it here.
    if (batchMode && !batchStartPromise &&
        html5QrCode.getState() !== Html5QrcodeScannerState.SCANNING) {
        startBatchCamera();
    }
}

function shiftBatchExpiry(days, months) {
    const inputEl = el.batchExpireAt;
    if (!inputEl) return;
    const base = inputEl.value ? parseInputDate(inputEl.value) : new Date();
    if (days !== 0) base.setDate(base.getDate() + days);
    if (months !== 0) base.setMonth(base.getMonth() + months);
    inputEl.value = formatInputDate(base);
}

function handleBatchAdd() {
    if (batchSubmitting) return;
    if (!batchAwaitingConfirm || !el.batchExpireAt || !el.batchQuantity) return;
    if (!el.batchExpireAt.value) {
        el.batchExpireAt.classList.add('is-invalid');
        return;
    }
    // valueAsDate is UTC midnight, matching the single-create path; a
    // local-midnight Date would shift the stored day back one in UTC+ zones.
    const expireDate = el.batchExpireAt.valueAsDate;
    batchQueue.push({
        barcode: batchPendingBarcode,
        productName: batchPendingName,
        expireAt: expireDate.toISOString(),
        quantity: Number.parseInt(el.batchQuantity.value, 10) || 1,
        storageLocationId: batchLocationId
    });
    saveBatchQueue();
    updateBatchTally();
    closeBatchConfirmPanel();
}

function handleBatchSkip() {
    closeBatchConfirmPanel();
}

function handleBatchUndo() {
    if (batchSubmitting || batchQueue.length === 0) return;
    batchQueue.pop();
    saveBatchQueue();
    updateBatchTally();
}

async function handleBatchDone() {
    if (batchQueue.length === 0 || batchSubmitting) return;
    const btn = document.getElementById('btnBatchDone');
    if (!btn) return;
    batchSubmitting = true;
    btn.disabled = true;
    btn.classList.add('loading');
    try {
        const response = await proviant.bulkCreateProducts(batchQueue);
        if (response.code === 200) {
            const failedIndices = new Set((response.errors || []).map((itemError) => itemError.index));
            if (failedIndices.size === 0) {
                batchQueue = [];
                saveBatchQueue();
                updateBatchTally();
                // showFeedback is a no-op when the feedback modal is missing,
                // so the redirect must not depend on its onClose firing.
                let redirected = false;
                const goToList = () => {
                    if (redirected) return;
                    redirected = true;
                    globalThis.location = '/web/products';
                };
                proviant.showFeedback('success', proviant.t("js.scan.batchSaved"), proviant.tn("js.scan.batchCreatedCount", response.created), goToList);
                setTimeout(goToList, 1600);
            } else {
                // Created items must leave the queue; failed items move to the
                // tail so tail-undo reaches them without discarding anything
                // that has not been tried yet.
                const failedItems = batchQueue.filter((_, index) => failedIndices.has(index));
                batchQueue = batchQueue.filter((_, index) => !failedIndices.has(index)).concat(failedItems);
                saveBatchQueue();
                updateBatchTally();
                proviant.showFeedback('warning', proviant.t("js.scan.batchPartiallySaved"), proviant.t("js.scan.batchPartialSummary", response.created, response.failed));
            }
        } else {
            proviant.showFeedback('error', proviant.t("js.scan.batchFailed"), response.message || proviant.t("js.scan.batchSaveFailed"));
        }
    } catch (_) {
        proviant.showFeedback('error', proviant.t("js.scan.batchFailed"), proviant.t("js.scan.batchNetworkError"));
    } finally {
        batchSubmitting = false;
        btn.disabled = false;
        btn.classList.remove('loading');
    }
}

// Async functions
async function queryProductInfoRequest(barcode) {
    return proviant.getOpenFoodFactsData(barcode).catch(() => {
        const bc = el.inputBarcode ? el.inputBarcode.value : 'unknown';
        showError(`Could not find product with barcode ${bc}!`);
    });
}
// Function handlers
function queryProductInfo(barcode) {
    clearProductInfo();
    // A new product starts with an untouched expiry field, so the shelf life
    // of its category may fill it again.
    if (el.inputExpireAt) {
        delete el.inputExpireAt.dataset.userEdited;
        delete el.inputExpireAt.dataset.suggested;
    }
    // Another barcode is another product: the naming of the last one does not
    // carry over. The lookup fills the fields again if it knows the product.
    if (el.productName) el.productName.value = '';
    if (el.packSize) el.packSize.value = '';
    setBarcodeLoading(true);
    queryProductInfoRequest(barcode).then((response) => {
        setBarcodeLoading(false);
        if (response && response.code === 200) {
            showProductData(response.message);
        } else {
            showError(`Could not find product with barcode ${barcode}`);
            clearProductInfo();
        }
    }).catch((error) => {
        setBarcodeLoading(false);
        showError('Error: ' + error);
        clearProductInfo();
    });
}
function storeBarcode(barcode) {
    if (el.inputBarcode) {
        el.inputBarcode.dataset.barcode = barcode;
        el.inputBarcode.value = barcode;
    }
    if (el.productDataWrapper) el.productDataWrapper.classList.remove('d-none');
}
function getSelectedProductName() {
    const dropdown = el.instanceDropdown;
    if (!dropdown || dropdown.selectedIndex < 0) return '';
    return dropdown[dropdown.selectedIndex].textContent.split(';')[0].trim();
}
function checkBarcode(barcode) {
  proviant.getProductsByBarcode(barcode).then((response) => {
    switch (response.code) {
      case 200:
        if (response.message.length > 0) {
          setProductOptionsState(ProductState.PRESENT, response.message);
        } else {
          setProductOptionsState(ProductState.NEW, []);
        }
        break;
      case 400:
        setProductOptionsState(ProductState.NEW, []);
        showAlert(false, proviant.t("js.scan.invalidRequestData"));
        break;
      case 500:
        setProductOptionsState(ProductState.NEW, []);
        showAlert(false, proviant.t("js.scan.backendServerError"));
        break;
      default:
        setProductOptionsState(ProductState.NEW, []);
        showAlert(false, proviant.t("js.products.undefinedError", response.message));
        break;
    }
  }).catch(error => {
    console.error(error);
  });
};

// Button handlers
function handleScanButton() {
    const btnScan = el.btnScan;
    if (btnScan.dataset.action == 'scan') {
        toggleGrowers(true);
        toggleBtnScan(false);
        if (el.barcodeReaderWrapper) el.barcodeReaderWrapper.style.display = '';
        html5QrCode.start(
            { facingMode: 'environment' },
            {
                fps: 10,
                qrbox: { width: 240, height: 100 }
            },
            (decodedText) => {
                queryProductInfo(decodedText);
                showBarcode(decodedText)
                storeBarcode(decodedText);
                clearScanUI();
            }
        ).catch(error => {
            showAlert(false, error);
            clearScanUI();
        });
    } else if (btnScan.dataset.action == 'stop') {
        clearScanUI();
    } else {
        showAlert(false, proviant.t("js.scan.undefinedDataAction", btnScan.dataset.action));
    }
};
function handleBtnAddProduct() {
    if (!el.inputBarcode || !el.inputExpireAt) return;
    // The add button sits outside the form, so the browser never validates on
    // its own. Without asking the field to report, an add that cannot happen
    // says nothing at all.
    if (!el.inputBarcode.checkValidity()) {
        el.inputBarcode.reportValidity();
        return;
    }
    if (!el.inputExpireAt.checkValidity()) {
        el.inputExpireAt.reportValidity();
        return;
    }

    const barcode = el.inputBarcode.value;
    if (barcode === '') {
        showAlert(false, proviant.t("js.scan.barcodeEmpty"));
        return;
    }
    const expireAt = el.inputExpireAt.valueAsDate.toISOString();
    const qtyEl = el.quantity;
    const quantity = qtyEl ? Number.parseInt(qtyEl.value, 10) || 1 : 1;
    const locationEl = el.modalStorageLocation;
    const storageLocationId = locationEl && locationEl.value ? Number.parseInt(locationEl.value, 10) : null;
    const categoryEl = el.productCategory;
    const categoryKey = categoryEl && categoryEl.value ? categoryEl.value : null;
    // The name and the pack size the household wrote are sent along, so the
    // row keeps the naming this household uses.
    const details = {
        productName: el.productName ? el.productName.value.trim() : '',
        packSize: el.packSize ? el.packSize.value.trim() : '',
    };

    const btn = el.btnAddProduct;
    if (!btn) return;
    btn.classList.add('loading');
    btn.disabled = true;

    proviant.createProduct(barcode, expireAt, quantity, storageLocationId, false, null, false, categoryKey, details).then((response) => {
        btn.classList.remove('loading');
        btn.disabled = false;
        switch (response.code) {
            case 200:
            case 201:
                showAlert(true, proviant.t("js.products.productCreatedBarcode", barcode));
                break;
            case 202:
                showAlert(true, proviant.t("js.products.productQueuedOffline"));
                break;
            case 409:
                // A conflict that names no duplicate — a clientSyncId another
                // member's private row holds — has nothing to offer.
                if (!response.duplicate) {
                    showAlert(false, response.message);
                    break;
                }
                openDuplicatePrompt(response, { barcode: barcode, expireAt: expireAt, quantity: quantity, storageLocationId: storageLocationId, categoryKey: categoryKey, details: details });
                break;
            case 400:
                showAlert(false, proviant.t("js.scan.invalidRequestData"));
                break;
            case 500:
                showAlert(false, proviant.t("js.scan.backendServerError"));
                break;
            default:
                showAlert(false, proviant.t("js.products.undefinedError", response.message));
                break;
        }
    }).catch((error) => {
        btn.classList.remove('loading');
        btn.disabled = false;
        showAlert(false, error);
    });
}

// The server refused the create because the household already has this
// product: the same barcode, the same expiry date, the same storage
// location. The user decides whether the pack belongs to that row, is a
// product of its own, or was a mis-scan.
function openDuplicatePrompt(response, draft) {
    if (typeof proviant.showDuplicatePrompt !== 'function') {
        showAlert(false, proviant.t("js.products.duplicateUnavailable"));
        return;
    }

    proviant.showDuplicatePrompt({
        duplicate: response.duplicate,
        quantity: draft.quantity,
        onAddPack: function (duplicate, quantity) {
            proviant.updateProductQuantity(duplicate.id, quantity).then((result) => {
                if (result.code !== 200 && result.code !== 202) {
                    showAlert(false, proviant.t("js.products.duplicateAddPackFailed", result.message));
                    return;
                }
                updateQuantityInPlace(duplicate, quantity);
                showAlert(true, proviant.tn("js.products.duplicateAddedPack", quantity, duplicate.productName));
            }).catch((error) => {
                showAlert(false, error);
            });
        },
        onAddSeparately: function () {
            // The same id as the refused attempt: nothing was written under it,
            // so the row this creates is idempotent as well.
            proviant.createProduct(draft.barcode, draft.expireAt, draft.quantity, draft.storageLocationId, false, response.clientSyncId, true, draft.categoryKey, draft.details).then((result) => {
                if (result.code === 200 || result.code === 201) {
                    showAlert(true, proviant.t("js.products.productCreatedBarcode", draft.barcode));
                    return;
                }
                // A queued create is a create: the row is written when the
                // outbox drains, so it is not a failure.
                if (result.code === 202) {
                    showAlert(true, proviant.t("js.products.productQueuedOffline"));
                    return;
                }
                showAlert(false, proviant.t("js.products.duplicateAddSeparatelyFailed", result.message));
            }).catch((error) => {
                showAlert(false, error);
            });
        },
        onCancel: function () {
            // Nothing was written, and nothing is written now.
        },
    });
}

// The merged row keeps its place in the list: its count grows by the packs
// that were added. The count stops at the bound a product may hold, the same
// bound the server writes.
function updateQuantityInPlace(duplicate, quantity) {
    const qtyEl = document.getElementById('qty-' + duplicate.id);
    if (!qtyEl) return;
    const maxQuantity = (window.proviantLimits && window.proviantLimits.maxQuantity) || 999;
    qtyEl.dataset.quantity = String(Math.min(duplicate.quantity + quantity, maxQuantity));
    if (typeof proviant.renderQuantityLabel === 'function') {
        proviant.renderQuantityLabel(qtyEl);
    }
}

function handleBtnShowProduct() {
    const dropdown = el.instanceDropdown;
    const productId = dropdown[dropdown.selectedIndex].value;
    globalThis.location = `/web/products/${productId}/edit`;
}
function handleBtnDeleteProductModal() {
    const dropdown = el.instanceDropdown;
    const productId = dropdown[dropdown.selectedIndex].value;
    const productName = getSelectedProductName();
    proviant.showConfirm(
        proviant.t("js.products.markAsWasted"),
        proviant.t("js.products.markWastedNamedConfirm", productName),
        function () {
            proviant.deleteProduct(productId, false).then((response) => {
                if (response.code == 200) {
                    globalThis.location.reload();
                } else {
                    proviant.showFeedback('error', proviant.t("js.products.wastedFailed"), proviant.t("js.products.wastedError", response.message));
                }
            }).catch((error) => {
                showAlert(false, error);
            });
        },
        proviant.t("js.products.wastedButton"),
        'danger'
    );
}

function handleBtnArchiveProductModal() {
    const dropdown = el.instanceDropdown;
    const productId = dropdown[dropdown.selectedIndex].value;
    const productName = getSelectedProductName();
    proviant.showConfirm(
        proviant.t("js.products.markAsConsumed"),
        proviant.t("js.products.markConsumedConfirm", productName),
        function () {
            proviant.deleteProduct(productId, true).then((response) => {
                if (response.code == 200) {
                    globalThis.location.reload();
                } else {
                    proviant.showFeedback('error', proviant.t("js.products.consumeFailed"), proviant.t("js.products.consumeError", response.message));
                }
            }).catch((error) => {
                showAlert(false, error);
            });
        },
        proviant.t("js.products.consumedButton"),
        'warning'
    );
}
function handleBtnShowProducts() {
    const bc = el.inputBarcode ? el.inputBarcode.value : '';
    globalThis.location = `/web/products?queryParam=barcode&queryValue=${bc}`;
}

// Input handlers
function handleChangedBarcode() {
    if (batchMode) return;
    if (!el.inputBarcode) return;
    if (!(el.inputBarcode.checkValidity())) {
        if (el.inputBarcode.classList.contains('border-success')) {
            el.inputBarcode.classList.remove('border-success')
        }
        return;
    }
    const barcode = el.inputBarcode.value;
    queryProductInfo(barcode);
    showBarcode(barcode)
    storeBarcode(barcode);
    clearScanUI();
    checkBarcode(barcode);
};

// Add event listeners — all use event delegation to avoid stale references

/* Event delegation for form submission */
document.addEventListener('submit', function (event) {
    const target = event.target;
    if (target.matches('form[data-bs-validate]')) {
        if (target.checkValidity() === false) {
            event.preventDefault();
            event.stopPropagation();
        } else {
            event.preventDefault();
            handleBtnAddProduct();
        }
    }
});

/* The name and the pack size are bounded in bytes, because the columns are:
   a character bound would let a name of multi-byte characters exceed what the
   database stores. The field is trimmed to the same rule the server applies,
   so what the user sees is what the row keeps. */
function trimToBytes(value, limit) {
    const encoder = new TextEncoder();
    if (encoder.encode(value).length <= limit) return value;
    let bytes = 0;
    let end = 0;
    for (const character of value) {
        const size = encoder.encode(character).length;
        if (bytes + size > limit) break;
        bytes += size;
        end += character.length;
    }
    return value.slice(0, end);
}

function enforceByteLimit(target) {
    const limits = window.proviantLimits || {};
    const limit = target.id === 'productName' ? limits.maxProductNameLength : limits.maxPackSizeLength;
    if (!limit) return;
    const trimmed = trimToBytes(target.value, limit);
    if (trimmed !== target.value) target.value = trimmed;
}

/* Event delegation for all clicks */
document.addEventListener('click', function (event) {
    const target = event.target;

    const frequentProduct = target.closest('.frequent-product');
    if (frequentProduct) {
        event.preventDefault();
        applyFrequentProduct({
            barcode: frequentProduct.dataset.barcode,
            productName: frequentProduct.dataset.productName,
            packSize: frequentProduct.dataset.packSize,
            categoryKey: frequentProduct.dataset.categoryKey,
            storageLocationId: frequentProduct.dataset.storageLocationId,
            imageUrl: frequentProduct.dataset.imageUrl,
        });
        return;
    }

    if (target.closest('#btnAddProduct')) {
        event.preventDefault();
        handleBtnAddProduct();
        return;
    }

    if (target.closest('#btnShowProduct')) {
        event.preventDefault();
        handleBtnShowProduct();
        return;
    }

    if (target.closest('#btnDeleteProductModal')) {
        event.preventDefault();
        handleBtnDeleteProductModal();
        return;
    }

    if (target.closest('#btnArchiveProductModal')) {
        event.preventDefault();
        handleBtnArchiveProductModal();
        return;
    }

    if (target.closest('#btnShowProducts')) {
        event.preventDefault();
        handleBtnShowProducts();
        return;
    }

    if (target.closest('#btnScan')) {
        event.preventDefault();
        handleScanButton();
        return;
    }

    if (target.closest('#btnQuantityDec')) {
        event.preventDefault();
        const hiddenInput = el.quantity;
        const display = el.quantityDisplay;
        if (hiddenInput && display) {
            const newVal = Math.max(1, Number.parseInt(hiddenInput.value, 10) - 1);
            hiddenInput.value = newVal;
            display.textContent = newVal;
        }
        return;
    }

    if (target.closest('#btnQuantityInc')) {
        event.preventDefault();
        const hiddenInput = el.quantity;
        const display = el.quantityDisplay;
        if (hiddenInput && display) {
            const newVal = Number.parseInt(hiddenInput.value, 10) + 1;
            hiddenInput.value = newVal;
            display.textContent = newVal;
        }
        return;
    }

    if (target.closest('#btnExpireAdd3')) {
        event.preventDefault();
        shiftExpiry(3, 0);
        return;
    }

    if (target.closest('#btnExpireAdd7')) {
        event.preventDefault();
        shiftExpiry(7, 0);
        return;
    }

    if (target.closest('#btnExpireAdd1m')) {
        event.preventDefault();
        shiftExpiry(0, 1);
        return;
    }

    // Batch scan mode
    if (target.closest('#btnBatchToggle')) {
        event.preventDefault();
        handleBatchToggle();
        return;
    }

    if (target.closest('#btnBatchStart')) {
        event.preventDefault();
        handleBatchStart();
        return;
    }

    if (target.closest('#btnBatchCancelStart')) {
        event.preventDefault();
        if (el.batchStartPanel) el.batchStartPanel.classList.add('d-none');
        return;
    }

    if (target.closest('#btnBatchQuantityDec')) {
        event.preventDefault();
        const hiddenInput = el.batchQuantity;
        const display = el.batchQuantityDisplay;
        if (hiddenInput && display) {
            const newVal = Math.max(1, Number.parseInt(hiddenInput.value, 10) - 1);
            hiddenInput.value = newVal;
            display.textContent = newVal;
        }
        return;
    }

    if (target.closest('#btnBatchQuantityInc')) {
        event.preventDefault();
        const hiddenInput = el.batchQuantity;
        const display = el.batchQuantityDisplay;
        if (hiddenInput && display) {
            const newVal = Number.parseInt(hiddenInput.value, 10) + 1;
            hiddenInput.value = newVal;
            display.textContent = newVal;
        }
        return;
    }

    if (target.closest('#btnBatchExpireAdd3')) {
        event.preventDefault();
        shiftBatchExpiry(3, 0);
        return;
    }

    if (target.closest('#btnBatchExpireAdd7')) {
        event.preventDefault();
        shiftBatchExpiry(7, 0);
        return;
    }

    if (target.closest('#btnBatchExpireAdd1m')) {
        event.preventDefault();
        shiftBatchExpiry(0, 1);
        return;
    }

    if (target.closest('#btnBatchAdd')) {
        event.preventDefault();
        handleBatchAdd();
        return;
    }

    if (target.closest('#btnBatchSkip')) {
        event.preventDefault();
        handleBatchSkip();
        return;
    }

    if (target.closest('#btnBatchUndo')) {
        event.preventDefault();
        handleBatchUndo();
        return;
    }

    if (target.closest('#btnBatchDone')) {
        event.preventDefault();
        void handleBatchDone();
        return;
    }

    // Manual barcode entry queues into the batch while batch mode is active.
    // The modal closes itself via data-bs-dismiss; the single-flow lookup is
    // skipped because the batch panel takes over.
    if (target.closest('#btnManualLookup')) {
        if (!batchMode || batchSubmitting) return;
        event.preventDefault();
        const barcodeInput = document.getElementById('barcode');
        const barcode = barcodeInput ? barcodeInput.value.trim() : '';
        if (!barcode) {
            return;
        }
        // Same gate as the single flow: an invalid barcode would be
        // deterministically rejected by the server and re-queued forever.
        if (!barcodeInput.checkValidity()) {
            barcodeInput.classList.add('is-invalid');
            return;
        }
        barcodeInput.classList.remove('is-invalid');
        handleBatchDecode(barcode);
        return;
    }
});

/* Event delegation for barcode input */
document.addEventListener('input', function (event) {
    const target = event.target;
    if (batchMode) return;
    if (target.id === 'barcode' || target.name === 'barcode') {
        handleChangedBarcode();
    }
});

/* A date the user set is theirs: a later suggestion must not overwrite it.
   The change event covers the expiry scan, which writes the field and fires
   it. */
document.addEventListener('input', function (event) {
    const target = event.target;
    if (target.id !== 'expireAt') return;
    target.dataset.userEdited = '1';
    delete target.dataset.suggested;
});

/* The free-text fields are bounded in bytes, like their columns. */
document.addEventListener('input', function (event) {
    const target = event.target;
    if (target.id !== 'productName' && target.id !== 'packSize') return;
    enforceByteLimit(target);
});

document.addEventListener('change', function (event) {
    const target = event.target;
    if (target.id === 'expireAt') {
        target.dataset.userEdited = '1';
        delete target.dataset.suggested;
        return;
    }
    if (target.id === 'productCategory') {
        applyCategorySuggestion();
    }
});

/* Restore a persisted batch queue after navigation */
document.addEventListener('DOMContentLoaded', function () {
    restoreBatchQueue();
});

/* The staples are loaded when the add-product dialog opens: the household
   buys between two adds, and the list is a server response anyway. The scan
   page has no dialog, so nothing is loaded there. */
const addProductModal = document.getElementById('addProductModal');
if (addProductModal) {
    addProductModal.addEventListener('shown.bs.dialog', function () {
        void loadFrequentProducts();
    });
}

/* A product picture that is gone — the image cache emptied, the file
   removed — has nothing to show, and a broken icon is worse than none. */
if (el.productInfoImage) {
    el.productInfoImage.addEventListener('error', function () {
        el.productInfoImage.classList.add('d-none');
    });
}

/* Mobile Safari suspends the video track while the page is backgrounded
   without changing html5-qrcode's internal state, so the scanner looks
   SCANNING but never decodes again. On return, restart the camera when its
   video is no longer playing. */
document.addEventListener('visibilitychange', function () {
    if (document.visibilityState !== 'visible' || !batchMode || batchStartPromise) return;
    if (html5QrCode.getState() !== Html5QrcodeScannerState.SCANNING) return;
    const video = document.querySelector('#barcode-reader video');
    if (video && !video.paused) return;
    restartBatchCamera();
});
