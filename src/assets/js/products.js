function showToast(message) {
    let infoToast = document.getElementById('infoToast');
    let infoToastBody = document.getElementById('infoToastBody');
    infoToastBody.innerText = message;
    let toastBootstrap = bootstrap.Toast.getOrCreateInstance(infoToast);
    toastBootstrap.show();
}

async function deleteProduct(productID) {
    await proviant.deleteProduct(productID).then((response) => {
        switch (response.code) {
            case 200:
                console.log("Product deleted")
            default:
                console.error(response.message)
        }
    });
}

document.querySelectorAll('.btn-product-delete').forEach(button => {
    button.addEventListener('click', async function() {
      const productId = this.getAttribute('data-id');
      await deleteProduct(productId);
      location.reload();
    });
  });

document.getElementById("btnSync").addEventListener("click", () => {
    location.reload();
})