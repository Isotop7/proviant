let imgProduct = document.getElementById('imgProduct');

let inputImageURL = document.getElementById('inputImageURL');

inputImageURL.onkeyup = function() {
    imgProduct.src = inputImageURL.value;
};