const proviant = {};
// A top-level `const` in a classic script is a global lexical binding, not a
// property of window, so offlineQueue.js — which wraps proviant.createProduct,
// proviant.updateProductQuantity and proviant.getProducts — would otherwise find
// window.proviant undefined, wrap an empty object of its own and leave every one
// of those features inert.
window.proviant = proviant;

// Read the csrf_token cookie set by the server's CSRF middleware.
proviant._getCsrfToken = function () {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
};

// Translate a catalog key. The server injects window.proviantI18n per page
// (see i18n.FuncMap); %s / %d / %v placeholders fill from the remaining args.
proviant.t = function (key) {
  const args = Array.prototype.slice.call(arguments, 1);
  const catalog = window.proviantI18n || {};
  let text = Object.prototype.hasOwnProperty.call(catalog, key) ? catalog[key] : key;
  args.forEach(function (arg) {
    text = text.replace(/%[sdv]/, String(arg));
  });
  return text;
};

// Translate a plural key: picks key_one for n === 1, key_other otherwise, with
// n as the first %s / %d placeholder (mirrors i18n.Tn on the server).
proviant.tn = function (key, n) {
  const suffix = n === 1 ? "_one" : "_other";
  const catalog = window.proviantI18n || {};
  const suffixed = key + suffix;
  const lookup = Object.prototype.hasOwnProperty.call(catalog, suffixed) ? suffixed : key;
  return proviant.t.apply(null, [lookup, n].concat(Array.prototype.slice.call(arguments, 2)));
};

// quantityLabel renders a pack count the way every quantity surface does: the
// count and the localized pack noun ("2 Stk."). A count never depends on a
// unit, so the label is the same everywhere.
proviant.quantityLabel = function (quantity) {
  return String(quantity) + " " + proviant.t("common.qty");
};

// renderQuantityLabel writes the count of a product or shopping list entry
// back into its label, reading the count from the data attribute so a
// stepper update keeps the display in sync.
proviant.renderQuantityLabel = function (element) {
  if (!element || element.dataset.quantity === undefined) return;
  const quantity = Number.parseInt(element.dataset.quantity, 10) || 0;
  element.textContent = proviant.quantityLabel(quantity);
};

// Intercept all fetch calls to inject X-CSRF-Token on state-mutating requests.
(function () {
  const _originalFetch = window.fetch;
  const mutatingMethods = new Set(["POST", "PATCH", "PUT", "DELETE"]);
  window.fetch = function (resource, options) {
    options = options || {};
    const method = ((options.method) || "GET").toUpperCase();
    if (mutatingMethods.has(method)) {
      const csrfToken = proviant._getCsrfToken();
      if (csrfToken) {
        options.headers = Object.assign({}, options.headers, { "X-CSRF-Token": csrfToken });
      }
    }
    return _originalFetch.call(this, resource, options);
  };
})();

proviant.debug = function () {
  console.log("Proviant loaded");
};

/* ── UI helpers ──────────────────────────────────────────────────────────────── */
// Page-level feedback. The design system replaces the feedback modal with a
// stack of toasts: the message is visible without blocking the page, and a
// second message does not swallow the first one's callback.
const TOAST_CONFIG = {
  success: { icon: "bi-check-circle-fill", delay: 4000 },
  error:   { icon: "bi-x-circle-fill", delay: null },
  warning: { icon: "bi-exclamation-triangle-fill", delay: 6000 },
  info:    { icon: "bi-info-circle-fill", delay: 4000 },
};

const TOAST_MAX = 4;

function toastStack() {
  let stack = document.getElementById("proviantToastStack");
  if (!stack) {
    stack = document.createElement("div");
    stack.className = "toast-stack";
    stack.id = "proviantToastStack";
    stack.setAttribute("aria-live", "polite");
    stack.setAttribute("aria-atomic", "true");
    document.body.appendChild(stack);
  }
  return stack;
}

