// Fetch products from the server
async function fetchProducts(queryParam, queryValue, sortParam, sortOrder) {
    const queryString = new URLSearchParams({
        queryParam: queryParam || "product_name",
        queryValue: queryValue || "",
        sort: sortParam || "product_name",
        order: sortOrder || "asc",
    }).toString();

    try {
        const response = await fetch(`/api/v1/products/search?${queryString}`);
        const data = await response.json();
        // Check if products exist
        if (data && data.length > 0) {
            renderProducts(data);
        } else {
            renderNoProductsMessage();
        }
    } catch (error) {
        console.error("Error fetching products:", error);
        renderErrorMessage();
    }
}

// Render categories
badgifyCategories = function (categories, limit) {
    output = "";
    // Split categories
    categories = categories.split(",");
    for (let index = 0; index < categories.length; index++) {
        // Get element and split
        const category = categories[index].trim();
        const contents = category.split(":");

        // early return
        if (index == limit) {
            break;
        }

        // Check if language was found
        if (contents.length == 2) {
            lang = contents[0].trim()
            definition = contents[1].trim()
            output += `<span class="badge bg-dark me-3">${lang}</span>${definition}</br>`
        } else {
            output += `${category}</br>`;
        }
    }
    return output;
}

// Render expire at
colorExpiry = function (date) {
    if (new Date(date) < Date.now()) { 
        return "bg-danger" 
    } else { 
        return "bg-primary" 
    }
}

// Render product list
function renderProducts(products) {
    const productList = document.getElementById("product-list");
    productList.innerHTML = products
        .map(
            (product) => `
            <div class="col">
                <div class="card h-100 d-flex flex-column">
                    <div class="row h-100 px-2">
                        <div class="col-md-3">
                            <div class="product-card-img-container">
                                <img src="${product.imageUrl}" class="img-fluid product-card-img">
                            </div>
                        </div>
                        <div class="col-md-9">
                            <div class="card-body d-flex flex-column">
                                <div class="row align-items-center">
                                    <div class="col-9">
                                        <a href="/web/products/${product.ID}/view" class="text-decoration-none">
                                            <h4 class="card-title">
                                                ${product.productName}
                                            </h4>
                                        </a>
                                    </div>
                                    <div class="col-3" style="text-align: right;">
                                        <h4>
                                            <a href="/web/products/${product.ID}/view">
                                                <span class="badge bg-primary">#${product.ID}</span>
                                            </a>
                                        </h4>
                                    </div>
                                </div>
                                <div class="card-text font-monospace">
                                    ${product.barcode}
                                </div>
                                <div class="card-text">
                                    <p>
                                        ${badgifyCategories(product.categories, 5)}
                                    </p>
                                </div>
                            </div>
                        </div>
                    </div>
                    <ul class="list-group list-group-flush d-flex justify-content-between">
                        <li class="list-group-item d-flex justify-content-evenly align-items-center">
                            <div class="ms-2 me-auto">
                                <i class="bi bi-trash3 me-3"></i>
                            </div>
                            <span class="badge rounded-pill ${colorExpiry(product.expireAt)}">
                                ${new Date(product.expireAt).toLocaleString("de-DE")}
                            </span>
                        </li>
                        <li class="list-group-item d-flex justify-content-evenly align-items-center">
                            <div class="ms-2 me-auto">
                                <i class="bi bi-upc-scan me-3"></i>
                            </div>
                            <span class="badge bg-primary rounded-pill">
                                ${new Date(product.scannedAt).toLocaleString("de-DE")}
                            </span>
                        </li>
                        <li class="list-group-item d-flex justify-content-evenly align-items-center">
                            <div class="ms-2 me-auto">
                                <i class="bi bi-bell me-3"></i>
                            </div>
                            <span class="badge bg-primary rounded-pill">
                                ${new Date(product.notifiedAt).toLocaleString("de-DE")}
                            </span>
                        </li>
                    </ul>
                    <div class="btn-group" role="group" aria-label="Basic example">
                        <a class="btn btn-outline-info" href="/web/products/${product.ID}/edit">
                            <i class="bi bi-pencil-square me-2"></i>Edit
                        </a>
                        <button class="btn btn-outline-warning btn-product-delete" data-id="${product.ID}">
                            <i class="bi bi-trash2 me-2"></i>Delete
                        </button>
                    </div>
                </div>
            </div>`
        )
        .join("");
}

// Render no products message
function renderNoProductsMessage() {
    const productList = document.getElementById("product-list");
    productList.innerHTML = `
        <div class="mx-auto">
            <div class="alert alert-warning text-center">No products found.</div>
        </div>
    `;
}

// Render error message
function renderErrorMessage() {
    const productList = document.getElementById("product-list");
    productList.innerHTML = `
        <div class="mx-auto">
            <div class="alert alert-danger text-center">An error occurred while fetching products.</div>
        </div>
    `;
}

// Event listener for the search button
document.getElementById("search-btn").addEventListener("click", () => {
    const queryParam = document.getElementById("search-param").value;
    const queryValue = document.getElementById("search-query").value;
    const sortParam = document.getElementById("sort-param").value;
    const sortOrder = document.getElementById("sort-order").value;

    fetchProducts(queryParam, queryValue, sortParam, sortOrder);
});

// Fetch initial products on page load
fetchProducts();