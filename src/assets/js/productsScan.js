// Get elements
let foundBarcodeWrapper = document.getElementById('foundBarcode');
let productDataWrapper = document.getElementById('productData');
let inputBarcode = document.getElementById('barcode');
const html5QrCode = new Html5Qrcode('barcode-reader',
    { formatsToSupport: [Html5QrcodeSupportedFormats.EAN_13] }
);

// UI functions
function showError(error) {
    inputBarcode.value = '';
    inputBarcode.style.backgroundColor = 'var(--bs-warning)';
    console.error(error);
}
function showBarcode(barcode) {
    inputBarcode.value = barcode;
    inputBarcode.style.backgroundColor = 'var(--bs-success)';
}
function clearProductInfo() {
    document.getElementById('productInfoImage').src = '';
    document.getElementById('productInfoImage').style.height = '160px';
    document.getElementById('productInfoName').innerText = '';
    document.getElementById('productInfoGenericName').innerText = '';
}
function showProductData(product) {
    document.getElementById('productInfoImage').src = product.image_url;
    document.getElementById('productInfoName').innerText = product.product_name;
    document.getElementById('productInfoGenericName').innerText = product.generic_name;
    document.getElementById('productData').style.display = '';
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
            element.style.display = '';
        });
    } else {
        placeholders.forEach(element => {
            element.style.display = 'none';
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
    document.getElementById('barcode-reader-wrapper').classList.remove('py-4');
    if (html5QrCode.getState() == Html5QrcodeScannerState.SCANNING) {
        html5QrCode.stop();
        html5QrCode.clear();
    }
};

// Async functions
async function queryProductInfoRequest(barcode) {
    let openFoodFactsAPIRURL = `https://world.openfoodfacts.org/api/v2/product/${barcode}?fields=product_name,countries,generic_name,image_url`;
    const response = await fetch(openFoodFactsAPIRURL, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json'
        }
    }).catch(() => {
        showError(`Could not find product with barcode ${inputBarcode.value}!`);
    });
    return response.json();
}
// Function handlers
function queryProductInfo(barcode) {
    clearProductInfo();
    try {
        queryProductInfoRequest(barcode).then((response) => {
            let product = response.product;
            showProductData(product);
        }).catch((error) => {
            showError('Error: ' + error);
            clearProductInfo();
        });
    } catch (error) {
        showError('Error: ' + error);
        clearProductInfo();
    }
}
function storeBarcode(barcode) {
    document.getElementById('barcode').dataset.barcode = barcode;
    document.getElementById('barcode').value = barcode;
}

// Button handlers
function handleScanButton() {
    const btnScan = document.getElementById('btnScan');
    if (btnScan.dataset.action == 'scan') {
        toggleGrowers(true);
        toggleBtnScan(false);
        document.getElementById('barcode-reader-wrapper').classList.add('py-4');
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
        );
    } else if (btnScan.dataset.action == 'stop') {
        clearScanUI();
    } else {
        console.error('Undefined data-action ' + btnScan.dataset.action);
    }
};
// Input handlers
function handleChangedBarcode() {
    if (!(inputBarcode.checkValidity())) {
        return;
    }
    const barcode = document.getElementById('barcode').value;
    queryProductInfo(barcode);
    showBarcode(barcode)
    storeBarcode(barcode);
    clearScanUI();
};

// Add event listeners
window.addEventListener('load', function () {
    // Fetch all the forms we want to apply custom Bootstrap validation styles to
    let forms = document.getElementsByClassName('needs-validation');
    // Loop over them and prevent submission
    Array.prototype.filter.call(forms, function (form) {
        form.addEventListener('submit', function (event) {
            if (form.checkValidity() === false) {
                event.preventDefault();
                event.stopPropagation();
            } else {
                createProduct();
                event.preventDefault();
            }
            form.classList.add('was-validated');
        }, false);
    });
}, false);
document.getElementById('barcode').addEventListener('input', function (event) {
    event.preventDefault();
    handleChangedBarcode();
})
document.getElementById('btnScan').onclick = function (event) {
    event.preventDefault();
    handleScanButton();
};