proviant.showFeedback = function (type, title, message, onClose) {
  const cfg = TOAST_CONFIG[type] || TOAST_CONFIG.info;
  const stack = toastStack();

  const toastEl = document.createElement("div");
  toastEl.className = "toast toast--" + (TOAST_CONFIG[type] ? type : "info");
  toastEl.setAttribute("role", "status");
  toastEl.setAttribute("aria-live", "polite");
  toastEl.setAttribute("aria-atomic", "true");

  const header = document.createElement("div");
  header.className = "toast-header";

  const icon = document.createElement("i");
  icon.className = "bi " + cfg.icon + " toast__icon";
  icon.setAttribute("aria-hidden", "true");
  header.appendChild(icon);

  const titleEl = document.createElement("strong");
  titleEl.className = "toast-title";
  titleEl.textContent = title || "";
  header.appendChild(titleEl);

  const closeBtn = document.createElement("button");
  closeBtn.type = "button";
  closeBtn.className = "btn-close";
  closeBtn.setAttribute("data-bs-dismiss", "toast");
  closeBtn.setAttribute("aria-label", proviant.t("a11y.close"));
  header.appendChild(closeBtn);

  toastEl.appendChild(header);

  if (message) {
    const body = document.createElement("div");
    body.className = "toast-body";
    body.textContent = message;
    toastEl.appendChild(body);
  }

  stack.appendChild(toastEl);

  // The stack never grows without bound. The oldest toast gives way, and its
  // callback still runs — a state refresh must not be lost to a new message.
  while (stack.children.length > TOAST_MAX) {
    const oldest = stack.firstElementChild;
    oldest.dispatchEvent(new Event("hidden.bs.toast"));
    oldest.remove();
  }

  // An error stays until it is dismissed: it is the one message that must
  // not disappear on its own.
  const autohide = cfg.delay !== null;
  let dismissed = false;
  const dismiss = function () {
    if (dismissed) return;
    dismissed = true;
    toastEl.remove();
    if (onClose) onClose();
  };

  // onClose fires on every dismissal path: the close button, and the
  // timeout for the auto-hiding types.
  toastEl.addEventListener("hidden.bs.toast", dismiss);

  if (typeof bootstrap !== "undefined" && bootstrap.Toast) {
    bootstrap.Toast.getOrCreateInstance(toastEl, {
      autohide: autohide,
      delay: autohide ? cfg.delay : 5000,
    }).show();
  } else {
    // No Bootstrap in this document — the script can run before the bundle
    // is parsed. Show the toast and dismiss it on a timer.
    toastEl.classList.add("show");
    if (autohide) setTimeout(dismiss, cfg.delay);
  }
  return toastEl;
};

proviant.showConfirm = function (title, message, onConfirm, confirmLabel, confirmType) {
  const modal = document.getElementById("proviantConfirmModal");
  if (!modal) return;
  const titleEl = document.getElementById("proviantConfirmTitle");
  const msgEl = document.getElementById("proviantConfirmMessage");
  const btnEl = document.getElementById("proviantConfirmBtn");
  if (titleEl) titleEl.textContent = title || proviant.t("js.feedback.confirmDefaultTitle");
  if (msgEl) msgEl.textContent = message || "";
  if (btnEl) {
    const confirmClasses = {
      primary: "btn btn-solid theme-primary",
      secondary: "btn btn-outline theme-secondary",
      warning: "btn btn-solid theme-primary",
      danger: "btn btn-solid theme-danger",
    };
    btnEl.textContent = confirmLabel || proviant.t("common.confirm");
    btnEl.className = confirmClasses[confirmType] || confirmClasses.danger;
    btnEl.onclick = function () { bootstrap.Dialog.getInstance(modal).hide(); onConfirm(); };
  }
  bootstrap.Dialog.getOrCreateInstance(modal).show();
};

proviant.copyToClipboard = async function (text) {
  try { await navigator.clipboard.writeText(text); return true; } catch { return false; }
};

/* ── Utility helpers ────────────────────────────────────────────────────────── */

proviant.badgifyCategories = function (categories, limit) {
  let output = "";
  const arr = categories.split(",");
  for (let i = 0; i < arr.length; i++) {
    if (i === limit) break;
    const cat = arr[i].trim();
    const parts = cat.split(":");
    if (parts.length === 2) {
      output += `<span class="badge text-bg-secondary me-3">${parts[0].trim()}</span>${parts[1].trim()}</br>`;
    } else {
      output += `${cat}</br>`;
    }
  }
  return output;
};

proviant.colorExpiry = function (date) {
  return new Date(date) < Date.now() ? "bg-danger" : "bg-primary";
};

