function getFormElements() {
    return {
        form:              document.getElementById('editProductForm'),
        inputProductName:  document.getElementById('inputProductName'),
        inputImageURL:     document.getElementById('inputImageURL'),
        inputCategories:   document.getElementById('categoriesHidden'),
        inputCountries:    document.getElementById('countriesHidden'),
        inputExpireAt:     document.getElementById('inputExpireAt'),
        inputQuantity:     document.getElementById('inputQuantity'),
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

    const inputPriceOverride = document.getElementById('inputPriceOverride');
    const priceOverride = inputPriceOverride && inputPriceOverride.value !== ''
        ? Number.parseFloat(inputPriceOverride.value) : null;

    const inputOpenedAt = document.getElementById('inputOpenedAt');
    const openedAtValue = inputOpenedAt && inputOpenedAt.value ? inputOpenedAt.value : null;
    const openedAtDate = openedAtValue ? new Date(openedAtValue + 'T00:00:00') : null;
    const openedAt = openedAtDate && !Number.isNaN(openedAtDate.getTime()) ? openedAtDate.toISOString() : null;

    const inputDaysAfterOpening = document.getElementById('inputDaysAfterOpening');
    const daysAfterOpening = inputDaysAfterOpening && inputDaysAfterOpening.value !== ''
        ? Number.parseInt(inputDaysAfterOpening.value, 10) : null;

    const product = {
        "ID": productID,
        "productName": els.inputProductName.value.trim(),
        "categories": els.inputCategories.value.trim(),
        "countries": els.inputCountries.value.trim(),
        "imageUrl": els.inputImageURL.value.trim(),
        "expireAt": expireDate ? expireDate.toISOString() : null,
        "quantity": els.inputQuantity ? Number.parseInt(els.inputQuantity.value) || 1 : 1,
        "storageLocationId": storageLocationId,
        "notificationLeadDays": notificationLeadDays,
        "minStockAmount": minStockAmount,
        "isPrivate": isPrivate,
        "priceOverride": priceOverride,
        "openedAt": openedAt,
        "daysAfterOpening": daysAfterOpening,
    };

    proviant.editProduct(product).then((response) => {
        switch (response.code) {
            case 200:
                proviant.showFeedback('success', proviant.t("js.products.productUpdated"), proviant.t("js.products.productUpdatedMsg", productID));
                break;
            case 400:
                proviant.showFeedback('error', proviant.t("js.products.invalidDataTitle"), proviant.t("js.products.invalidRequestData"));
                break;
            case 500:
                proviant.showFeedback('error', proviant.t("js.products.serverErrorTitle"), proviant.t("js.products.backendServerErrorMsg", response.message));
                break;
            default:
                proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.products.undefinedError", response.message));
                break;
        }
    }).catch((error) => {
        proviant.showFeedback('error', proviant.t("common.error"), error?.message || String(error));
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
            proviant.showFeedback('error', proviant.t("js.products.deleteFailed"), proviant.t("js.products.deleteFailedMsg", response.message));
        }
    }).catch((error) => {
        proviant.showFeedback('error', proviant.t("common.error"), error?.message || String(error));
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
            proviant.showFeedback('error', proviant.t("js.products.restoreProductFailed"), proviant.t("js.products.restoreProductFailedMsg", response.message));
        }
    }).catch((error) => {
        proviant.showFeedback('error', proviant.t("common.error"), error?.message || String(error));
    });
}

/* Clear opened date button */
document.addEventListener("click", function (event) {
    const btn = event.target.closest("#btnClearOpenedAt");
    if (!btn) return;
    event.preventDefault();
    const input = document.getElementById('inputOpenedAt');
    if (input) input.value = '';
    const daysInput = document.getElementById('inputDaysAfterOpening');
    if (daysInput) daysInput.value = '';
});

