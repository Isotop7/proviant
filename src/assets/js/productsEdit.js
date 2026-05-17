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
        inputNotificationLeadDays: document.getElementById('inputNotificationLeadDays'),
        inputMinStockAmount:      document.getElementById('inputMinStockAmount'),
        inputIsPrivate:    document.getElementById('inputIsPrivate'),
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

    const inputNotificationLeadDays = document.getElementById('inputNotificationLeadDays');
    const notificationLeadDays = inputNotificationLeadDays && inputNotificationLeadDays.value
        ? Number.parseInt(inputNotificationLeadDays.value) : null;

    const inputMinStockAmount = document.getElementById('inputMinStockAmount');
    const minStockAmount = inputMinStockAmount && inputMinStockAmount.value
        ? Number.parseInt(inputMinStockAmount.value) || 0 : 0;

    const inputIsPrivate = document.getElementById('inputIsPrivate');
    const isPrivate = inputIsPrivate && inputIsPrivate.checked;

    const product = {
        "ID": productID,
        "productName": els.inputProductName.value.trim(),
        "categories": els.inputCategories.value.trim(),
        "countries": els.inputCountries.value.trim(),
        "imageUrl": els.inputImageURL.value.trim(),
        "expireAt": expireDate ? expireDate.toISOString() : null,
        "amount": els.inputAmount ? Number.parseInt(els.inputAmount.value) || 0 : 0,
        "storageLocationId": storageLocationId,
        "notificationLeadDays": notificationLeadDays,
        "minStockAmount": minStockAmount,
        "isPrivate": isPrivate,
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
            window.location.href = '/web/products';
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
            window.location.href = '/web/products';
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

function consumeProduct() {
    const labelProductID = document.getElementById('labelProductID');
    const productID = Number.parseInt(labelProductID?.innerText?.trim());
    if (!Number.isInteger(productID) || productID <= 0) return;

    proviant.deleteProduct(productID, true).then((response) => {
        if (response.code === 200) {
            window.location.href = '/web/products';
        } else {
            proviant.showFeedback('error', 'Consume Failed', `Could not mark product as consumed: ${response.message}`);
        }
    });
}

/* Delete confirmation button in modal */
document.addEventListener("click", function (event) {
    if (event.target.closest('#btnConfirmDelete')) {
        deleteProduct();
    }
    if (event.target.closest('#btnRestoreProduct') || event.target.closest('#btnRestoreProductMobile')) {
        restoreProduct();
    }
    if (event.target.closest('#btnConsumeProduct') || event.target.closest('#btnConsumeProductMobile')) {
        consumeProduct();
    }

    // Set expiry date to today
    if (event.target.closest('#btnTodayExpiry')) {
        var expireInput = document.getElementById('inputExpireAt');
        if (expireInput) {
            expireInput.value = new Date().toISOString().slice(0, 10);
            expireInput.dispatchEvent(new Event('blur'));
        }
    }

    // Amount stepper buttons
    var stepBtn = event.target.closest('[data-amount-step]');
    if (stepBtn) {
        var amountInput = document.getElementById('inputAmount');
        if (amountInput) {
            var step = parseInt(stepBtn.dataset.amountStep, 10);
            amountInput.value = Math.max(0, parseInt(amountInput.value, 10) + step);
        }
    }

    // Focus image URL input
    if (event.target.closest('#btnMobilePhotoFocus') || event.target.closest('#btnDesktopChangeImage')) {
        var imageInput = document.getElementById('inputImageURL');
        if (imageInput) imageInput.focus();
    }
});

/* Blur validation for required fields */
document.addEventListener("DOMContentLoaded", function () {
    var inputProductName = document.getElementById("inputProductName");
    if (inputProductName) {
        inputProductName.addEventListener("blur", function () {
            var valid = this.value.trim() !== "";
            this.classList.toggle("is-invalid", !valid);
            this.classList.toggle("is-valid", valid);
        });
    }

    var inputExpireAt = document.getElementById("inputExpireAt");
    if (inputExpireAt) {
        inputExpireAt.addEventListener("blur", function () {
            var valid = this.value !== "";
            this.classList.toggle("is-invalid", !valid);
            this.classList.toggle("is-valid", valid);
        });
    }

    // Set up image fallback handling
    var imgProduct = document.getElementById('imgProduct');
    if (imgProduct) {
        imgProduct.addEventListener('load', function () {
            this.style.opacity = '1';
            var fb = document.getElementById('imgProductFallback');
            if (fb) fb.style.display = 'none';
        });
        imgProduct.addEventListener('error', function () {
            var fb = document.getElementById('imgProductFallback');
            if (fb) fb.style.display = 'flex';
        });
    }

    var mobileThumb = document.querySelector('.pv-mobile-thumb img');
    if (mobileThumb) {
        mobileThumb.addEventListener('load', function () {
            this.style.opacity = '1';
            if (this.nextElementSibling) this.nextElementSibling.style.display = 'none';
        });
        mobileThumb.addEventListener('error', function () {
            if (this.nextElementSibling) this.nextElementSibling.style.display = 'flex';
        });
    }
});
