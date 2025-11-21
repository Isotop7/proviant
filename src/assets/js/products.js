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

function handleCardClickEffect(cardId) {
    const card = document.getElementById(cardId);
    console.log(cardId);
    if (card) {
        card.classList.add('card-clicked');
        setTimeout(() => card.classList.remove('card-clicked'), 100);
    }
}

async function handleSelect() {
    const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
    if (selectedProducts.length == 1) {
      document.getElementById('edit-product').disabled = false;
    } else {
      document.getElementById('edit-product').disabled = true;
    }
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

document.querySelectorAll('.card').forEach(card => {
    card.addEventListener('click', async function () {
      const cardId = this.id;
      const productId = cardId.split('-')[1];
      const checkbox = document.getElementById(`checkbox-${productId}`);
      if (checkbox) {
          checkbox.checked = !checkbox.checked;
          document.getElementById(cardId).classList.toggle('border-info')
          handleSelect();
      }
      handleCardClickEffect(cardId);
    });
});
