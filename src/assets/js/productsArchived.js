async function restoreProduct(productID) {
    await proviant.restoreProduct(productID).then((response) => {
        switch (response.code) {
            case 200:
                console.log("Product restored")
            default:
                console.error(response.message)
        }
    });
}

document.querySelectorAll('.btn-product-restore').forEach(button => {
    button.addEventListener('click', async function () {
        const productId = this.getAttribute('data-id');
        await restoreProduct(productId);
        location.reload();
    });
});
