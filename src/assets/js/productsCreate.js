// Get elements
let queryInfoButton = document.getElementById('btnQueryProductInfo');
let createProductButton = document.getElementById('btnCreateProduct');
let inputBarcode = document.getElementById('barcode');
let inputExpireAt = document.getElementById('expireAt');
let productInfoShown = false;

// Async functions
async function queryProductInfoRequest(barcode) {
    let openFoodFactsAPIRURL = `https://world.openfoodfacts.org/api/v2/product/${barcode}?fields=product_name,countries,generic_name,image_url`;
    const response = await fetch(openFoodFactsAPIRURL, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json'
        }
    }).catch(() => {
        document.getElementById('alertQueryProductInfo').style = '';
        document.getElementById('alertQueryProductInfo').innerText = `Could not find product with barcode ${inputBarcode.value}!`
    });
    return response.json();
}
// Function handlers
function clearProductInfo() {
    document.getElementById('productData').style = 'display: none';
    document.getElementById('productInfoImage').src = '';
    document.getElementById('productInfoName').innerText = '';
    document.getElementById('productInfoGenericName').innerText = '';
    productInfoShown = false;
}
function queryProductInfo() {
    clearProductInfo();
    if (!inputBarcode.checkValidity()) {
        return;
    }
    try {
        queryProductInfoRequest(inputBarcode.value).then((response) => {
            // TODO: Catch error and trigger :invalid on barcode
            if (response) {
                let product = response.product;
                document.getElementById('productInfoImage').src = product.image_url;
                document.getElementById('productInfoName').innerText = product.product_name;
                document.getElementById('productInfoGenericName').innerText = product.generic_name;
                document.getElementById('productData').style = '';
                productInfoShown = true;
            }
        });
    } catch (error) {
        console.error("Error: " + error);
        clearProductInfo();
    }
}
function createProduct() {                            
    if (!(inputBarcode.checkValidity() && inputExpireAt.checkValidity())) {
        return;
    }
    try {
        let barcode = inputBarcode.value;
        let expireAt = inputExpireAt.valueAsDate.toISOString();
        expiro.createProduct(barcode, expireAt).then((response) => {
            // Show alert
            let alert = document.getElementById('alertCreateProductInfo')
            alert.style = ''
            switch (response.status) {
                case 201:
                    alert.classList.remove(...alert.classList);
                    alert.classList.add("alert", "alert-success");
                    alert.innerText = `Product with barcode '${barcode}' was created successfully`;
                    break;
                case 400:
                    alert.classList.remove(...alert.classList);
                    alert.classList.add("alert", "alert-warning");
                    alert.innerText = 'Request contained invalid data';
                    break;
                case 500:
                    alert.classList.remove(...alert.classList);
                    alert.classList.add("alert", "alert-danger");
                    alert.innerText = 'Backend server error';
                    break;
                default:
                    alert.classList.remove(...alert.classList);
                    alert.classList.add("alert", "alert-danger");
                    alert.innerText = `Undefined error: ${response}`;
                    break;
            }
        }).catch(error => {
            console.error("Error: " + error);
        });
    } catch (error) {
        console.error("Error: " + error);
    }
}

// Add event listeners
window.addEventListener('load', function () {
    // Fetch all the forms we want to apply custom Bootstrap validation styles to
    var forms = document.getElementsByClassName('needs-validation');
    // Loop over them and prevent submission
    var validation = Array.prototype.filter.call(forms, function (form) {
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
window.addEventListener('load', () => {
    document.getElementById('productData').style = 'display: none';
    document.getElementById('alertQueryProductInfo').style = 'display: none';
});
inputBarcode.onchange = function () {
    if (productInfoShown) {
        clearProductInfo();
    } else {
        document.getElementById('alertQueryProductInfo').style = 'display: none';
    }
};
queryInfoButton.onclick = function (event) {
    event.preventDefault();
    queryProductInfo();
};
queryInfoButton.onsubmit = function (event) {
    event.preventDefault();
    queryProductInfo();
};