/* ── Offline edit queue ──────────────────────────────────────────────────────── */
// The queue itself lives in offlineQueue.js, which wraps updateProductQuantity and
// createProduct at load. Everything here is a plain network call, so this module
// holds no offline state of its own.

proviant._sendQuantityDelta = async function (productID, delta, clientMutationId) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}/quantity`;
  let res;
  try {
    res = await fetch(url, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ delta, clientMutationId }) });
  } catch (cause) {
    // Tagged so the outbox can tell "the request never left" from "the server
    // answered with a body I cannot parse": only the first costs no retry
    // attempt, or a 502-with-HTML would park nothing and retry forever.
    const err = new Error(cause && cause.message ? cause.message : String(cause));
    err.transport = true;
    throw err;
  }
  let body;
  try {
    body = await res.json();
  } catch (cause) {
    // The server answered but the body is not JSON — a proxy error page,
    // say. Carry the status so the outbox can tell a server-side failure
    // (5xx: pace and retry) from a broken request (4xx: park it), while
    // keeping the parse message: the verdict stays visible either way.
    const err = new Error(cause && cause.message ? cause.message : String(cause));
    err.status = res.status;
    throw err;
  }
  return {
    code: res.status,
    message: body.message,
    // The outbox paces itself on this; the status alone cannot say how long
    // the server wants the client to stay away.
    retryAfter: res.headers && res.headers.get ? res.headers.get("Retry-After") : null,
    deleted: res.status === 200 && typeof body.message === "string" && body.message.includes("deleted"),
  };
};

proviant.updateProductQuantity = async function (productID, delta, clientMutationId) {
  return proviant._sendQuantityDelta(productID, delta, clientMutationId);
};

/* ── Product API ─────────────────────────────────────────────────────────────── */
proviant.createProduct = async function (barcode, expireAt, quantity, storageLocationId, isPrivate = false, clientSyncId, allowDuplicate = false) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products`;
  const payload = { barcode, expireAt, quantity: quantity || 1, storageLocationId: storageLocationId || null, isPrivate, clientSyncId: clientSyncId || null };
  if (allowDuplicate) payload.allowDuplicate = true;
  const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
  const body = await res.json();
  // A refusal that names a duplicate carries the matched product, so the
  // client can ask whether the pack belongs to that row.
  return { code: res.status, message: body.message, action: body.action, duplicate: body.duplicate || null, clientSyncId: clientSyncId || null };
};

proviant._sendProductAction = async function (action, productID, quantity, clientMutationId) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}/${action}`;
  let res;
  try {
    res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ quantity, clientMutationId }),
    });
  } catch (cause) {
    const err = new Error(cause && cause.message ? cause.message : String(cause));
    err.transport = true;
    throw err;
  }
  let body;
  try {
    body = await res.json();
  } catch (cause) {
    const err = new Error(cause && cause.message ? cause.message : String(cause));
    err.status = res.status;
    throw err;
  }
  return {
    code: res.status,
    message: body.message,
    removed: typeof body.removed === "number" ? body.removed : null,
    retryAfter: res.headers && res.headers.get ? res.headers.get("Retry-After") : null,
  };
};

proviant.consumeProduct = async function (productID, quantity, clientMutationId) {
  return proviant._sendProductAction("consume", productID, quantity, clientMutationId);
};

proviant.wasteProduct = async function (productID, quantity, clientMutationId) {
  return proviant._sendProductAction("waste", productID, quantity, clientMutationId);
};

proviant.getOpenFoodFactsData = async function (barcode) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/openfoodfacts/${barcode}`;
  const res = await fetch(url, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.getProductsByBarcode = async function (barcode) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/byBarcode/${barcode}`;
  const res = await fetch(url, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.editProduct = async function (product) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${product.ID}`;
  const res = await fetch(url, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(product) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.deleteProduct = async function (productID, archiveOnly) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}`;
  if (archiveOnly) url += "?archiveOnly=true";
  const res = await fetch(url, { method: "DELETE", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.restoreProduct = async function (productID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}/restore`;
  const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.getRestockSuggestion = async function (productID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}/restock-suggestion`;
  const res = await fetch(url, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.markProductOpened = async function (productID, force = false) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}/open`;
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ force }),
  });
  let body = {};
  try { body = await res.json(); } catch (_) { /* non-JSON body stays {} */ }
  return { code: res.status, product: body, conflict: res.status === 409, openedAt: body.openedAt };
};

