async function deleteProduct(productID, archiveOnly) {
    await proviant.deleteProduct(productID, archiveOnly).then((response) => {
        switch (response.code) {
            case 200:
                console.log("Product deleted")
            default:
                console.error(response.message)
        }
    });
}

document.querySelectorAll('.btn-product-delete').forEach(button => {
    button.addEventListener('click', async function () {
        const productId = this.getAttribute('data-id');
        await deleteProduct(productId, false);
        location.reload();
    });
});

document.querySelectorAll('.btn-product-archive').forEach(button => {
    button.addEventListener('click', async function () {
        const productId = this.getAttribute('data-id');
        await deleteProduct(productId, true);
        location.reload();
    });
});
