function getFormElements() {
    return {
        form:              document.getElementById('editProductForm'),
        inputProductName:  document.getElementById('inputProductName'),
        inputImageURL:     document.getElementById('inputImageURL'),
        inputCategories:   document.getElementById('categoriesHidden'),
        inputCountries:    document.getElementById('countriesHidden'),
        inputExpireAt:     document.getElementById('inputExpireAt'),
        inputAmount:       document.getElementById('inputAmount'),
        labelProductID:    document.getElementById('labelProductID'),
        imgProduct:        document.getElementById('imgProduct'),
        alertEditProduct:  document.getElementById('alertEditProduct'),
    };
}

function checkFormValidity() {
    const els = getFormElements();
    const productID = Number.parseInt(els.labelProductID?.innerText?.trim());
    if (!Number.isInteger(productID) || productID <= 0) return false;
    if (!els.inputProductName?.value?.trim()) return false;
    return true;
}

function editProduct() {
    const els = getFormElements();
    if (!els.form || !els.labelProductID || !els.inputProductName ||
        !els.inputCategories || !els.inputCountries || !els.inputImageURL || !els.inputExpireAt) {
        return;
    }

    if (!checkFormValidity()) return;

    const productID = Number.parseInt(els.labelProductID.innerText.trim());
    const expireDate = els.inputExpireAt.valueAsDate;

    const selectStorageLocation = document.getElementById('selectStorageLocation');
    const storageLocationId = selectStorageLocation && selectStorageLocation.value
        ? Number.parseInt(selectStorageLocation.value) : null;

    const product = {
        "ID": productID,
        "productName": els.inputProductName.value.trim(),
        "categories": els.inputCategories.value.trim(),
        "countries": els.inputCountries.value.trim(),
        "imageUrl": els.inputImageURL.value.trim(),
        "expireAt": expireDate ? expireDate.toISOString() : null,
        "amount": els.inputAmount ? Number.parseInt(els.inputAmount.value) || 0 : 0,
        "storageLocationId": storageLocationId,
    };

    proviant.editProduct(product).then((response) => {
        switch (response.code) {
            case 200:
                proviant.showFeedback('success', 'Product Updated', `Product #${productID} was updated successfully.`);
                break;
            case 400:
                proviant.showFeedback('error', 'Invalid Data', 'Request contained invalid data.');
                break;
            case 500:
                proviant.showFeedback('error', 'Server Error', `Backend server error: ${response.message}`);
                break;
            default:
                proviant.showFeedback('error', 'Error', `Undefined error: ${response.message}`);
                break;
        }
    });
}

function deleteProduct() {
    const labelProductID = document.getElementById('labelProductID');
    const productID = Number.parseInt(labelProductID?.innerText?.trim());
    if (!Number.isInteger(productID) || productID <= 0) return;

    proviant.deleteProduct(productID, false).then((response) => {
        if (response.code === 200) {
            window.location.href = '/products';
        } else {
            proviant.showFeedback('error', 'Delete Failed', `Could not delete product: ${response.message}`);
        }
    });
}

function restoreProduct() {
    const labelProductID = document.getElementById('labelProductID');
    const productID = Number.parseInt(labelProductID?.innerText?.trim());
    if (!Number.isInteger(productID) || productID <= 0) return;

    proviant.restoreProduct(productID).then((response) => {
        if (response.code === 200) {
            window.location.href = '/products';
        } else {
            proviant.showFeedback('error', 'Restore Failed', `Could not restore product: ${response.message}`);
        }
    });
}

/* Form submission */
document.addEventListener("submit", function (event) {
    const target = event.target;
    if (target.id === "editProductForm" || target.classList.contains("needs-validation")) {
        if (target.checkValidity() === false) {
            event.preventDefault();
            event.stopPropagation();
        } else {
            event.preventDefault();
            editProduct();
        }
        target.classList.add('was-validated');
    }
});

/* Update all product image elements when URL field changes */
document.addEventListener("keyup", function (event) {
    const target = event.target;
    if (target.id === "inputImageURL") {
        document.querySelectorAll('.pv-img-product').forEach(el => {
            el.src = target.value;
        });
    }
});

/* Delete confirmation button in modal */
document.addEventListener("click", function (event) {
    if (event.target.closest('#btnConfirmDelete')) {
        deleteProduct();
    }
    if (event.target.closest('#btnRestoreProduct') || event.target.closest('#btnRestoreProductMobile')) {
        restoreProduct();
    }
});