/* Form submission */
document.addEventListener("submit", function (event) {
    const target = event.target;
    if (target.id === "editProductForm" || target.matches("form[data-bs-validate]")) {
        if (target.checkValidity() === false) {
            event.preventDefault();
            event.stopPropagation();
        } else {
            event.preventDefault();
            editProduct();
        }
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
            proviant.showFeedback('error', proviant.t("js.products.consumeFailed"), proviant.t("js.products.couldNotMarkConsumed", response.message));
        }
    }).catch((error) => {
        proviant.showFeedback('error', proviant.t("common.error"), error?.message || String(error));
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

    // Add to shopping list — used on the product detail and edit pages.
    // The button is event-delegated so the listener survives DOM updates.
    var addToListBtn = event.target.closest('.btn-add-to-shopping-list');
    if (addToListBtn) {
        // Edit page: the button is inside an <a> wrapper that would
        // otherwise navigate. The detail page has the button on its own
        // and uses the same delegation.
        if (event.target.closest('a')) return;
        event.preventDefault();
        handleAddToShoppingList(addToListBtn);
        return;
    }

    // Set expiry date to today
    if (event.target.closest('#btnTodayExpiry')) {
        var expireInput = document.getElementById('inputExpireAt');
        if (expireInput) {
            expireInput.value = new Date().toISOString().slice(0, 10);
            expireInput.dispatchEvent(new Event('blur'));
        }
    }

    // Quantity stepper buttons
    var stepBtn = event.target.closest('[data-quantity-step]');
    if (stepBtn) {
        var quantityInput = document.getElementById('inputQuantity');
        if (quantityInput) {
            var step = Number.parseInt(stepBtn.dataset.quantityStep, 10);
            quantityInput.value = Math.max(1, Number.parseInt(quantityInput.value, 10) + step);
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

/* Add-to-shopping-list flow. The button lives on the product edit page
   (productsEdit.tmpl), which renders a button with class
   .btn-add-to-shopping-list and a data-product-id attribute. The modal
   markup (#restockSuggestionModal) is provided by the host template. */
function handleAddToShoppingList(btn) {
    var productId = btn.dataset.productId;
    if (!productId) return;

    void proviant.runWithButtonBusyState(
        btn,
        async function () {
            var response = await proviant.getRestockSuggestion(productId);
            if (response.code !== 200) {
                // Fall back to a direct add with default quantity (also
                // covers the "no suggestion available" case).
                await proviant.addToShoppingList(productId, 1);
                return;
            }
            var suggestion = response.message;
            if (suggestion && suggestion.hasSuggestion && suggestion.suggestedQty > 0) {
                var choice = await openRestockModal(suggestion);
                if (choice === proviant.CANCEL) return proviant.CANCEL;
                await proviant.addToShoppingList(productId, choice.qty);
            } else {
                await proviant.addToShoppingList(productId, 1);
            }
        },
        proviant.t("js.products.addedToShoppingList"),
        function (err) {
            proviant.showFeedback('error', proviant.t("js.products.failedToAdd"), err.message || proviant.t("js.products.addToShoppingListFailed"));
        }
    );
}

// Returns a Promise that resolves with {qty} on confirm, or with
// proviant.CANCEL when the user dismisses the modal (Cancel, X, backdrop,
// Escape). runWithButtonBusyState treats CANCEL as a no-op.
function openRestockModal(suggestion) {
    return new Promise(function (resolve) {
        var modalEl = document.getElementById('restockSuggestionModal');
        if (!modalEl || typeof bootstrap === 'undefined') {
            resolve({ qty: suggestion.suggestedQty });
            return;
        }

        var qtyEl = document.getElementById('restockQty');
        var reasonEl = document.getElementById('restockReason');
        var productEl = document.getElementById('restockProductName');
        var confirmBtn = document.getElementById('restockConfirmBtn');

        if (qtyEl) qtyEl.value = suggestion.suggestedQty;
        if (productEl) {
            productEl.textContent = suggestion.productName || suggestion.display || proviant.t("js.products.suggestedQuantity");
        }
        if (reasonEl) {
            // Single source of truth: the server pre-formats this in
            // formatRateTemplateMessage / formatMinStockTemplateMessage.
            reasonEl.textContent = buildModalReason(suggestion);
        }

        var resolved = false;
        var onHidden = function () { finish(proviant.CANCEL); };
        var finish = function (result) {
            if (resolved) return;
            resolved = true;
            modalEl.removeEventListener('hidden.bs.dialog', onHidden);
            resolve(result);
        };

        modalEl.addEventListener('hidden.bs.dialog', onHidden);

        if (confirmBtn) {
            var newBtn = confirmBtn.cloneNode(true);
            confirmBtn.parentNode.replaceChild(newBtn, confirmBtn);
            newBtn.addEventListener('click', function () {
                var qty = qtyEl ? Math.max(1, Number.parseInt(qtyEl.value, 10) || 1) : 1;
                var modal = bootstrap.Dialog.getInstance(modalEl);
                if (modal) modal.hide();
                finish({ qty: qty });
            });
        }

        bootstrap.Dialog.getOrCreateInstance(modalEl).show();
    });
}

function buildModalReason(suggestion) {
    if (suggestion.source === 'consumption_rate' && suggestion.perWeekDisplay) {
        return proviant.t("js.products.basedOnUsage", suggestion.perWeekDisplay);
    }
    if (suggestion.source === 'min_stock') {
        return proviant.t("js.products.basedOnMinStock");
    }
    return proviant.t("js.products.suggestedQuantityReason");
}
