const proviant = {};

// Define a public method
proviant.debug = function () {
  console.log("Proviant loaded");
};

proviant.createProduct = async function (barcode, expireAt) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products`;
  let data = JSON.stringify({ barcode, expireAt });
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
  if (!apiCall.ok) {
    return {
      code: apiCall.status,
      body: "Error logging in",
    };
  }
  const body = await apiCall.json();
  const response = {
    code: apiCall.status,
    body: body.message,
  };
  return response;
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

proviant.updateUser = async function (username, mailAddress) {
  let url = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/user`;
  const apiCall = await fetch(url, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ username, mailAddress }),
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

proviant.logoutUser = function () {
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
