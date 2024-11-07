// Get elements
let foundBarcodeWrapper = document.getElementById("foundBarcode");
let productDataWrapper = document.getElementById("productData");
const html5QrCode = new Html5Qrcode("barcode-reader",
    { formatsToSupport: [Html5QrcodeSupportedFormats.EAN_13] }
);

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
function toggleLoadingSpinner(state) {
    if (state) {
        document.getElementById('loadingSpinner').style.display = '';
    } else {
        document.getElementById('loadingSpinner').style.display = 'none';
    }
}
function toggleBtnLiveScan(state) {
    if (state) {
        document.getElementById('btnLiveScan').disabled = false;
    } else {
        document.getElementById('btnLiveScan').disabled = true;
    }
}
function toggleBtnStopLiveScan(state) {
    if (state) {
        document.getElementById('btnStopLiveScan').disabled = false;
    } else {
        document.getElementById('btnStopLiveScan').disabled = true;
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
            showError("Error: " + error);
            clearProductInfo();
        });
    } catch (error) {
        showError("Error: " + error);
        clearProductInfo();
    }
}
function handleLiveScanButton() {
    toggleLoadingSpinner(true);
    toggleGrowers(true);
    toggleBtnStopLiveScan(true);
    toggleBtnLiveScan(false);
    html5QrCode.start(
        { facingMode: "environment" },
        {
            fps: 10,
            qrbox: { width: 240, height: 100 }
        },
        (decodedText) => {
            queryProductInfo(decodedText);
            showBarcode(decodedText)
            toggleLoadingSpinner(false);
            toggleGrowers(false);
        }
    );
}
function handleStopLiveScanButton() {
    toggleLoadingSpinner(false);
    toggleGrowers(false);
    toggleBtnStopLiveScan(false);
    toggleBtnLiveScan(true);
    html5QrCode.stop();
    html5QrCode.clear();
}

// Add event listeners
window.addEventListener('load', function () {
    hideScanResult();
});
document.getElementById('btnLiveScan').onclick = function (event) {
    event.preventDefault();
    handleLiveScanButton();
}
document.getElementById('btnStopLiveScan').onclick = function (event) {
    event.preventDefault();
    handleStopLiveScanButton();
}