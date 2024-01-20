const expiro = {};

// Define a public method
expiro.debug = function () {
    console.log('Expiro loaded');
};

expiro.createProduct = async function (barcode, expireAt) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/products`
    let data = JSON.stringify({ barcode, expireAt })
    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: data
    })
    return response;
}

expiro.deleteProduct = async function (productID) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/products/${productID}`
    const response = await fetch(url, {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json'
        }
    })
    return response;
}

// Export the namespace
module.exports = expiro;