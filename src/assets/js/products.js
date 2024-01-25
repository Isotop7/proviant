function showToast(message) {
    let infoToast = document.getElementById('infoToast');
    let infoToastBody = document.getElementById('infoToastBody');
    infoToastBody.innerText = message;
    let toastBootstrap = bootstrap.Toast.getOrCreateInstance(infoToast);
    toastBootstrap.show();
}

function multipleCheckboxesSelected() {
    let counter = 0;
    let checkboxes = document.getElementsByClassName("form-check-input");
    for (let checkbox of checkboxes) {
        if (checkbox.checked) {
            counter++;
        }
        if (counter > 1) {
            return true;
        }
    }
    return counter > 1;
}

function getSelectedProduct() {
    let checkboxes = document.getElementsByClassName("form-check-input");
    for (let checkbox of checkboxes) {
        if (checkbox.checked) {
            return checkbox.value;
        }
    }
    return 0;
}

function getSelectedProducts() {
    let selectedIDs = [];
    let checkboxes = document.getElementsByClassName("form-check-input");
    for (let checkbox of checkboxes) {
        if (checkbox.checked) {
            selectedIDs.push(checkbox.value);
        }
    }
    return selectedIDs;
}

async function deleteProducts() {
    let productIDs = getSelectedProducts();
    if (productIDs.length <= 0) {
        return;
    }
    for(let productID of productIDs) {
        await expiro.deleteProduct(productID).then((response) => {
            switch (response.code) {
                case 200:
                    console.log("Products deleted")
                default:
                    console.error(response.message)
            }
        })
    }
}

function handleButtonView() {
    if (multipleCheckboxesSelected()) {
        showToast("Viewing multiple products is not supported. Please select only one product!");
    } else {
        let id = getSelectedProduct();
        if (id > 0) {
            window.location.href = `${window.location.protocol}//${window.location.host}/web/products/${id}/view`;
        } else {
            showToast("You need to select one product");
        }
    }
}

function handleButtonEdit() {
    if (multipleCheckboxesSelected()) {
        showToast("Editing multiple products is not supported. Please select only one product!");
    } else {
        let id = getSelectedProduct();
        if (id > 0) {
            window.location.href = `${window.location.protocol}//${window.location.host}/web/products/${id}/edit`;
        } else {
            showToast("You need to select one product");
        }
    }
}

async function handleButtonDelete() {
    await deleteProducts();
    location.reload();
}

document.getElementById("checkbox-all").addEventListener("change", () => {
    let checkboxes = document.getElementsByClassName("form-check-input");
    for (checkbox of checkboxes) {
        console.log("Changed checkbox " + checkbox.id);
        checkbox.checked = document.getElementById("checkbox-all").checked;
    }
});
document.getElementById("btnView").addEventListener("click", handleButtonView);
document.getElementById("btnView").addEventListener("submit", handleButtonView);
document.getElementById("btnEdit").addEventListener("click", handleButtonEdit);
document.getElementById("btnEdit").addEventListener("submit", handleButtonEdit);
document.getElementById("btnDelete").addEventListener("click", handleButtonDelete);
document.getElementById("btnDelete").addEventListener("submit", handleButtonDelete);
document.getElementById("btnSync").addEventListener("click", () => {
    location.reload();
})