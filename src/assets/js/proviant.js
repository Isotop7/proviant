const proviant = {};

// Define a public method
proviant.debug = function () {
  console.log("Proviant loaded");
};

/* Unified feedback modal — use for all page-level async success/error results */
proviant.showFeedback = function (type, title, message, onClose) {
  const modal = document.getElementById("proviantFeedbackModal");
  if (!modal) return;

  const iconEl = document.getElementById("proviantFeedbackIcon");
  const titleEl = document.getElementById("proviantFeedbackTitle");
  const msgEl = document.getElementById("proviantFeedbackMessage");
  const btnEl = document.getElementById("proviantFeedbackBtn");

  const configs = {
    success: { icon: "bi-check-circle-fill", color: "text-success" },
    error:   { icon: "bi-x-circle-fill",     color: "text-danger"  },
    warning: { icon: "bi-exclamation-circle-fill", color: "text-warning" },
    info:    { icon: "bi-info-circle-fill",   color: "text-secondary" },
  };
  const cfg = configs[type] || configs.info;

  if (iconEl) iconEl.className = "bi " + cfg.icon + " " + cfg.color + " fs-1 mb-3 d-block";
  if (titleEl) titleEl.textContent = title || "";
  if (msgEl) msgEl.textContent = message || "";
  if (btnEl) btnEl.onclick = onClose || null;

  bootstrap.Modal.getOrCreateInstance(modal).show();
};

/* Confirmation modal — destructive actions requiring user decision */
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
    btnEl.onclick = function () {
      bootstrap.Modal.getInstance(modal).hide();
      onConfirm();
    };
  }

  bootstrap.Modal.getOrCreateInstance(modal).show();
};

proviant.createProduct = async function (barcode, expireAt, amount) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products`;
  let data = JSON.stringify({ barcode, expireAt, amount: amount || 1 });
  const apiCall = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: data,
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.getOpenFoodFactsData = async function (barcode) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/openfoodfacts/${barcode}`;
  const apiCall = await fetch(url, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body,
  };
  return response;
};

proviant.getProductsByBarcode = async function (barcode) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/byBarcode/${barcode}`;
  const apiCall = await fetch(url, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body,
  };
  return response;
};

proviant.editProduct = async function (product) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${product.ID}`;
  let data = JSON.stringify(product);
  const apiCall = await fetch(url, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: data,
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

// Offline edit queue — persists amount-delta ops in localStorage for sync on reconnect
proviant._offlineQueue = JSON.parse(localStorage.getItem('proviant_offline_queue') || '[]');

proviant._saveQueue = function () {
  localStorage.setItem('proviant_offline_queue', JSON.stringify(proviant._offlineQueue));
};

proviant._enqueueAmountDelta = function (productID, delta) {
  const existing = proviant._offlineQueue.find(function (op) { return op.productID === productID; });
  if (existing) {
    existing.delta += delta;
    if (existing.delta === 0) {
      proviant._offlineQueue = proviant._offlineQueue.filter(function (op) { return op.productID !== productID; });
    }
  } else {
    proviant._offlineQueue.push({ productID: productID, delta: delta, ts: Date.now() });
  }
  proviant._saveQueue();
  proviant._updateOfflineBadge();
};

proviant._updateOfflineBadge = function () {
  var countEl = document.getElementById('offlineQueueCount');
  if (!countEl) return;
  var n = proviant._offlineQueue.length;
  if (n > 0) {
    countEl.textContent = '(' + n + ' pending)';
    countEl.classList.remove('d-none');
  } else {
    countEl.classList.add('d-none');
  }
};

proviant._flushQueue = async function () {
  if (!navigator.onLine || proviant._offlineQueue.length === 0) return;
  var queue = proviant._offlineQueue.slice();
  proviant._offlineQueue = [];
  proviant._saveQueue();
  proviant._updateOfflineBadge();
  for (var i = 0; i < queue.length; i++) {
    var op = queue[i];
    try {
      await proviant._sendAmountDelta(op.productID, op.delta);
    } catch (_e) {
      proviant._enqueueAmountDelta(op.productID, op.delta);
    }
  }
};

proviant._sendAmountDelta = async function (productID, delta) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}/amount`;
  const apiCall = await fetch(url, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ delta: delta }),
  });
  const body = await apiCall.json();
  return {
    code: apiCall.status,
    message: body.message,
    deleted: apiCall.status === 200 && typeof body.message === "string" && body.message.includes("deleted"),
  };
};

proviant.updateProductAmount = async function (productID, delta) {
  if (!navigator.onLine) {
    proviant._enqueueAmountDelta(productID, delta);
    return { code: 200, message: 'queued', deleted: false, queued: true };
  }
  return proviant._sendAmountDelta(productID, delta);
};

proviant.deleteProduct = async function (productID, archiveOnly) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}`;
  if (archiveOnly) {
    url += "?archiveOnly=true";
  }
  const apiCall = await fetch(url, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.bulkDeleteProducts = async function (productIDs) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulkDelete`;
  const apiCall = await fetch(url, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ productIDs }),
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.bulkArchiveProducts = async function (productIDs) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulkArchive`;
  const apiCall = await fetch(url, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ productIDs }),
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.restoreProduct = async function (productID) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/${productID}/restore`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.bulkRestoreProducts = async function (productIDs) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/bulkRestore`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ productIDs }),
  });
  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.loginUser = async function (username, password) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/auth/login`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ username, password }),
  });
  let body = {};
  try {
    body = await apiCall.json();
  } catch (_) {
    // empty or non-JSON response
  }
  return {
    code: apiCall.status,
    body: body.message || body.code || "",
    retryAfter: apiCall.headers.get("Retry-After"),
  };
};

proviant.signupUser = async function (username, mailAddress, password, inviteToken) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/auth/signup`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ username, mailAddress, password, inviteToken }),
  });
  const body = await apiCall.json();
  const response = {
    code: apiCall.status,
    body: body.message,
  };

  if (!apiCall.ok) {
    return {
      code: apiCall.status,
      body: `${body.message}`,
    };
  }
  return response;
};

