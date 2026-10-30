'use strict';

// The duplicate prompt asks what to do with a pack whose product the household
// already has. The server owns the match; this file only asks.
//
// The three choices are deliberate: adding a pack to the matched row, writing
// a separate row, and adding nothing at all. A refusal that names no choice
// would leave the user guessing what the app did with their scan.

// The pending prompt. It is read inside the click handler, never captured in
// a closure at load time, so a second prompt replaces the first cleanly.
var duplicatePromptState = { duplicate: null, quantity: 1, handlers: null };

function duplicatePromptElement(id) {
    return document.getElementById(id);
}

// The stored expiry is the UTC midnight of the day the user picked, so it is
// read in UTC: the browser's own zone would show the day before.
function duplicatePromptExpiry(duplicate) {
    if (!duplicate || !duplicate.expireAt) return proviant.t("js.products.duplicateNoExpiry");
    const date = new Date(duplicate.expireAt);
    if (Number.isNaN(date.getTime())) return proviant.t("js.products.duplicateNoExpiry");
    return date.toLocaleDateString(window.proviantLang || "en", { timeZone: "UTC" });
}

function duplicatePromptClose() {
    const dialog = duplicatePromptElement('duplicateProductModal');
    if (!dialog) return;
    const instance = bootstrap.Dialog.getInstance(dialog);
    if (instance) instance.hide();
}

// showDuplicatePrompt fills the dialog with the matched product and opens it.
// The handlers answer the three choices; a missing handler means the choice
// does nothing.
proviant.showDuplicatePrompt = function (options) {
    const dialog = duplicatePromptElement('duplicateProductModal');
    if (!dialog) return;

    const duplicate = (options && options.duplicate) || {};
    duplicatePromptState.duplicate = duplicate;
    duplicatePromptState.quantity = (options && options.quantity) || 1;
    duplicatePromptState.handlers = options || {};

    const nameEl = duplicatePromptElement('duplicateProductName');
    if (nameEl) nameEl.textContent = duplicate.productName || '';

    const expiryEl = duplicatePromptElement('duplicateProductExpiry');
    if (expiryEl) expiryEl.textContent = proviant.t("js.products.duplicateExpiry", duplicatePromptExpiry(duplicate));

    const locationEl = duplicatePromptElement('duplicateProductLocation');
    if (locationEl) {
        const location = duplicate.storageLocation ? duplicate.storageLocation.name : '';
        locationEl.textContent = location
            ? proviant.t("js.products.duplicateLocation", location)
            : proviant.t("js.products.duplicateNoLocation");
    }

    bootstrap.Dialog.getOrCreateInstance(dialog).show();

    // The live region carries the whole question, so a screen reader hears
    // the three choices and not only the title. It is filled after the dialog
    // is open: a closed dialog is hidden, and a change to a live region that
    // is not rendered announces nothing.
    const announcementEl = duplicatePromptElement('duplicateProductAnnouncement');
    if (announcementEl) {
        announcementEl.textContent = [
            proviant.t("products.duplicateTitle"),
            proviant.t("js.products.duplicateBody", duplicate.productName || ''),
            proviant.t("products.duplicateOptionAddPack"),
            proviant.t("products.duplicateOptionAddSeparately"),
            proviant.t("products.duplicateOptionCancel"),
        ].join(' ');
    }
};

document.addEventListener('click', function (event) {
    const target = event.target;
    if (!target || !target.closest) return;

    const addPack = target.closest('#btnDuplicateAddPack');
    const addSeparately = target.closest('#btnDuplicateAddSeparately');
    const cancel = target.closest('#btnDuplicateCancel');
    if (!addPack && !addSeparately && !cancel) return;

    event.preventDefault();

    const handlers = duplicatePromptState.handlers;
    const duplicate = duplicatePromptState.duplicate;
    const quantity = duplicatePromptState.quantity;
    duplicatePromptState.handlers = null;
    duplicatePromptState.duplicate = null;
    duplicatePromptClose();

    if (addPack && handlers && handlers.onAddPack) {
        handlers.onAddPack(duplicate, quantity);
        return;
    }
    if (addSeparately && handlers && handlers.onAddSeparately) {
        handlers.onAddSeparately(duplicate, quantity);
        return;
    }
    if (cancel && handlers && handlers.onCancel) {
        handlers.onCancel(duplicate, quantity);
    }
});
