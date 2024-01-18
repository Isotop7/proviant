// Get elements
let foundBarcodeWrapper = document.getElementById("foundBarcode");
let productDataWrapper = document.getElementById("productData");

// UI functions
function showError(error) {
    document.getElementById('scanResult').classList = '';
    document.getElementById('scanResult').classList.add('mx-4', 'my-1', 'alert', 'alert-danger', 'font-monospace');
    document.getElementById('scanResult').innerText = error;
    document.getElementById('scanResult').style.display = '';
}
function showBarcode(barcode) {
    document.getElementById('scanResult').classList = '';
    document.getElementById('scanResult').classList.add('mx-4', 'my-1', 'alert', 'alert-success');
    document.getElementById('scanResult').innerHTML = `Found barcode: <span class="font-monospace" id="foundBarcode">${barcode}</span>`
    document.getElementById('scanResult').style.display = '';
}
function hideScanResult() {
    document.getElementById('scanResult').style.display = 'none'
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
async function scanProductRequest() {
    let uploadElement = document.getElementById('upload');
    if (uploadElement == null) {
        throw new Error('No image selected');
    }
    let fileList = uploadElement.files;
    if (fileList.length == 0) {
        throw new Error('No image found');
    }
    let file = fileList[0];
    const formData = new FormData();
    formData.append('image', file);
    try {
        let url = `${window.location.protocol}//${window.location.host}/api/v1/products/scan`
        const response = await fetch(url, {
            method: 'POST',
            body: formData
        });

        if (response.ok) {
            return response.json();
        } else {
            throw new Error('Scanning failed');
        }
    } catch (error) {
        throw error;
    }
}
// Function handlers
function queryProductInfo(barcode) {
    clearProductInfo();
    try {
        queryProductInfoRequest(barcode).then((response) => {
            let product = response.product;
            console.error(product);
            showProductData(product);
        }).catch((error) => {
            showError("Error: " + error);
            clearProductInfo();
        });
    } catch (error) {
        showError("Error: " + error);
        clearProductInfo();
    }
}
function handleScanButton() {
    scanProductRequest().then((response) => {
        let barcode = response.barcode;
        if (barcode == null) {
            showError('No barcode found');
        }
        queryProductInfo(barcode);
        showBarcode(barcode)
    }).catch((error) => {
        showError(error);
    });
}


// Add event listeners
window.addEventListener('load', hideScanResult);
document.getElementById('upload').onchange = function() {
    clearProductInfo();
    hideScanResult();
};
document.getElementById('btnScan').onclick = function (event) {
    event.preventDefault();
    handleScanButton();
};
document.getElementById('btnScan').onsubmit = function (event) {
    event.preventDefault();
    handleScanButton();
};