proviant.updateUser = async function (displayName, mailAddress) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user`;
  const apiCall = await fetch(url, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ displayName, mailAddress }),
  });

  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.updateUserPassword = async function (username, password) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/password`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ username, password }),
  });

  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.updateNotificationSettings = async function (preferences) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/notification-preferences`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(preferences),
  });

  const body = await apiCall.json();
  let response = {
    code: apiCall.status,
    message: body.message,
  };
  return response;
};

proviant.updateHouseholdName = async function (name) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/name`;
  const apiCall = await fetch(url, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.cancelApplication = async function (applicationID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/applications/${applicationID}`;
  const apiCall = await fetch(url, { method: "DELETE" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.removeMember = async function (userID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/members/${userID}`;
  const apiCall = await fetch(url, { method: "DELETE" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.leaveHousehold = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/household/leave`;
  const apiCall = await fetch(url, { method: "POST" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.createHousehold = async function (name) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/household/create`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.applyForHousehold = async function (householdID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/${householdID}/apply`;
  const apiCall = await fetch(url, { method: "POST" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.approveApplication = async function (applicationID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/applications/${applicationID}/approve`;
  const apiCall = await fetch(url, { method: "POST" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.rejectApplication = async function (applicationID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/applications/${applicationID}/reject`;
  const apiCall = await fetch(url, { method: "POST" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.getHouseholdUsers = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/users`;
  const apiCall = await fetch(url, { method: "GET" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.updateHouseholdUser = async function (userID, username, mailAddress) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/users/${userID}`;
  const apiCall = await fetch(url, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, mailAddress }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.deleteHouseholdUser = async function (userID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/users/${userID}`;
  const apiCall = await fetch(url, { method: "DELETE" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.resetHouseholdUserPassword = async function (userID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/admin/users/${userID}/reset-password`;
  const apiCall = await fetch(url, { method: "POST" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

// Helper function to get JWT token from cookie
proviant.getToken = function () {
  const name = "jwt=";
  const decodedCookie = decodeURIComponent(document.cookie);
  const ca = decodedCookie.split(';');
  // Iterate over each cookie entry
  for (let i = 0; i < ca.length; i++) {
    let c = ca[i];
    // Trim leading whitespace from cookie string
    while (c.charAt(0) == ' ') {
      c = c.substring(1);
    }
    // Check if this cookie starts with the target name (e.g., "jwt=")
    if (c.indexOf(name) == 0) {
      // Extract and return the cookie value (everything after name)
      return c.substring(name.length, c.length);
    }
  }
  return "";
};

proviant.logoutUser = async function () {
  // Get the current JWT token from cookie
  const token = proviant.getToken();
  if (token) {
    try {
      // Call logout endpoint to revoke the token
      await fetch('/auth/logout', {
        method: 'POST',
        credentials: 'include'
      });
    } catch (error) {
      console.error('Logout request failed:', error);
      // Continue with cookie expiration even if API call fails
    }
  }
  // Expire the JWT cookie
  document.cookie = "jwt=; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT;";
};

proviant.formatDate = function (timestamp) {
  const date = new Date(timestamp);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0"); // Months are 0-based
  const day = String(date.getDate()).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  const seconds = String(date.getSeconds()).padStart(2, "0");

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

// Render categories
proviant.badgifyCategories = function (categories, limit) {
  let output = "";
  // Split categories
  const categoriesArray = categories.split(",");
  for (let index = 0; index < categoriesArray.length; index++) {
    // Get element and split
    const category = categoriesArray[index].trim();
    const contents = category.split(":");

    // early return
    if (index == limit) {
      break;
    }

    // Check if language was found
    if (contents.length == 2) {
      const lang = contents[0].trim();
      const definition = contents[1].trim();
      output += `<span class="badge bg-dark me-3">${lang}</span>${definition}</br>`;
    } else {
      output += `${category}</br>`;
    }
  }
  return output;
};

// Render expire at
proviant.colorExpiry = function (date) {
  if (new Date(date) < Date.now()) {
    return "bg-danger";
  } else {
    return "bg-primary";
  }
};

/* Invitation API methods */
proviant.createInvitation = async function (email) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/invitations`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.getInvitations = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/invitations`;
  const apiCall = await fetch(url, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, invitations: body };
};

proviant.cancelInvitation = async function (invitationID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/household/invitations/${invitationID}`;
  const apiCall = await fetch(url, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.acceptInvitation = async function (token) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/auth/invite/accept`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

/* Onboarding API methods */
proviant.getOnboardingState = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/state`;
  const apiCall = await fetch(url, { method: "GET" });
  const body = await apiCall.json();
  return { code: apiCall.status, body };
};

proviant.getOnboardingHouseholds = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/households`;
  const apiCall = await fetch(url, { method: "GET" });
  const body = await apiCall.json();
  return { code: apiCall.status, body };
};

proviant.applyOnboardingHousehold = async function (householdId) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/apply-household`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ householdId }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.completeOnboarding = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/complete`;
  const apiCall = await fetch(url, { method: "POST" });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.updateOnboardingProfile = async function (displayName) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/profile`;
  const apiCall = await fetch(url, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ displayName }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.createOnboardingHousehold = async function (name) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/create-household`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.joinOnboardingByInvite = async function (token) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/onboarding/join-invite`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body.message };
};

proviant.getProductStats = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/stats`;
  const apiCall = await fetch(url, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.getNotifications = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/notifications`;
  const apiCall = await fetch(url, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.createPAT = async function (name, expiresAt) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/tokens`;
  const payload = { name };
  if (expiresAt) {
    payload.expiresAt = expiresAt;
  }
  const apiCall = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.getPATs = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/tokens`;
  const apiCall = await fetch(url, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.deletePAT = async function (patID) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user/tokens/${patID}`;
  const apiCall = await fetch(url, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.exportProductsCSV = function (from, to) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/export/products.csv`;
  const params = [];
  if (from) params.push(`from=${from}`);
  if (to) params.push(`to=${to}`);
  if (params.length) url += `?${params.join("&")}`;
  window.location.href = url;
};

proviant.exportProductsJSON = function (from, to) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/export/products.json`;
  const params = [];
  if (from) params.push(`from=${from}`);
  if (to) params.push(`to=${to}`);
  if (params.length) url += `?${params.join("&")}`;
  window.location.href = url;
};

proviant.exportArchiveCSV = function (from, to) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/export/archive.csv`;
  const params = [];
  if (from) params.push(`from=${from}`);
  if (to) params.push(`to=${to}`);
  if (params.length) url += `?${params.join("&")}`;
  window.location.href = url;
};

proviant.exportFullJSON = function () {
  window.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/export/full.json`;
};

/* Calendar sync API methods */
proviant.getCalendarTokenStatus = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/token`;
  const apiCall = await fetch(url, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.createCalendarToken = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/token`;
  const apiCall = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.deleteCalendarToken = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/token`;
  const apiCall = await fetch(url, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.downloadCalendarICS = function (token) {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/calendar/export.ics?token=${encodeURIComponent(token)}`;
  window.location.href = url;
};

proviant.copyToClipboard = async function (text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
};

/* Webhook API methods */
proviant.getWebhooks = async function () {
  const url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks`;
  const apiCall = await fetch(url, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, webhooks: body.webhooks };
};

proviant.createWebhook = async function (url, secret, events, active) {
  const apiUrl = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks`;
  const apiCall = await fetch(apiUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, secret, events, active }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.updateWebhook = async function (id, url, secret, events, active) {
  const apiUrl = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks/${id}`;
  const apiCall = await fetch(apiUrl, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, secret, events, active }),
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.deleteWebhook = async function (id) {
  const apiUrl = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks/${id}`;
  const apiCall = await fetch(apiUrl, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, message: body };
};

proviant.getWebhookDeliveries = async function (id) {
  const apiUrl = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/webhooks/${id}/deliveries`;
  const apiCall = await fetch(apiUrl, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  const body = await apiCall.json();
  return { code: apiCall.status, deliveries: body.deliveries };
};
