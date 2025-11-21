// Get elements
let foundBarcodeWrapper = document.getElementById('foundBarcode');
let productDataWrapper = document.getElementById('productData');
let inputBarcode = document.getElementById('barcode');
let inputExpireAt = document.getElementById('expireAt');
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
    inputBarcode.value = '';
    inputBarcode.style.backgroundColor = 'var(--bs-warning)';
    inputBarcode.style.color = 'var(--bs-warning-text)';
    console.error(error);
}
function showBarcode(barcode) {
    inputBarcode.value = barcode;
    inputBarcode.classList.add('border-success');
}
function showAlert(isSuccess, message) {
    const alertElement = document.getElementById('productAlert');
    const alertMessage = document.getElementById('alertMessage');

    if (isSuccess) {
        alertElement.classList.remove('alert-danger');
        alertElement.classList.add('alert-success');
        alertMessage.textContent = message || "Product created successfully!";
    } else {
        alertElement.classList.remove('alert-success');
        alertElement.classList.add('alert-danger');
        alertMessage.textContent = message || "Failed to create product. Please try again.";
    }

    alertElement.classList.add('show');
    setTimeout(() => dismissAlert(), 3000);
}
function dismissAlert() {
    const alertElement = document.getElementById('productAlert');
    alertElement.classList.remove('show');
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
    if (product.generic_name != 'undefined' && product.generic_name != null) {
        document.getElementById('productInfoGenericName').innerText = product.generic_name;
    }
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
function setDeleteModalBody () {
    const instanceDropdown = document.getElementById('instanceDropdown');
    const productData = instanceDropdown[instanceDropdown.selectedIndex].innerText;
    const deleteButtonModalBody = document.getElementById('deleteModalBody');
    deleteButtonModalBody.innerHTML = `Do you want to delete the following product:</br></br>${productData.replaceAll(';','</br>')}`;
}
function setArchiveModalBody () {
    const instanceDropdown = document.getElementById('instanceDropdown');
    const productData = instanceDropdown[instanceDropdown.selectedIndex].innerText;
    const archiveButtonModalBody = document.getElementById('archiveModalBody');
    archiveButtonModalBody.innerHTML = `Do you want to archive the following product:</br></br>${productData.replaceAll(';','</br>')}`;
}
function checkBarcode(barcode) {
    try {
        proviant.getProductsByBarcode(barcode).then((response) => {
            switch (response.code) {
                case 200:
                    const products = response.message;
                    if (products.length > 0) {
                        setProductOptionsState(ProductState.PRESENT, products);
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
        })
    } catch (error) {
        console.error(error);
    }
};

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
    if (!(inputBarcode.checkValidity() && inputExpireAt.checkValidity())) {
        return;
    }

    dismissAlert();

    try {
        const barcode = document.getElementById('barcode').value;
        if (barcode === '') {
            throw new Error('Barcode cannot be empty');
        }
        const expireAt = document.getElementById('expireAt').valueAsDate.toISOString();
        proviant.createProduct(barcode, expireAt).then((response) => {
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
        }).catch(error => {
            showAlert(false, error);
        });
    } catch (error) {
        showAlert(false, error);
    }
};
function handleBtnShowProduct() {
    const instanceDropdown = document.getElementById('instanceDropdown');
    const productId = instanceDropdown[instanceDropdown.selectedIndex].value;
    window.location = `/web/products/${productId}/view`;
}
function handleBtnDeleteProduct() {
    const instanceDropdown = document.getElementById('instanceDropdown');
    const productId = instanceDropdown[instanceDropdown.selectedIndex].value;

    proviant.deleteProduct(productId, false).then((response) => {
        console.log(response.code);
        switch (response.code) {
            case 200:
                window.location.reload();
                break;
            default:
                showAlert(false, `Error deleting product with ID ${productId}: ${response.message}`);
                break;
        }
    });
}
function handleBtnArchiveProduct() {
    const instanceDropdown = document.getElementById('instanceDropdown');
    const productId = instanceDropdown[instanceDropdown.selectedIndex].value;

    proviant.deleteProduct(productId, true).then((response) => {
        console.log(response.code);
        switch (response.code) {
            case 200:
                window.location.reload();
                break;
            default:
                showAlert(false, `Error archiving product with ID ${productId}: ${response.message}`);
                break;
        }
    });
}
function handleBtnShowProducts() {
    const barcode = document.getElementById('barcode').value;
    window.location = `/web/products/search?barcode=${barcode}`;
}

// Input handlers
function handleChangedBarcode() {
    if (!(inputBarcode.checkValidity())) {
        if (inputBarcode.classList.contains('border-success')) {
            inputBarcode.classList.remove('border-success')
        }
        return;
    }
    const barcode = document.getElementById('barcode').value;
    queryProductInfo(barcode);
    showBarcode(barcode)
    storeBarcode(barcode);
    clearScanUI();
    checkBarcode(barcode);
    dismissAlert();
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
document.addEventListener("DOMContentLoaded", () => {
    const alertElement = document.getElementById('productAlert');
    alertElement.addEventListener('close.bs.alert', function (event) {
        event.preventDefault();
        alertElement.classList.remove('show');
    });
});
document.getElementById('barcode').addEventListener('input', function (event) {
    event.preventDefault();
    handleChangedBarcode();
})
document.getElementById('btnAddProduct').onclick = function (event) {
    event.preventDefault();
    handleBtnAddProduct();
}
document.getElementById('btnShowProduct').onclick = function (event) {
    event.preventDefault();
    handleBtnShowProduct();
}
document.getElementById('deleteModal').addEventListener('show.bs.modal', (event) => {
    setDeleteModalBody();
});
document.getElementById('archiveModal').addEventListener('show.bs.modal', (event) => {
    setArchiveModalBody();
});
document.getElementById('btnDeleteProduct').onclick = function (event) {
    event.preventDefault();
    handleBtnDeleteProduct();
}
document.getElementById('btnArchiveProduct').onclick = function (event) {
    event.preventDefault();
    handleBtnArchiveProduct();
}
document.getElementById('btnShowProducts').onclick = function (event) {
    event.preventDefault();
    handleBtnShowProducts();
}
document.getElementById('btnScan').onclick = function (event) {
    event.preventDefault();
    handleScanButton();
};
