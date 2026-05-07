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

    // Show offline banner immediately if page loaded from cache while offline
    updateOfflineBanner();

    // Flush any queued edits from previous session
    if (navigator.onLine) {
        proviant._flushQueue();
    }
    proviant._updateOfflineBadge();
});

// Offline banner toggle
function updateOfflineBanner() {
    var banner = document.getElementById('offlineBanner');
    if (!banner) return;
    if (!navigator.onLine) {
        banner.classList.remove('d-none');
    } else {
        banner.classList.add('d-none');
    }
}

window.addEventListener('offline', updateOfflineBanner);
window.addEventListener('online', function () {
    updateOfflineBanner();
    proviant._flushQueue();
});

if ('serviceWorker' in navigator) {
    window.addEventListener('load', function () {
        var firstActivation = !navigator.serviceWorker.controller;
        navigator.serviceWorker.register('/sw.js').catch(function (err) {
            console.warn('Service worker registration failed:', err);
        });
        navigator.serviceWorker.addEventListener('controllerchange', function () {
            if (firstActivation) { firstActivation = false; return; }
            var banner = document.getElementById('swUpdateBanner');
            if (banner) banner.classList.remove('d-none');
        });
    });
}