proviant.addToShoppingList = async function (productId, quantity) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/shopping-list`;
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      productId: Number.parseInt(productId, 10),
      quantity: quantity || 1,
    }),
  });
  if (!res.ok) {
    const body = await res.json().catch(function () { return {}; });
    throw new Error(body.message || proviant.t("js.feedback.addToShoppingListFailed"));
  }
  const body = await res.json();
  return { code: res.status, message: body };
};

/* runWithButtonBusyState runs `work` while showing a spinner on `btn`,
   then briefly shows a success check, and finally restores the original
   button HTML. If `work` throws/rejects with a `cancelled` sentinel
   (see proviant.CANCEL) the work is treated as a no-op: the button is
   restored and no feedback is shown. Any other error invokes `onError`
   and restores the button immediately. */
proviant.CANCEL = Symbol('cancel');
proviant.runWithButtonBusyState = async function (btn, work, successMessage, onError) {
  if (!btn) return;
  const originalHtml = btn.innerHTML;
  const setBusy = (busy) => {
    btn.disabled = busy;
    btn.innerHTML = busy
      ? '<i class="bi bi-hourglass-split"></i>'
      : originalHtml;
  };

  setBusy(true);
  let result;
  try {
    result = await work();
  } catch (err) {
    setBusy(false);
    if (err === proviant.CANCEL) return;
    if (onError) onError(err);
    return;
  }

  if (result === proviant.CANCEL) {
    setBusy(false);
    return;
  }

  btn.innerHTML = '<i class="bi bi-check-lg"></i>';
  setTimeout(() => { setBusy(false); }, 1500);

  if (successMessage) {
    proviant.showFeedback('success', proviant.t("js.feedback.added"), successMessage);
  }
};

/* ── Bulk product operations ─────────────────────────────────────────────────── */
proviant.bulkDeleteProducts = async function (productIDs) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulkDelete`;
  const res = await fetch(url, { method: "DELETE", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ productIDs }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.bulkArchiveProducts = async function (productIDs) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulkArchive`;
  const res = await fetch(url, { method: "DELETE", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ productIDs }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.bulkConsumeProducts = async function (productIDs) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulkConsume`;
  const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ productIDs }) });
  const body = await res.json();
  return { code: res.status, message: body.message, failedIds: body.failedIds || [] };
};

proviant.bulkWasteProducts = async function (productIDs) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulkWaste`;
  const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ productIDs }) });
  const body = await res.json();
  return { code: res.status, message: body.message, failedIds: body.failedIds || [] };
};

proviant.bulkRestoreProducts = async function (productIDs) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulkRestore`;
  const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ productIDs }) });
  const body = await res.json();
  return { code: res.status, message: body.message, failedIds: body.failedIds || [] };
};

proviant.cookProducts = async function (items) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/cook`;
  const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ items }) });
  const body = await res.json();
  return { code: res.status, consumed: body.consumed, partial: body.partial, errors: body.errors || [] };
};

proviant.bulkCreateProducts = async function (items) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulk`;
  // The bulk endpoint expects expireAt as a YYYY-MM-DD date string and reports
  // per-item outcomes as results[{index, status, reason}]; the batch UI works
  // with created/failed/errors, so normalize both here.
  const payload = (items || []).map((item) => ({ ...item, expireAt: String(item.expireAt || "").slice(0, 10) }));
  const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ items: payload }) });
  const body = await res.json();
  const results = Array.isArray(body.results) ? body.results : [];
  let created = 0;
  const errors = [];
  results.forEach((result) => {
    if (result.status === "created") {
      created += 1;
    } else {
      errors.push({ index: result.index, reason: result.reason });
    }
  });
  return { code: res.status, message: body.message, created, failed: errors.length, errors };
};

