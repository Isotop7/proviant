const proviant = {};

// Read the csrf_token cookie set by the server's CSRF middleware.
proviant._getCsrfToken = function () {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
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
let feedbackModalHidden = null;
let deferredFeedbackCloses = [];

function runDeferredFeedbackCloses() {
  const pending = deferredFeedbackCloses;
  deferredFeedbackCloses = [];
  pending.forEach(function (fn) { fn(); });
}

proviant.showFeedback = function (type, title, message, onClose) {
  const modal = document.getElementById("proviantFeedbackModal");
  if (!modal) return;
  const iconEl = document.getElementById("proviantFeedbackIcon");
  const titleEl = document.getElementById("proviantFeedbackTitle");
  const msgEl = document.getElementById("proviantFeedbackMessage");
  const configs = {
    success: { icon: "bi-check-circle-fill",       boxBg: "oklch(0.94 0.04 145)", boxColor: "oklch(0.40 0.10 145)" },
    error:   { icon: "bi-x-circle-fill",           boxBg: "oklch(0.95 0.05 25)",  boxColor: "oklch(0.45 0.20 25)"  },
    warning: { icon: "bi-exclamation-triangle-fill", boxBg: "oklch(0.96 0.06 80)", boxColor: "oklch(0.55 0.18 75)"  },
    info:    { icon: "bi-info-circle-fill",         boxBg: "oklch(0.94 0.03 230)", boxColor: "oklch(0.45 0.12 230)" },
  };
  const cfg = configs[type] || configs.info;
  const boxEl = document.getElementById("proviantFeedbackIconBox");
  if (iconEl) iconEl.className = "bi " + cfg.icon;
  if (boxEl) { boxEl.style.background = cfg.boxBg; boxEl.style.color = cfg.boxColor; }
  if (titleEl) titleEl.textContent = title || "";
  if (msgEl) msgEl.textContent = message || "";
  // Wire onClose to any dismissal path (OK button, ESC, backdrop) via a
  // one-shot hidden listener — the OK button only carries data-bs-dismiss,
  // so a btn.onclick assignment would fire onClose twice on click and
  // never on ESC/backdrop. The handler removes itself when it fires.
  // A still-pending onClose replaced by a newer toast is deferred instead
  // of dropped or fired synchronously: firing it here would run a reload
  // mid-render and kill the replacement toast before it is visible. The
  // deferred callback runs after the replacement toast is dismissed
  // (chained after a new onClose), so a state-refreshing reload is never
  // silently lost and never swallows the currently shown message.
  if (feedbackModalHidden) {
    modal.removeEventListener("hidden.bs.modal", feedbackModalHidden);
    deferredFeedbackCloses.push(feedbackModalHidden);
    feedbackModalHidden = null;
  }
  if (onClose) {
    const handler = function () {
      modal.removeEventListener("hidden.bs.modal", handler);
      if (feedbackModalHidden === handler) feedbackModalHidden = null;
      onClose();
      runDeferredFeedbackCloses();
    };
    feedbackModalHidden = handler;
    modal.addEventListener("hidden.bs.modal", handler);
  } else if (deferredFeedbackCloses.length) {
    const deferredHandler = function () {
      modal.removeEventListener("hidden.bs.modal", deferredHandler);
      runDeferredFeedbackCloses();
    };
    modal.addEventListener("hidden.bs.modal", deferredHandler);
  }
  bootstrap.Modal.getOrCreateInstance(modal).show();
};

proviant.showConfirm = function (title, message, onConfirm, confirmLabel, confirmType) {
  const modal = document.getElementById("proviantConfirmModal");
  if (!modal) return;
  const titleEl = document.getElementById("proviantConfirmTitle");
  const msgEl = document.getElementById("proviantConfirmMessage");
  const btnEl = document.getElementById("proviantConfirmBtn");
  if (titleEl) titleEl.textContent = title || "Are you sure?";
  if (msgEl) msgEl.textContent = message || "";
  if (btnEl) {
    btnEl.textContent = confirmLabel || "Confirm";
    btnEl.className = "btn px-4 btn-" + (confirmType || "danger");
    btnEl.onclick = function () { bootstrap.Modal.getInstance(modal).hide(); onConfirm(); };
  }
  bootstrap.Modal.getOrCreateInstance(modal).show();
};

proviant.copyToClipboard = async function (text) {
  try { await navigator.clipboard.writeText(text); return true; } catch { return false; }
};

/* ── Utility helpers ────────────────────────────────────────────────────────── */
proviant.formatDate = function (timestamp) {
  const date = new Date(timestamp);
  const pad = (n) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth()+1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
};

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
proviant._offlineQueue = JSON.parse(localStorage.getItem('proviant_offline_queue') || '[]');

proviant._saveQueue = function () {
  localStorage.setItem('proviant_offline_queue', JSON.stringify(proviant._offlineQueue));
};

proviant._enqueueAmountDelta = function (productID, delta) {
  const existing = proviant._offlineQueue.find(op => op.productID === productID);
  if (existing) {
    existing.delta += delta;
    if (existing.delta === 0) {
      proviant._offlineQueue = proviant._offlineQueue.filter(op => op.productID !== productID);
    }
  } else {
    proviant._offlineQueue.push({ productID, delta, ts: Date.now() });
  }
  proviant._saveQueue();
  proviant._updateOfflineBadge();
};

proviant._updateOfflineBadge = function () {
  const el = document.getElementById('offlineQueueCount');
  if (!el) return;
  const n = proviant._offlineQueue.length;
  if (n > 0) { el.textContent = `(${n} pending)`; el.classList.remove('d-none'); }
  else { el.classList.add('d-none'); }
};

proviant._flushQueue = async function () {
  if (!navigator.onLine || proviant._offlineQueue.length === 0) return;
  const queue = proviant._offlineQueue.slice();
  proviant._offlineQueue = [];
  proviant._saveQueue();
  proviant._updateOfflineBadge();
  for (const op of queue) {
    try { await proviant._sendAmountDelta(op.productID, op.delta); }
    catch (_) { proviant._enqueueAmountDelta(op.productID, op.delta); }
  }
};

proviant._sendAmountDelta = async function (productID, delta) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}/amount`;
  const res = await fetch(url, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ delta }) });
  const body = await res.json();
  return { code: res.status, message: body.message, deleted: res.status === 200 && typeof body.message === "string" && body.message.includes("deleted") };
};

proviant.updateProductAmount = async function (productID, delta) {
  if (!navigator.onLine) { proviant._enqueueAmountDelta(productID, delta); return { code: 200, message: 'queued', deleted: false, queued: true }; }
  return proviant._sendAmountDelta(productID, delta);
};

/* ── Product API ─────────────────────────────────────────────────────────────── */
proviant.createProduct = async function (barcode, expireAt, amount, storageLocationId, isPrivate = false) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products`;
  const res = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ barcode, expireAt, amount: amount || 1, storageLocationId: storageLocationId || null, isPrivate }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
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
  try { body = await res.json(); } catch (_) {}
  return { code: res.status, product: body, conflict: res.status === 409, openedAt: body.openedAt };
};

