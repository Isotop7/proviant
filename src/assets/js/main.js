/* Event delegation for logout button */
document.addEventListener("click", function (event) {
    const target = event.target;

    if (target.closest("#btnLogout")) {
        event.preventDefault();
        proviant.logoutUser().then(() => {
            window.location.href = '/web';
        });
        return;
    }
});

document.addEventListener('DOMContentLoaded', function () {
    var tooltipEls = document.querySelectorAll('[data-bs-toggle="tooltip"]');
    tooltipEls.forEach(function (el) { new bootstrap.Tooltip(el); });
});

if ('serviceWorker' in navigator) {
    window.addEventListener('load', function () {
        navigator.serviceWorker.register('/sw.js').catch(function (err) {
            console.warn('Service worker registration failed:', err);
        });
    });
}