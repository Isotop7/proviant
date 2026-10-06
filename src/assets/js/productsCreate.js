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
    amount: document.getElementById('amount'),
    amountDisplay: document.getElementById('amountDisplay'),
    modalStorageLocation: document.getElementById('modalStorageLocation'),
    btnAmountDec: document.getElementById('btnAmountDec'),
    btnAmountInc: document.getElementById('btnAmountInc'),
    btnExpireAdd3: document.getElementById('btnExpireAdd3'),
    btnExpireAdd7: document.getElementById('btnExpireAdd7'),
    btnExpireAdd1m: document.getElementById('btnExpireAdd1m'),
    // Batch scan mode — only present on the scan page, every use is guarded
    batchStartPanel: document.getElementById('batchStartPanel'),
    batchConfirmPanel: document.getElementById('batchConfirmPanel'),
    batchTally: document.getElementById('batchTally'),
    batchStorageLocation: document.getElementById('batchStorageLocation'),
    batchProductName: document.getElementById('batchProductName'),
    batchProductBarcode: document.getElementById('batchProductBarcode'),
    batchAmount: document.getElementById('batchAmount'),
    batchAmountDisplay: document.getElementById('batchAmountDisplay'),
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
        proviant.showFeedback('success', 'Product Created', message || 'Product created successfully!');
    } else {
        proviant.showFeedback('error', 'Error', message || 'Failed to create product. Please try again.');
    }
}
function clearProductInfo() {
    if (el.productInfoImage) {
        el.productInfoImage.src = '';
        el.productInfoImage.alt = '';
    }
    if (el.productInfoName) el.productInfoName.innerText = '';
    if (el.productInfoGenericName) el.productInfoGenericName.innerText = '';
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
    if (el.productInfoImage) {
        el.productInfoImage.src = product.imageUrl;
        el.productInfoImage.alt = product.productName || 'Product image';
    }
    if (el.productInfoName) el.productInfoName.innerText = product.productName;
    if (product.categories != 'undefined' && product.categories != null) {
        if (el.productInfoGenericName) el.productInfoGenericName.innerText = product.categories;
    }
    if (el.storageHint) {
        if (product.storageHint) {
            el.storageHint.textContent = product.storageHint;
            el.storageHint.classList.remove('d-none');
        } else {
            el.storageHint.classList.add('d-none');
        }
    }
    if (el.productDataWrapper) el.productDataWrapper.classList.remove('d-none');
}

// UI toggle functions
function toggleBtnScan(state) {
    if (el.btnScan) {
        if (state) {
            el.btnScan.dataset.action = 'scan';
            el.btnScan.innerHTML = '<i class="bi bi-upc-scan px-2"></i>Scan';
        } else {
            el.btnScan.dataset.action = 'stop';
            el.btnScan.innerHTML = '<i class="bi bi-stop-circle px-2"></i>Stop';
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
        document.querySelectorAll('.spinner-grow.spinner-grow-sm.text-secondary').forEach((elem) => {
            elem.style.display = '';
        });
    } else {
        document.querySelectorAll('.spinner-grow.spinner-grow-sm.text-secondary').forEach((elem) => {
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
const formatProductDataAsOption = (product) => `${product.productName}; Created: ${proviant.formatDate(product.CreatedAt)}; Expire at: ${proviant.formatDate(product.expireAt)}; Notified at: ${proviant.formatDate(product.notifiedAt)}`;
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
                option.value = product.ID; // Set the value to the instance ID
                option.textContent = formatProductDataAsOption(product);
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
        if (countEl) countEl.textContent = `${batchQueue.length} item${batchQueue.length === 1 ? '' : 's'} queued`;
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
        btn.innerHTML = '<i class="bi bi-x-circle" aria-hidden="true"></i>Exit batch mode';
    } else {
        btn.innerHTML = '<i class="bi bi-stack" aria-hidden="true"></i>Batch scan';
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
                option.value = location.ID;
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
        loadBatchStorageLocations();
    }
}

function handleBatchStart() {
    if (!el.batchStartPanel || !el.batchStorageLocation) return;
    batchLocationId = el.batchStorageLocation.value ? parseInt(el.batchStorageLocation.value, 10) : null;
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
        proviant.showFeedback('error', 'Camera Error', String(error));
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
    if (el.batchProductName) el.batchProductName.textContent = 'Looking up…';
    if (el.batchProductBarcode) el.batchProductBarcode.textContent = barcode;
    if (el.batchAmount) el.batchAmount.value = 1;
    if (el.batchAmountDisplay) el.batchAmountDisplay.textContent = '1';
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
    if (!batchAwaitingConfirm || !el.batchExpireAt || !el.batchAmount) return;
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
        amount: parseInt(el.batchAmount.value, 10) || 1,
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
                proviant.showFeedback('success', 'Batch Saved', `${response.created} products created`, goToList);
                setTimeout(goToList, 1600);
            } else {
                // Created items must leave the queue; failed items move to the
                // tail so tail-undo reaches them without discarding anything
                // that has not been tried yet.
                const failedItems = batchQueue.filter((_, index) => failedIndices.has(index));
                batchQueue = batchQueue.filter((_, index) => !failedIndices.has(index)).concat(failedItems);
                saveBatchQueue();
                updateBatchTally();
                proviant.showFeedback('warning', 'Partially Saved', `${response.created} created, ${response.failed} failed — failed items stay queued.`);
            }
        } else {
            proviant.showFeedback('error', 'Batch Failed', response.message || 'Could not save the batch. Your queue is kept.');
        }
    } catch (_) {
        proviant.showFeedback('error', 'Batch Failed', 'Network error — your queue is kept.');
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
    return dropdown[dropdown.selectedIndex].innerText.split(';')[0].trim();
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
        showAlert(false, 'Request contained invalid data');
        break;
      case 500:
        setProductOptionsState(ProductState.NEW, []);
        showAlert(false, 'Backend server error');
        break;
      default:
        setProductOptionsState(ProductState.NEW, []);
        showAlert(false, `Undefined error: ${response.message}`);
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
        showAlert(false, `Undefined data-action '${btnScan.dataset.action}'`);
    }
};
function handleBtnAddProduct() {
    if (!el.inputBarcode || !el.inputExpireAt) return;
    if (!(el.inputBarcode.checkValidity() && el.inputExpireAt.checkValidity())) {
        return;
    }

    const barcode = el.inputBarcode.value;
    if (barcode === '') {
        showAlert(false, 'Barcode cannot be empty');
        return;
    }
    const expireAt = el.inputExpireAt.valueAsDate.toISOString();
    const amountEl = el.amount;
    const amount = amountEl ? parseInt(amountEl.value, 10) || 1 : 1;
    const locationEl = el.modalStorageLocation;
    const storageLocationId = locationEl && locationEl.value ? parseInt(locationEl.value, 10) : null;

    const btn = el.btnAddProduct;
    if (!btn) return;
    btn.classList.add('loading');
    btn.disabled = true;

    proviant.createProduct(barcode, expireAt, amount, storageLocationId).then((response) => {
        btn.classList.remove('loading');
        btn.disabled = false;
        switch (response.code) {
            case 201:
                showAlert(true, `Product with barcode '${barcode}' was created successfully`);
                break;
            case 400:
                showAlert(false, 'Request contained invalid data');
                break;
            case 500:
                showAlert(false, 'Backend server error');
                break;
            default:
                showAlert(false, `Undefined error: ${response.message}`);
                break;
        }
    }).catch((error) => {
        btn.classList.remove('loading');
        btn.disabled = false;
        showAlert(false, error);
    });
}
function handleBtnShowProduct() {
    const dropdown = el.instanceDropdown;
    const productId = dropdown[dropdown.selectedIndex].value;
    globalThis.location = `/web/products/${productId}/view`;
}
function handleBtnDeleteProductModal() {
    const dropdown = el.instanceDropdown;
    const productId = dropdown[dropdown.selectedIndex].value;
    const productName = getSelectedProductName();
    proviant.showConfirm(
        'Mark as wasted',
        `Mark "${productName}" as wasted? This cannot be undone.`,
        function () {
            proviant.deleteProduct(productId, false).then((response) => {
                if (response.code == 200) {
                    globalThis.location.reload();
                } else {
                    proviant.showFeedback('error', 'Wasted Failed', `Error marking product as wasted: ${response.message}`);
                }
            });
        },
        'Wasted',
        'danger'
    );
}

