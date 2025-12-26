// Event listener for search button
document.getElementById("search-btn").addEventListener("click", () => {
    const queryParam = document.getElementById("search-param").value;
    const queryValue = document.getElementById("search-query").value;
    const sortParam = document.getElementById("sort-param").value;
    const sortOrder = document.getElementById("sort-order").value;
    
    // Build query string and redirect to products page
    const queryString = new URLSearchParams({
        queryParam: queryParam || "product_name",
        queryValue: queryValue || "",
        sort: sortParam || "created_at",
        order: sortOrder || "asc",
    }).toString();
    
    window.location.href = `/web/products?${queryString}`;
});

// Event listener for Show All button
document.getElementById("show-all-btn").addEventListener("click", () => {
    // Redirect to products page without any query parameters (clears all filters)
    window.location.href = "/web/products";
});