proviant.getStorageLocations = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/storage-locations`;
  const res = await fetch(url, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

// Clears the bulk selection before a reload. Browsers restore form control
// state (checkbox checked states) across location.reload(); without this the
// selection would land on the products that shifted into those table rows.
// Defined here (not products.js) because it is the default onDone for
// proviant.bulkAction, which pages without products.js may also call.
function clearBulkSelectionAndReload() {
  document.querySelectorAll('.row-checkbox:checked').forEach(function (cb) {
    cb.checked = false;
  });
  const selectAll = document.getElementById('selectAll');
  if (selectAll) selectAll.checked = false;
  if (typeof updateBulkSelected === 'function') updateBulkSelected();
  location.reload();
}

proviant.bulkAction = async function (type, ids, options = {}) {
  const { confirm: needsConfirm = false, confirmTitle = '', confirmMsg = '', onDone = () => clearBulkSelectionAndReload() } = options;
  const fn = { delete: proviant.bulkWasteProducts, restore: proviant.bulkRestoreProducts, archive: proviant.bulkConsumeProducts }[type];
  if (!fn) return;
  // Failures must reach the user, and the reload must wait for that message:
  // reloading right away would wipe the modal. Every bulk endpoint answers a
  // partial run with 200 + failedIds, so status alone is not enough to detect
  // it. Status first: a 404 carries failedIds for every id, which must read as
  // a failure, not as a partial success.
  const verb = { restore: proviant.t("js.feedback.bulkVerbRestored"), archive: proviant.t("js.feedback.bulkVerbConsumed"), delete: proviant.t("js.feedback.bulkVerbWasted") }[type];
  const finish = (res) => {
    if (res.code !== 200) {
      proviant.showFeedback("error", proviant.t("js.feedback.bulkActionFailed"), res.message || proviant.t("js.feedback.requestFailedWithStatus", res.code), onDone);
    } else if (res.failedIds && res.failedIds.length) {
      proviant.showFeedback("warning", proviant.t("js.feedback.bulkPartialTitle", verb), proviant.t("js.feedback.bulkPartialMessage", res.failedIds.length, ids.length, verb), onDone);
    } else {
      onDone();
    }
  };
  // A rejected fetch (network drop, a rate-limit page that is not JSON) has
  // no status to report, but the user must still learn that the action was
  // not confirmed — and onDone must still run, or the modal stays open over
  // a stale selection.
  const run = async () => {
    try {
      finish(await fn(ids));
    } catch (err) {
      proviant.showFeedback("error", proviant.t("js.feedback.bulkActionFailed"), (err && err.message) || proviant.t("js.feedback.requestFailed"), onDone);
    }
  };
  if (needsConfirm) {
    proviant.showConfirm(
      confirmTitle || (type === 'delete' ? proviant.t("js.feedback.confirmMarkWasted") : type === 'archive' ? proviant.t("js.feedback.confirmMarkConsumed") : proviant.t("common.confirm")),
      confirmMsg || proviant.tn("js.feedback.bulkConfirmCount", ids.length) + (type === 'delete' ? ' ' + proviant.t("js.feedback.bulkConfirmIrreversible") : ''),
      run,
      type === 'delete' ? proviant.t("js.feedback.bulkConfirmBtnWasted") : type === 'archive' ? proviant.t("js.feedback.bulkConfirmBtnConsumed") : proviant.t("js.feedback.bulkConfirmBtnRestore"),
      type === 'delete' ? 'danger' : type === 'archive' ? 'warning' : 'primary'
    );
  } else {
    await run();
  }
};

/* ── Auth / user API ─────────────────────────────────────────────────────────── */
proviant.loginUser = async function (username, password) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/auth/login`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, password }) });
  let body = {}; try { body = await res.json(); } catch (_) { /* non-JSON body stays {} */ }
  return { code: res.status, body: body.message || body.code || "", retryAfter: res.headers.get("Retry-After") };
};

