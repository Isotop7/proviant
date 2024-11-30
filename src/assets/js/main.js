let inputSearch = document.getElementById('inputSearch');
let btnSearch = document.getElementById('btnSearch');

function handleSearch() {
    let inputSearchValue = inputSearch.value;
    window.location.href = `${window.location.protocol}//${window.location.host}/web/products/search?productName=${inputSearchValue}`;
};

btnSearch.onsubmit = ((event) => {
    event.preventDefault();
    event.stopPropagation();
    handleSearch();
});
btnSearch.onclick = ((event) => {
    event.preventDefault();
    event.stopPropagation();
    handleSearch();
});