function handleBtnArchiveProductModal() {
    const dropdown = el.instanceDropdown;
    const productId = dropdown[dropdown.selectedIndex].value;
    const productName = getSelectedProductName();
    proviant.showConfirm(
        'Mark as consumed',
        `Mark "${productName}" as consumed?`,
        function () {
            proviant.deleteProduct(productId, true).then((response) => {
                if (response.code == 200) {
                    globalThis.location.reload();
                } else {
                    proviant.showFeedback('error', 'Consume Failed', `Error marking product as consumed: ${response.message}`);
                }
            });
        },
        'Consumed',
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
    if (target.classList.contains('needs-validation')) {
        if (target.checkValidity() === false) {
            event.preventDefault();
            event.stopPropagation();
        } else {
            event.preventDefault();
            handleBtnAddProduct();
        }
        target.classList.add('was-validated');
    }
});

/* Event delegation for all clicks */
document.addEventListener('click', function (event) {
    const target = event.target;

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

    if (target.closest('#btnAmountDec')) {
        event.preventDefault();
        const hiddenInput = el.amount;
        const display = el.amountDisplay;
        if (hiddenInput && display) {
            const newVal = Math.max(1, parseInt(hiddenInput.value, 10) - 1);
            hiddenInput.value = newVal;
            display.textContent = newVal;
        }
        return;
    }

    if (target.closest('#btnAmountInc')) {
        event.preventDefault();
        const hiddenInput = el.amount;
        const display = el.amountDisplay;
        if (hiddenInput && display) {
            const newVal = parseInt(hiddenInput.value, 10) + 1;
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

    if (target.closest('#btnBatchAmountDec')) {
        event.preventDefault();
        const hiddenInput = el.batchAmount;
        const display = el.batchAmountDisplay;
        if (hiddenInput && display) {
            const newVal = Math.max(1, parseInt(hiddenInput.value, 10) - 1);
            hiddenInput.value = newVal;
            display.textContent = newVal;
        }
        return;
    }

    if (target.closest('#btnBatchAmountInc')) {
        event.preventDefault();
        const hiddenInput = el.batchAmount;
        const display = el.batchAmountDisplay;
        if (hiddenInput && display) {
            const newVal = parseInt(hiddenInput.value, 10) + 1;
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
        handleBatchDone();
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

/* Restore a persisted batch queue after navigation */
document.addEventListener('DOMContentLoaded', function () {
    restoreBatchQueue();
});

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