proviant.addToShoppingList = async function (productId, quantity, unit) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/shopping-list`;
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      productId: parseInt(productId, 10),
      quantity: quantity || 1,
      unit: unit || "",
    }),
  });
  if (!res.ok) {
    const body = await res.json().catch(function () { return {}; });
    throw new Error(body.message || "Could not add to shopping list");
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
    proviant.showFeedback('success', 'Added', successMessage);
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
  const verb = { restore: "restored", archive: "marked as consumed", delete: "marked as wasted" }[type];
  const finish = (res) => {
    if (res.code !== 200) {
      proviant.showFeedback("error", "Bulk action failed", res.message || `Request failed with status ${res.code}`, onDone);
    } else if (res.failedIds && res.failedIds.length) {
      proviant.showFeedback("warning", `Not all products were ${verb}`, `${res.failedIds.length} of ${ids.length} products were not ${verb} (not found or no access).`, onDone);
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
      proviant.showFeedback("error", "Bulk action failed", (err && err.message) || "Request failed", onDone);
    }
  };
  if (needsConfirm) {
    proviant.showConfirm(
      confirmTitle || (type === 'delete' ? 'Mark as wasted' : type === 'archive' ? 'Mark as consumed' : 'Confirm'),
      confirmMsg || `${ids.length} product${ids.length !== 1 ? 's' : ''}?${type === 'delete' ? ' This cannot be undone.' : ''}`,
      run,
      type === 'delete' ? 'Wasted' : type === 'archive' ? 'Consumed' : 'Restore',
      type === 'delete' ? 'danger' : type === 'archive' ? 'warning' : 'primary'
    );
  } else {
    await run();
  }
};

/* ── Auth / user API ─────────────────────────────────────────────────────────── */
proviant.loginUser = async function (username, password) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/auth/login`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, password }) });
  let body = {}; try { body = await res.json(); } catch (_) {}
  return { code: res.status, body: body.message || body.code || "", retryAfter: res.headers.get("Retry-After") };
};

proviant.signupUser = async function (username, mailAddress, password, inviteToken) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/auth/signup`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username, mailAddress, password, inviteToken }) });
  const body = await res.json();
  if (!res.ok) return { code: res.status, body: `${body.message}` };
  return { code: res.status, body: body.message };
};

proviant.updateUser = async function (displayName, mailAddress) {
  const res = await fetch(`${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ displayName, mailAddress }) });
  const body = await res.json();
  return { code: res.status, message: body.message };
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