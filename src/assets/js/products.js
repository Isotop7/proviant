async function bulkDeleteProducts(productIDs) {
  await proviant.bulkDeleteProducts(productIDs).then((response) => {
      if (response.code == 200) {
        console.log("Products archived")
      } else {
        console.error(response.message)
      }
  });
}

async function bulkRestoreProducts(productIDs) {
  await proviant.bulkRestoreProducts(productIDs).then((response) => {
      if (response.code == 200) {
        console.log("Products restored")
      } else {
        console.error(response.message)
      }
  });
}

async function bulkArchiveProducts(productIDs) {
  await proviant.bulkArchiveProducts(productIDs).then((response) => {
      if (response.code == 200) {
        console.log("Products archived")
      } else {
        console.error(response.message)
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
    const editBtn = document.getElementById('edit-product');
    if (editBtn) {
        editBtn.disabled = selectedProducts.length !== 1;
    }
}

/* Event delegation for clicks */
document.addEventListener("click", function (event) {
    const target = event.target;

    // Edit product button
    if (target.closest("#edit-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        if (selectedProducts.length === 1) {
            window.location.href = `/web/products/${selectedProducts[0]}/edit`;
        }
        return;
    }

    // Delete product button
    if (target.closest("#delete-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        const count = selectedProducts.length;
        proviant.showConfirm(
            'Delete Products',
            `Delete ${count} selected product${count !== 1 ? 's' : ''}? This cannot be undone.`,
            function () {
                bulkDeleteProducts(selectedProducts).then(() => location.reload());
            },
            'Delete',
            'danger'
        );
        return;
    }

    // Restore product button
    if (target.closest("#restore-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        bulkRestoreProducts(selectedProducts).then(() => location.reload());
        return;
    }

    // Archive product button
    if (target.closest("#archive-product")) {
        event.preventDefault();
        const selectedProducts = Array.from(document.querySelectorAll('input[type="checkbox"]:checked')).map(checkbox => checkbox.id.split('-')[1]);
        const count = selectedProducts.length;
        proviant.showConfirm(
            'Archive Products',
            `Archive ${count} selected product${count !== 1 ? 's' : ''}?`,
            function () {
                bulkArchiveProducts(selectedProducts).then(() => location.reload());
            },
            'Archive',
            'warning'
        );
        return;
    }

    // Card click
    const card = target.closest('.card');
    if (card) {
        const cardId = card.id;
        const productId = cardId.split('-')[1];
        const checkbox = document.getElementById(`checkbox-${productId}`);
        if (checkbox) {
            checkbox.checked = !checkbox.checked;
            card.classList.toggle('border-info');
            handleSelect();
            handleCardClickEffect(cardId);
        }
        return;
    }

    // Search button
    if (target.closest("#search-btn")) {
        event.preventDefault();
        performSearch();
        return;
    }

    // Show All button
    if (target.closest("#show-all-btn")) {
        event.preventDefault();
        window.location.href = "/web/products";
        return;
    }
});

/* Event delegation for checkbox changes */
document.addEventListener("change", function (event) {
    const target = event.target;
    if (target.matches('input[type="checkbox"]')) {
        const cardId = `card-${target.id.split('-')[1]}`;
        const card = document.getElementById(cardId);
        const checkbox = document.getElementById(`checkbox-${target.id.split('-')[1]}`);
        if (card) {
            card.classList.toggle('border-info');
            if (checkbox) {
                checkbox.checked = !checkbox.checked;
            }
        }
    }
});

/* Event delegation for keypress (search input Enter key) */
document.addEventListener("keypress", function (event) {
    const target = event.target;
    if (target.id === "search-query" && event.key === "Enter") {
        event.preventDefault();
        performSearch();
    }
});

// Function to perform search
function performSearch() {
    const queryParam = document.getElementById("search-param");
    const queryValue = document.getElementById("search-query");
    const sortParam = document.getElementById("sort-param");
    const sortOrder = document.getElementById("sort-order");

    // Build query string and redirect to products page
    const queryString = new URLSearchParams({
        queryParam: queryParam ? queryParam.value || "product_name" : "product_name",
        queryValue: queryValue ? queryValue.value || "" : "",
        sort: sortParam ? sortParam.value || "created_at" : "created_at",
        order: sortOrder ? sortOrder.value || "asc" : "asc",
    }).toString();

    window.location.href = `/web/products?${queryString}`;
}