proviant.signupUser = async function (username, mailAddress, password, inviteToken) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/auth/signup`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, mailAddress, password, inviteToken }) });
  const body = await res.json();
  if (!res.ok) return { code: res.status, body: `${body.message}` };
  return { code: res.status, body: body.message };
};

proviant.updateUser = async function (displayName, mailAddress, language) {
  const body = { displayName, mailAddress };
  if (language !== undefined) {
    body.language = language;
  }
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const data = await res.json();
  return { code: res.status, message: data.message };
};

proviant.updateUserPassword = async function (username, password) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/password`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, password }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.updateNotificationSettings = async function (preferences) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/notification-preferences`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(preferences) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.updateReceiptScanSettings = async function (settings) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/receipt-scan-settings`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(settings) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.getDashboardLayout = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/dashboard-layout`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.saveDashboardLayout = async function (layout) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/dashboard-layout`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(layout) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.generateTelegramLinkToken = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/telegram-link-token`, { method: "POST" });
  const body = await res.json();
  return { code: res.status, token: body.token, botUsername: body.botUsername };
};

proviant.getToken = function () {
  const match = document.cookie.match(/(?:^|;\s*)jwt=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : "";
};

proviant.logoutUser = async function () {
  try { await fetch('/auth/logout', { method: 'POST', credentials: 'include' }); }
  catch (e) { console.error('Logout request failed:', e); }
};

/* ── Household API ───────────────────────────────────────────────────────────── */
proviant.updateHouseholdName = async function (name) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/name`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.cancelApplication = async function (applicationID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/applications/${applicationID}`, { method: "DELETE" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.removeMember = async function (userID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/members/${userID}`, { method: "DELETE" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.leaveHousehold = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/household/leave`, { method: "POST" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.createHousehold = async function (name) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/household/create`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.applyForHousehold = async function (householdID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/${householdID}/apply`, { method: "POST" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.approveApplication = async function (applicationID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/applications/${applicationID}/approve`, { method: "POST" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.rejectApplication = async function (applicationID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/applications/${applicationID}/reject`, { method: "POST" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.getHouseholdUsers = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/users`, { method: "GET" });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.updateHouseholdUser = async function (userID, username, mailAddress) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/users/${userID}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, mailAddress }) });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.deleteHouseholdUser = async function (userID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/users/${userID}`, { method: "DELETE" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.resetHouseholdUserPassword = async function (userID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/users/${userID}/reset-password`, { method: "POST" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.updateMemberRole = async function (userID, role) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/members/${userID}/role`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ role }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

/* ── Invitation API ──────────────────────────────────────────────────────────── */
proviant.createInvitation = async function (email) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/invitations`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.getInvitations = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/invitations`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, invitations: body };
};

proviant.cancelInvitation = async function (invitationID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/invitations/${invitationID}`, { method: "DELETE", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.acceptInvitation = async function (token) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/auth/invite/accept`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

/* ── Onboarding API ─────────────────────────────────────────────────────────── */
proviant.getOnboardingState = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/state`, { method: "GET" });
  const body = await res.json();
  return { code: res.status, body };
};

proviant.getOnboardingHouseholds = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/households`, { method: "GET" });
  const body = await res.json();
  return { code: res.status, body };
};

proviant.applyOnboardingHousehold = async function (householdId) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/apply-household`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ householdId }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.completeOnboarding = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/complete`, { method: "POST" });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.updateOnboardingProfile = async function (displayName) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/profile`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ displayName }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.createOnboardingHousehold = async function (name) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/create-household`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.joinOnboardingByInvite = async function (token) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/join-invite`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

/* ── Stats API ─────────────────────────────────────────────────────────────── */
proviant.getProductStats = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/stats`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.getStreak = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/streak`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.getHouseholdSettings = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/settings`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.updateHouseholdSettings = async function (settings) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/settings`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.getSavingsStats = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/savings/stats`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.getWasteAnalytics = async function (period, sort) {
  const params = new URLSearchParams();
  if (period) params.set("period", period);
  if (sort) params.set("sort", sort);
  const q = params.toString() ? `?${params.toString()}` : "";
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/stats/waste${q}`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.getNotifications = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/notifications`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

/* ── Household Activity API ──────────────────────────────────────────────────── */
proviant.getActivityFeed = async function (limit = 10, offset = 0) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/activity?limit=${limit}&offset=${offset}`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

/* ── Personal Access Token API ──────────────────────────────────────────────── */
proviant.createPAT = async function (name, expiresAt) {
  const payload = { name };
  if (expiresAt) payload.expiresAt = expiresAt;
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/tokens`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.getPATs = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/tokens`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.deletePAT = async function (patID) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/tokens/${patID}`, { method: "DELETE", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

/* ── Export API ─────────────────────────────────────────────────────────────── */
proviant.exportProductsCSV = function (from, to) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/export/products.csv`;
  if (from) url += `?from=${from}`;
  if (to) url += `${from ? '&' : '?'}to=${to}`;
  window.location.href = url;
};

