/* Event delegation for logout button */
document.addEventListener("click", function (event) {
    const target = event.target;

    if (target.closest("[data-logout]")) {
        event.preventDefault();
        void proviant.logoutUser().then(() => {
            window.location.href = '/web';
        });
        return;
    }
});

document.addEventListener('DOMContentLoaded', function () {
    // Show offline banner immediately if page loaded from cache while offline
    updateOfflineBanner();

    // Flush any queued edits from previous session. The offlineQueue.js seam is
    // guarded: if that script 404s or is refused by CSP, an unguarded call
    // would abort the rest of this handler and leave the banner permanently
    // blind to stuck changes.
    if (navigator.onLine && proviant._flushQueue) {
        void proviant._flushQueue();
    }
    if (proviant._updateOfflineBadge) {
        void proviant._updateOfflineBadge();
    }
});

// Offline banner toggle. Stuck changes keep the banner up while online: it is
// the only place the user can see and clear them.
function updateOfflineBanner() {
    var banner = document.getElementById('offlineBanner');
    if (!banner) return;
    var offline = !navigator.onLine;
    var stuck = !offline && !!(proviant.offline && proviant.offline.failedCount > 0);
    if (offline || stuck) {
        banner.classList.remove('d-none');
    } else {
        banner.classList.add('d-none');
    }
    setOfflineBannerText(offline, stuck);
}

// The banner copy names the network, which is the wrong reason once back online
// and only stuck changes hold the banner open.
function setOfflineBannerText(offline, stuck) {
    var icon = document.getElementById('offlineBannerIcon');
    var text = document.getElementById('offlineBannerText');
    if (icon) {
        icon.classList.toggle('bi-wifi-off', offline);
        icon.classList.toggle('bi-exclamation-triangle', stuck);
    }
    if (text) {
        text.textContent = stuck ? proviant.t('js.app.offlineBannerFailed') : proviant.t('common.offlineBanner');
    }
}

window.addEventListener('offline', updateOfflineBanner);
window.addEventListener('online', function () {
    updateOfflineBanner();
    if (proviant._flushQueue) void proviant._flushQueue();
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