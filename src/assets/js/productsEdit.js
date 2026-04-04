/* Helper to dynamically query form elements */
function getFormElements() {
    return {
        form: document.getElementById('editProductForm'),
        inputProductName: document.getElementById('inputProductName'),
        inputImageURL: document.getElementById('inputImageURL'),
        inputCategories: document.getElementById('inputCategories'),
        inputCountries: document.getElementById('inputCountries'),
        inputExpireAt: document.getElementById('inputExpireAt'),
        inputAmount: document.getElementById('inputAmount'),
        labelProductID: document.getElementById('labelProductID'),
        imgProduct: document.getElementById('imgProduct'),
        alertEditProduct: document.getElementById('alertEditProduct'),
    };
}

function checkFormValidity() {
    // TODO: Implement actual validation logic
    return true;
}

function editProduct() {
    const els = getFormElements();
    if (!els.form || !els.labelProductID || !els.inputProductName ||
        !els.inputCategories || !els.inputCountries || !els.inputImageURL || !els.inputExpireAt) {
        return;
    }

    // Return if form is invalid
    if (!checkFormValidity()) {
        return;
    }

    // Get form values
    const productID = Number.parseInt(els.labelProductID.innerText.trim());
    const expireDate = els.inputExpireAt.valueAsDate;

    const product = {
        "ID": productID,
        "productName": els.inputProductName.value.trim(),
        "categories": els.inputCategories.value.trim(),
        "countries": els.inputCountries.value.trim(),
        "imageUrl": els.inputImageURL.value.trim(),
        "expireAt": expireDate ? expireDate.toISOString() : null,
        "amount": els.inputAmount ? Number.parseInt(els.inputAmount.value) || 0 : 0,
    };

    // Edit product
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

/* Event delegation for form submission */
document.addEventListener("submit", function (event) {
    const target = event.target;
    // Handle editProductForm or any form with needs-validation class
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

/* Event delegation for keyup on image URL input */
document.addEventListener("keyup", function (event) {
    const target = event.target;
    if (target.id === "inputImageURL") {
        const imgProduct = document.getElementById("imgProduct");
        if (imgProduct) {
            imgProduct.src = target.value;
        }
    }
});