proviant.exportProductsJSON = function (from, to) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/export/products.json`;
  if (from) url += `?from=${from}`;
  if (to) url += `${from ? '&' : '?'}to=${to}`;
  window.location.href = url;
};

proviant.exportArchiveCSV = function (from, to) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/export/archive.csv`;
  if (from) url += `?from=${from}`;
  if (to) url += `${from ? '&' : '?'}to=${to}`;
  window.location.href = url;
};

proviant.exportFullJSON = function () {
  window.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/export/full.json`;
};

/* ── Calendar API ───────────────────────────────────────────────────────────── */
proviant.getCalendarTokenStatus = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/token`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.createCalendarToken = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/token`, { method: "POST", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.deleteCalendarToken = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/token`, { method: "DELETE", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.rotateCalendarToken = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/token/rotate`, { method: "POST", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.downloadCalendarICS = function (token) {
  window.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/export.ics?token=${encodeURIComponent(token)}`;
};

/* ── Webhook API ────────────────────────────────────────────────────────────── */
proviant.getWebhooks = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, webhooks: body.webhooks };
};

proviant.createWebhook = async function (url, secret, events, active) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ url, secret, events, active }) });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.updateWebhook = async function (id, url, secret, events, active) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks/${id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ url, secret, events, active }) });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.deleteWebhook = async function (id) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks/${id}`, { method: "DELETE", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body };
};

proviant.getWebhookDeliveries = async function (id) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks/${id}/deliveries`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, deliveries: body.deliveries };
};

/* ── Web Push Notification API ───────────────────────────────────────────────── */
proviant.getWebPushVAPIDPublicKey = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/notifications/push/vapidPublicKey`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, publicKey: body.publicKey };
};

proviant.subscribeWebPush = async function (subscription) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/notifications/push/subscribe`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(subscription) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.unsubscribeWebPush = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/notifications/push/subscribe`, { method: "DELETE", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

/* ── Audit log API ──────────────────────────────────────────────────────────── */
proviant.getAuditLogs = async function (date) {
  const params = date ? `?date=${encodeURIComponent(date)}` : "";
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/audit-log${params}`, { method: "GET" });
  const body = await res.json();
  return { code: res.status, message: body };
};
/* ── Saved recipes API ───────────────────────────────────────────────────────── */
proviant.getRecipes = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/recipes`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, recipes: Array.isArray(body) ? body : [], message: body.message };
};

proviant.getRecipe = async function (id) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/recipes/${id}`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, recipe: body, message: body.message };
};

proviant.createRecipe = async function (recipe) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/recipes`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(recipe) });
  const body = await res.json();
  return { code: res.status, recipe: body, message: body.message };
};

proviant.updateRecipe = async function (id, recipe) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/recipes/${id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(recipe) });
  const body = await res.json();
  return { code: res.status, recipe: body, message: body.message };
};

proviant.deleteRecipe = async function (id) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/recipes/${id}`, { method: "DELETE", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.cookRecipe = async function (id, servings) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/recipes/${id}/cook`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ servings: servings || 0 }) });
  const body = await res.json();
  return { code: res.status, consumed: body.consumed, partial: body.partial, errors: body.errors || [], message: body.message };
};

/* ── Meal plan API ───────────────────────────────────────────────────────────── */
proviant.getMealPlans = async function (from, to) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/mealplans?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, entries: Array.isArray(body) ? body : [], message: body.message };
};

proviant.createMealPlan = async function (entry) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/mealplans`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(entry) });
  const body = await res.json();
  return { code: res.status, entry: body, message: body.message };
};

proviant.updateMealPlan = async function (id, entry) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/mealplans/${id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(entry) });
  const body = await res.json();
  return { code: res.status, entry: body, message: body.message };
};

proviant.deleteMealPlan = async function (id) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/mealplans/${id}`, { method: "DELETE", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

proviant.generateShoppingListFromMealPlan = async function (from, to) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/mealplans/shopping-list`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ from, to }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
};

/* ── Product lookup for recipe ingredients ───────────────────────────────────── */
proviant.getProducts = async function () {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products`, { method: "GET", headers: { "Content-Type": "application/json" } });
  const body = await res.json();
  return { code: res.status, products: Array.isArray(body) ? body : [], message: body.message };
};
