/* Helper to dynamically query DOM elements — always fresh, never stale */
function getFormElements() {
    return {
        foundBarcodeWrapper: document.getElementById('foundBarcode'),
        productDataWrapper: document.getElementById('productData'),
        inputBarcode: document.getElementById('barcode'),
        inputExpireAt: document.getElementById('expireAt'),
    };
}
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
    const els = getFormElements();
    if (els.inputBarcode) {
        els.inputBarcode.value = '';
        els.inputBarcode.style.backgroundColor = 'var(--bs-warning)';
        els.inputBarcode.style.color = 'var(--bs-warning-text)';
    }
    console.error(error);
}
function showBarcode(barcode) {
    const els = getFormElements();
    if (els.inputBarcode) {
        els.inputBarcode.value = barcode;
        els.inputBarcode.classList.add('border-success');
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
    document.getElementById('productInfoImage').src = '';
    document.getElementById('productInfoImage').alt = '';
    document.getElementById('productInfoName').innerText = '';
    document.getElementById('productInfoGenericName').innerText = '';
    const hintEl = document.getElementById('storageHint');
    if (hintEl) {
        hintEl.textContent = '';
        hintEl.classList.add('d-none');
    }
}

// Loading state helper for barcode lookup
function setBarcodeLoading(loading) {
  const el = document.getElementById('barcodeLookupLoading');
  if (el) {
    if (loading) {
      el.classList.remove('d-none');
    } else {
      el.classList.add('d-none');
    }
  }
}
function showProductData(product) {
    document.getElementById('productInfoImage').src = product.imageUrl;
    document.getElementById('productInfoImage').alt = product.productName || 'Product image';
    document.getElementById('productInfoName').innerText = product.productName;
    if (product.categories != 'undefined' && product.categories != null) {
        document.getElementById('productInfoGenericName').innerText = product.categories;
    }
    const hintEl = document.getElementById('storageHint');
    if (hintEl) {
        if (product.storageHint) {
            hintEl.textContent = product.storageHint;
            hintEl.classList.remove('d-none');
        } else {
            hintEl.classList.add('d-none');
        }
    }
    document.getElementById('productData').classList.remove('d-none');
}

// UI toggle functions
function toggleBtnScan(state) {
    const btnScan = document.getElementById('btnScan');
    if (state) {
        btnScan.dataset.action = 'scan';
        btnScan.innerHTML = '<i class="bi bi-upc-scan px-2"></i>Scan';
    } else {
        btnScan.dataset.action = 'stop';
        btnScan.innerHTML = '<i class="bi bi-stop-circle px-2"></i>Stop';
    }
}
function toggleBtnAddProduct(state) {
    if (state) {
        document.getElementById('btnAddProduct').disabled = false;
    } else {
        document.getElementById('btnAddProduct').disabled = true;
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
    document.getElementById('barcode-reader-wrapper').style.display = 'none';
    if (html5QrCode.getState() == Html5QrcodeScannerState.SCANNING) {
        html5QrCode.stop();
        html5QrCode.clear();
    }
};
const formatProductDataAsOption = (product) => `${product.productName}; Created: ${proviant.formatDate(product.CreatedAt)}; Expire at: ${proviant.formatDate(product.expireAt)}; Notified at: ${proviant.formatDate(product.notifiedAt)}`;
function setProductOptionsState(productState, products) {
    // Get all elements
    const btnAdd = document.getElementById('btnAddProduct');
    const btnShow = document.getElementById('btnShowProduct')
    const btnDeleteModal = document.getElementById('btnDeleteProductModal');
    const btnArchiveModal = document.getElementById('btnArchiveProductModal');
    const btnShowAll = document.getElementById('btnShowProducts');
    const instanceDropdown = document.getElementById('instanceDropdown');

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
    // Always read the element fresh — avoids stale reference after DOM updates
    const el = document.getElementById('expireAt');
    const base = el.value ? parseInputDate(el.value) : new Date();
    if (days !== 0) base.setDate(base.getDate() + days);
    if (months !== 0) base.setMonth(base.getMonth() + months);
    el.value = formatInputDate(base);
    // Flash the field so the user sees the value changed
    el.classList.remove('date-updated');
    void el.offsetWidth; // force reflow to restart the animation if clicked repeatedly
    el.classList.add('date-updated');
    el.addEventListener('animationend', () => el.classList.remove('date-updated'), { once: true });
}

// Async functions
async function queryProductInfoRequest(barcode) {
    return proviant.getOpenFoodFactsData(barcode).catch(() => {
        const els = getFormElements();
        const bc = els.inputBarcode ? els.inputBarcode.value : 'unknown';
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
    document.getElementById('barcode').dataset.barcode = barcode;
    document.getElementById('barcode').value = barcode;
    document.getElementById('productData').classList.remove('d-none');
}
function getSelectedProductName() {
    const instanceDropdown = document.getElementById('instanceDropdown');
    if (!instanceDropdown || instanceDropdown.selectedIndex < 0) return '';
    return instanceDropdown[instanceDropdown.selectedIndex].innerText.split(';')[0].trim();
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
    const btnScan = document.getElementById('btnScan');
    if (btnScan.dataset.action == 'scan') {
        toggleGrowers(true);
        toggleBtnScan(false);
        document.getElementById('barcode-reader-wrapper').style.display = '';
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
    const els = getFormElements();
    if (!els.inputBarcode || !els.inputExpireAt) return;
    if (!(els.inputBarcode.checkValidity() && els.inputExpireAt.checkValidity())) {
        return;
    }

    const barcode = document.getElementById('barcode').value;
    if (barcode === '') {
        showAlert(false, 'Barcode cannot be empty');
        return;
    }
    const expireAt = document.getElementById('expireAt').valueAsDate.toISOString();
    const amountEl = document.getElementById('amount');
    const amount = amountEl ? parseInt(amountEl.value, 10) || 1 : 1;
    const locationEl = document.getElementById('modalStorageLocation');
    const storageLocationId = locationEl && locationEl.value ? parseInt(locationEl.value, 10) : null;

    const btn = document.getElementById('btnAddProduct');
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
    const instanceDropdown = document.getElementById('instanceDropdown');
    const productId = instanceDropdown[instanceDropdown.selectedIndex].value;
    globalThis.location = `/web/products/${productId}/view`;
}
function handleBtnDeleteProductModal() {
    const instanceDropdown = document.getElementById('instanceDropdown');
    const productId = instanceDropdown[instanceDropdown.selectedIndex].value;
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
    const instanceDropdown = document.getElementById('instanceDropdown');
    const productId = instanceDropdown[instanceDropdown.selectedIndex].value;
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
    const barcode = document.getElementById('barcode').value;
    globalThis.location = `/web/products?queryParam=barcode&queryValue=${barcode}`;
}

// Input handlers
function handleChangedBarcode() {
    const els = getFormElements();
    if (!els.inputBarcode) return;
    if (!(els.inputBarcode.checkValidity())) {
        if (els.inputBarcode.classList.contains('border-success')) {
            els.inputBarcode.classList.remove('border-success')
        }
        return;
    }
    const barcode = document.getElementById('barcode').value;
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
        const hiddenInput = document.getElementById('amount');
        const display = document.getElementById('amountDisplay');
        if (hiddenInput && display) {
            const newVal = Math.max(1, parseInt(hiddenInput.value, 10) - 1);
            hiddenInput.value = newVal;
            display.textContent = newVal;
        }
        return;
    }

    if (target.closest('#btnAmountInc')) {
        event.preventDefault();
        const hiddenInput = document.getElementById('amount');
        const display = document.getElementById('amountDisplay');
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
});

/* Event delegation for barcode input */
document.addEventListener('input', function (event) {
    const target = event.target;
    if (target.id === 'barcode' || target.name === 'barcode') {
        handleChangedBarcode();
    }
});
