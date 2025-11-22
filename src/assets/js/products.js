async function bulkDeleteProducts(productIDs) {
  await proviant.bulkDeleteProducts(productIDs).then((response) => {
      switch (response.code) {
          case 200:
              console.log("Products archived")
              break;
          default:
              console.error(response.message)
              break;
      }
  });
}

async function bulkArchiveProducts(productIDs) {
  await proviant.bulkArchiveProducts(productIDs).then((response) => {
      switch (response.code) {
          case 200:
              console.log("Products archived")
              break;
          default:
              console.error(response.message)
              break;
      }
  });
}

function handleCardClickEffect(cardId) {
    const card = document.getElementById(cardId);
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
        globalThis.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/web/products/${selectedProducts[0]}/edit`;
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
      const cardId = `card-${this.id.split('-')[1]}`;
      const card = document.getElementById(cardId);
      const checkbox = document.getElementById(`checkbox-${this.id.split('-')[1]}`);
      if (card) {
        document.getElementById(cardId).classList.toggle('border-info')
        checkbox.checked = !checkbox.checked;
      }
    });
    checkbox.addEventListener('click', async function () {
      const cardId = `card-${this.id.split('-')[1]}`;
      const card = document.getElementById(cardId);
      if (card) {
        document.getElementById(cardId).classList.toggle('border-info')
      }
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
          handleCardClickEffect(cardId);
      }
    });
});
