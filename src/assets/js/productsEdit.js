// Get elemens
let editProductForm = document.getElementById('editProductForm');
let inputProductName = document.getElementById('inputProductName');
let inputImageURL = document.getElementById('inputImageURL');
let inputCategories = document.getElementById('inputCategories');
let inputCountries = document.getElementById('inputCountries');
let inputExpireAt = document.getElementById('inputExpireAt');
let labelProductID = document.getElementById('labelProductID');
let imgProduct = document.getElementById('imgProduct');
let alertEditProduct = document.getElementById('alertEditProduct');

function checkFormValidity() {
    return false;
}

function editProduct() {
    // Return if form is invalid
    if (!(checkFormValidity)) {
        return;
    }
    // Get form values
    let productID = parseInt(labelProductID.innerText);
    let product = {
        "ID": productID,
        "productName": inputProductName.value.trim(),
        "categories": inputCategories.value.trim(),
        "countries": inputCountries.value.trim(),
        "imageUrl": inputImageURL.value.trim(),
        "expireAt": inputExpireAt.valueAsDate.toISOString()
    };
    // Edit product
    expiro.editProduct(product).then((response) => {
        // Get and show alert
        let alert = document.getElementById('alertEditProduct');
        alert.style.display = '';
        // Switch on response code
        switch (response.code) {
            case 200:
                alert.classList.remove(...alert.classList);
                alert.classList.add("alert", "alert-success");
                alert.innerText = `Product with id '${productID}' was updated successfully`;
                break;
            case 400:
                alert.classList.remove(...alert.classList);
                alert.classList.add("alert", "alert-warning");
                alert.innerText = 'Request contained invalid data';
                break;
            case 500:
                alert.classList.remove(...alert.classList);
                alert.classList.add("alert", "alert-danger");
                alert.innerText = `Backend server error: ${response.message}`;
                break;
            default:
                alert.classList.remove(...alert.classList);
                alert.classList.add("alert", "alert-danger");
                alert.innerText = `Undefined error: ${response.message}`;
                break;
        }
    });
};

// Validity checker
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
                editProduct();
                event.preventDefault();
            }
            form.classList.add('was-validated');
        }, false);
    });
}, false);

inputImageURL.onkeyup = function () {
    imgProduct.src = inputImageURL.value;
};