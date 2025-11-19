async function bulkDeleteProducts(productIDs) {
  await proviant.bulkDeleteProducts(productIDs).then((response) => {
      switch (response.code) {
          case 200:
              console.log("Products archived")
          default:
              console.error(response.message)
      }
  });
}

async function bulkArchiveProducts(productIDs) {
  await proviant.bulkArchiveProducts(productIDs).then((response) => {
      switch (response.code) {
          case 200:
              console.log("Products archived")
          default:
              console.error(response.message)
      }
  });
}

async function handleSelect() {
    const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
    if (selectedProducts.length == 1) {
      document.getElementById('edit-product').disabled = false;
    } else {
      document.getElementById('edit-product').disabled = true;
    }

    const cards = Array.from(document.querySelectorAll('.card')).map(card => card.id);
    cards.forEach(cardId => {
      let id = cardId.split('-')[1];
      if (selectedProducts.includes(id)) {
        document.getElementById(cardId).classList.add('border-info');
      } else {
        document.getElementById(cardId).classList.remove('border-info');
      }
    });
}

document.querySelectorAll('#edit-product').forEach(button => {
    button.addEventListener('click', async function () {
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        window.location.href = `${window.location.protocol}//${window.location.host}/web/products/${selectedProducts[0]}/edit`;
    });
});

document.querySelectorAll('#delete-product').forEach(button => {
    button.addEventListener('click', async function () {
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        await bulkDeleteProducts(selectedProducts);
        location.reload();
    });
});

document.querySelectorAll('#archive-product').forEach(button => {
    button.addEventListener('click', async function () {
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        await bulkArchiveProducts(selectedProducts);
        location.reload();
    });
});

document.querySelectorAll('input[type="checkbox"]').forEach(checkbox => {
    checkbox.addEventListener('change', async function () {
      handleSelect();
    });
});
