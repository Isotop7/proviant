/* ── Button loading helper ────────────────────────────────────────── */
function setButtonLoading(btn, loading) {
  if (!btn) return;
  if (loading) {
    btn.disabled = true;
    btn.dataset.originalHtml = btn.innerHTML;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span>Saving…';
  } else {
    btn.disabled = false;
    btn.innerHTML = btn.dataset.originalHtml || btn.innerHTML;
  }
}

function ShowSuccessModal(message, btnFunction) {
  proviant.showFeedback('success', 'Done', message, btnFunction);
}

/* ── Update personal details ─────────────────────────────────────── */
function UpdateSettings() {
  let formIsValid = true;
  const inputUsername = document.getElementById("inputUsername");
  const inputMailAddress = document.getElementById("inputMailAddress");
  if (!inputUsername || !inputMailAddress) return;

  if (!inputUsername.value) {
    inputUsername.classList.add("is-invalid");
    formIsValid = false;
  }
  if (!inputMailAddress.value) {
    inputMailAddress.classList.add("is-invalid");
    formIsValid = false;
  }
  if (!formIsValid) return;

  const btn = document.getElementById("btnUpdatePersonalDetails");
  setButtonLoading(btn, true);

  proviant.updateUser(inputUsername.value, inputMailAddress.value).then((response) => {
    setButtonLoading(btn, false);
    switch (response.code) {
      case 200:
        ShowSuccessModal(
          "User update complete. Please reload page to show changed values.",
          function (event) {
            event.preventDefault();
            location.reload();
          },
        );
        break;
      case 401:
        proviant.showFeedback('error', 'Error', `Update failed: ${response.message}`);
        break;
      default:
        proviant.showFeedback('error', 'Error', `Update failed: ${response.message}`);
        break;
    }
  });
}

/* ── Update password ─────────────────────────────────────────────── */
function UpdatePassword() {
  let formIsValid = true;
  const inputPassword = document.getElementById("inputPassword");
  const inputPasswordVerification = document.getElementById("inputPasswordVerification");
  const inputUsername = document.getElementById("inputUsername");
  if (!inputPassword || !inputPasswordVerification || !inputUsername) return;

  const password = inputPassword.value;
  const passwordVerification = inputPasswordVerification.value;
  const username = inputUsername.value;

  if (!username) {
    inputUsername.classList.add("is-invalid");
    formIsValid = false;
  }
  if (!password) {
    inputPassword.classList.add("is-invalid");
    formIsValid = false;
  }
  if (password && password.length < 12) {
    inputPassword.classList.add("is-invalid");
    formIsValid = false;
  }
  if (!passwordVerification) {
    inputPasswordVerification.classList.add("is-invalid");
    formIsValid = false;
  }
  if (password && passwordVerification && password !== passwordVerification) {
    inputPassword.classList.add("is-invalid");
    inputPasswordVerification.classList.add("is-invalid");
    formIsValid = false;
  }
  if (!formIsValid) return;

  const btn = document.getElementById("btnUpdatePassword");
  setButtonLoading(btn, true);

  proviant.updateUserPassword(username, password).then((response) => {
    setButtonLoading(btn, false);
    switch (response.code) {
      case 200:
        ShowSuccessModal(
          "Password update complete. Please log out to use changed password.",
          function (event) {
            event.preventDefault();
            proviant.logoutUser();
            location.reload();
          },
        );
        break;
      case 401:
        proviant.showFeedback('error', 'Error', `Update failed: ${response.message}`);
        break;
      default:
        proviant.showFeedback('error', 'Error', `Update failed: ${response.message}`);
        break;
    }
  });
}

/* ── Notification settings ───────────────────────────────────────── */
function toggleNtfySettings() {
  const toggleNtfyNotifications = document.getElementById("toggleNtfyNotifications");
  const ntfySettings = document.getElementById("ntfySettings");
  if (ntfySettings && toggleNtfyNotifications) {
    ntfySettings.classList.toggle("d-none", !toggleNtfyNotifications.checked);
  }
}

function UpdateNotificationSettings() {
  const toggleEmailNotifications = document.getElementById("toggleEmailNotifications");
  const toggleNtfyNotifications = document.getElementById("toggleNtfyNotifications");
  const inputNtfyUrl = document.getElementById("inputNtfyUrl");
  const inputNtfyTopic = document.getElementById("inputNtfyTopic");
  const inputNtfyToken = document.getElementById("inputNtfyToken");
  const inputNotificationThreshold = document.getElementById("inputNotificationThreshold");
  if (!toggleEmailNotifications || !toggleNtfyNotifications || !inputNtfyUrl || !inputNtfyTopic || !inputNtfyToken) return;

  // Field-level validation
  if (toggleNtfyNotifications.checked) {
    if (!inputNtfyUrl.value) {
      inputNtfyUrl.classList.add("is-invalid");
      return;
    }
    if (!inputNtfyTopic.value) {
      inputNtfyTopic.classList.add("is-invalid");
      return;
    }
  }

  const thresholdDays = inputNotificationThreshold ? parseInt(inputNotificationThreshold.value, 10) : 0;
  if (isNaN(thresholdDays) || thresholdDays < 0) {
    if (inputNotificationThreshold) inputNotificationThreshold.classList.add("is-invalid");
    return;
  }

  const btn = document.getElementById("btnUpdateNotificationSettings");
  setButtonLoading(btn, true);

  const notificationData = {
    emailEnabled: toggleEmailNotifications.checked,
    ntfyEnabled: toggleNtfyNotifications.checked,
    ntfyUrl: inputNtfyUrl.value || "",
    ntfyTopic: inputNtfyTopic.value || "",
    ntfyToken: inputNtfyToken.value || "",
    notificationThresholdDays: thresholdDays,
  };

  proviant
    .updateNotificationSettings(notificationData)
    .then((response) => {
      setButtonLoading(btn, false);
      switch (response.code) {
        case 200:
          ShowSuccessModal(
            "Notification settings updated successfully.",
            function (event) {
              event.preventDefault();
              location.reload();
            },
          );
          break;
        case 400:
          proviant.showFeedback('error', 'Error', `Update failed: ${response.message}`);
          break;
        case 401:
          proviant.showFeedback('error', 'Unauthorized', response.message);
          break;
        default:
          proviant.showFeedback('error', 'Error', `Error updating notification settings: ${response.message}`);
          break;
      }
    })
    .catch((error) => {
      setButtonLoading(btn, false);
      proviant.showFeedback('error', 'Error', `Network error: ${error.message}`);
    });
}

/* ── Household management helpers ────────────────────────────────── */
function showHouseholdAlert(elementId, message, isSuccess) {
  const el = document.getElementById(elementId);
  if (!el) return;
  el.className = `alert fade mt-3 ${isSuccess ? "alert-success" : "alert-danger"}`;
  const span = el.querySelector("span") || el;
  span.textContent = message;
  el.classList.remove("d-none");
  el.classList.add("show");
}

/* Leave household */
function handleLeaveHousehold() {
  proviant.showConfirm(
    'Leave Household',
    'Leave your current household? You will be assigned a new personal household.',
    function () {
      proviant.leaveHousehold().then((response) => {
        if (response.code === 200) {
          ShowSuccessModal("You have left the household. Reloading page.", function (e) {
            e.preventDefault();
            location.reload();
          });
        } else {
          showHouseholdAlert("leaveHouseholdAlert", `Error: ${response.message}`, false);
        }
      });
    },
    'Leave',
    'danger'
  );
}

/* Create new household */
function handleCreateHousehold() {
  const nameInput = document.getElementById("inputNewHouseholdName");
  const name = nameInput ? nameInput.value.trim() : "";
  if (!name) {
    if (nameInput) nameInput.classList.add("is-invalid");
    return;
  }
  if (nameInput) nameInput.classList.remove("is-invalid");
  proviant.createHousehold(name).then((response) => {
    if (response.code === 200) {
      ShowSuccessModal("Household created. Reloading page.", function (e) {
        e.preventDefault();
        location.reload();
      });
    } else {
      showHouseholdAlert("createHouseholdAlert", `Error: ${response.message}`, false);
    }
  });
}

/* Apply to join household */
function handleApplyHousehold() {
  const idInput = document.getElementById("inputApplyHouseholdID");
  const householdID = idInput ? parseInt(idInput.value, 10) : NaN;
  if (!householdID || householdID < 1) {
    if (idInput) idInput.classList.add("is-invalid");
    return;
  }
  if (idInput) idInput.classList.remove("is-invalid");
  proviant.applyForHousehold(householdID).then((response) => {
    if (response.code === 200) {
      location.reload();
    } else if (response.code === 409) {
      showHouseholdAlert("applyHouseholdAlert", "You already have a pending application for this household.", false);
    } else if (response.code === 404) {
      showHouseholdAlert("applyHouseholdAlert", "Household not found.", false);
    } else {
      showHouseholdAlert("applyHouseholdAlert", `Error: ${response.message}`, false);
    }
  });
}

/* Approve application */
function handleApproveApplication(btn) {
  const applicationID = btn.dataset.id;
  proviant.approveApplication(applicationID).then((response) => {
    if (response.code === 200) {
      const row = document.getElementById(`application-${applicationID}`);
      if (row) row.remove();
    } else {
      showHouseholdAlert("applicationsAlert", `Error: ${response.message}`, false);
    }
  });
}

/* Reject application */
function handleRejectApplication(btn) {
  const applicationID = btn.dataset.id;
  proviant.rejectApplication(applicationID).then((response) => {
    if (response.code === 200) {
      const row = document.getElementById(`application-${applicationID}`);
      if (row) row.remove();
    } else {
      showHouseholdAlert("applicationsAlert", `Error: ${response.message}`, false);
    }
  });
}

/* Rename household */
function handleUpdateHouseholdName() {
  const nameInput = document.getElementById("inputHouseholdName");
  const name = nameInput ? nameInput.value.trim() : "";
  if (!name) {
    if (nameInput) nameInput.classList.add("is-invalid");
    return;
  }
  if (nameInput) nameInput.classList.remove("is-invalid");
  proviant.updateHouseholdName(name).then((response) => {
    if (response.code === 200) {
      location.reload();
    } else {
      showHouseholdAlert("updateNameAlert", `Error: ${response.message}`, false);
    }
  });
}

/* Cancel own pending application */
function handleCancelApplication(btn) {
  const applicationID = btn.dataset.id;
  proviant.showConfirm('Cancel Application', 'Cancel this pending application to join the household?', function () {
    proviant.cancelApplication(applicationID).then((response) => {
      if (response.code === 200) {
        const row = document.getElementById(`my-application-${applicationID}`);
        if (row) row.remove();
      } else {
        proviant.showFeedback('error', 'Error', response.message || 'Could not cancel application.');
      }
    });
  });
}

/* Remove household member */
function handleRemoveMember(btn) {
  const memberID = btn.dataset.id;
  proviant.showConfirm('Remove Member', 'Remove this member from the household? They will be assigned a new personal household.', function () {
    proviant.removeMember(memberID).then((response) => {
      if (response.code === 200) {
        const row = document.getElementById(`member-${memberID}`);
        if (row) row.remove();
      } else {
        proviant.showFeedback('error', 'Error', response.message || 'Could not remove member.');
      }
    });
  });
}

/* ── Invitation helpers ───────────────────────────────────────────── */
function showInviteAlert(elementId, message, isSuccess) {
  const el = document.getElementById(elementId);
  if (!el) return;
  el.className = `alert fade mt-3 ${isSuccess ? "alert-success" : "alert-danger"}`;
  const span = el.querySelector("span") || el;
  span.textContent = message;
  el.classList.remove("d-none");
  el.classList.add("show");
}

/* Send invitation */
function handleSendInvitation() {
  const emailInput = document.getElementById("inputInviteEmail");
  const email = emailInput ? emailInput.value.trim() : "";
  if (!email) {
    if (emailInput) emailInput.classList.add("is-invalid");
    return;
  }
  if (emailInput) emailInput.classList.remove("is-invalid");
  proviant.createInvitation(email).then((response) => {
    if (response.code === 201) {
      ShowSuccessModal("Invitation sent. Reloading page.", function (e) {
        e.preventDefault();
        location.reload();
      });
    } else if (response.code === 409) {
      showInviteAlert("sendInviteAlert", response.message, false);
    } else if (response.code === 400) {
      showInviteAlert("sendInviteAlert", response.message, false);
    } else {
      showInviteAlert("sendInviteAlert", `Error: ${response.message}`, false);
    }
  });
}

/* ── Admin User Management ─────────────────────────────────────── */
function showAdminUserAlert(message, isSuccess) {
  const el = document.getElementById("adminUserAlert");
  if (!el) return;
  el.className = `alert fade mt-3 ${isSuccess ? "alert-success" : "alert-danger"}`;
  const span = el.querySelector("span") || el;
  span.textContent = message;
  el.classList.remove("d-none");
  el.classList.add("show");
}

function renderAdminUserRow(user, currentUserID) {
  if (!user || !user.username) return "";
  const isSelf = user.id === currentUserID;
  return `
    <li class="list-group-item d-flex justify-content-between align-items-center" id="admin-user-${user.id}">
      <span>
        <i class="bi bi-person me-2"></i>${user.username}
        <span class="text-muted ms-1">&lt;${user.mailAddress || ""}&gt;</span>
        ${isSelf ? '<span class="badge bg-secondary ms-1">You</span>' : ""}
      </span>
      <div class="btn-group btn-group-sm">
        <button type="button" class="btn btn-outline-primary btn-edit-user" data-id="${user.id}" data-username="${user.username}" data-email="${user.mailAddress || ""}" title="Edit user">
          <i class="bi bi-pencil"></i>
        </button>
        <button type="button" class="btn btn-outline-warning btn-reset-password" data-id="${user.id}" title="Reset password">
          <i class="bi bi-key"></i>
        </button>
        ${!isSelf ? `
        <button type="button" class="btn btn-outline-danger btn-delete-user" data-id="${user.id}" title="Delete user">
          <i class="bi bi-trash"></i>
        </button>` : ""}
      </div>
    </li>
  `;
}

function loadAdminUsers() {
  const list = document.getElementById("adminUserList");
  const loading = document.getElementById("adminUserLoading");
  if (!list) return;

  if (loading) loading.classList.remove("d-none");

  proviant.getHouseholdUsers().then((response) => {
    if (loading) loading.classList.add("d-none");
    if (response.code === 200) {
      const users = response.message;
      if (!users || !Array.isArray(users) || users.length === 0) {
        list.innerHTML = '<li class="list-group-item text-center text-muted py-3">No users in household.</li>';
        return;
      }
      const currentUserID = parseInt(document.querySelector('span.badge.bg-primary[ID]')?.textContent?.replace("#", "") || "0", 10);
      const rendered = users.map((u) => renderAdminUserRow(u, currentUserID)).join("");
      if (!rendered) {
        list.innerHTML = '<li class="list-group-item text-center text-muted py-3">No users in household.</li>';
        return;
      }
      list.innerHTML = rendered;
    } else if (response.code === 403) {
      list.innerHTML = '<li class="list-group-item text-danger py-3">Access denied.</li>';
    } else {
      showAdminUserAlert(`Error: ${response.message || "Unknown error"}`, false);
    }
  });
}

function handleRefreshUsers() {
  loadAdminUsers();
}

function handleEditUser(btn) {
  const userID = btn.dataset.id;
  const username = btn.dataset.username;
  const email = btn.dataset.email;
  document.getElementById("editUserID").value = userID;
  document.getElementById("editUsername").value = username;
  document.getElementById("editMailAddress").value = email;
  const modalEl = document.getElementById("editUserModal");
  if (window.bootstrap) {
    const modal = new bootstrap.Modal(modalEl);
    modal.show();
  }
}

function handleSaveUserEdit() {
  const userID = document.getElementById("editUserID").value;
  const username = document.getElementById("editUsername").value.trim();
  const email = document.getElementById("editMailAddress").value.trim();
  let formValid = true;
  const usernameInput = document.getElementById("editUsername");
  const emailInput = document.getElementById("editMailAddress");

  if (!username) {
    usernameInput.classList.add("is-invalid");
    formValid = false;
  } else {
    usernameInput.classList.remove("is-invalid");
  }
  if (!email || !email.includes("@")) {
    emailInput.classList.add("is-invalid");
    formValid = false;
  } else {
    emailInput.classList.remove("is-invalid");
  }
  if (!formValid) return;

  const btn = document.getElementById("btnSaveUserEdit");
  setButtonLoading(btn, true);

  proviant.updateHouseholdUser(userID, username, email).then((response) => {
    setButtonLoading(btn, false);
    if (response.code === 200) {
      const modalEl = document.getElementById("editUserModal");
      if (window.bootstrap) {
        bootstrap.Modal.getInstance(modalEl)?.hide();
      }
      loadAdminUsers();
    } else {
      showAdminUserAlert(`Error: ${response.message}`, false);
    }
  });
}

function handleResetPassword(btn) {
  const userID = btn.dataset.id;
  proviant.showConfirm(
    "Reset Password",
    "Send a password reset email to this user?",
    function () {
      proviant.resetHouseholdUserPassword(userID).then((response) => {
        if (response.code === 200) {
          proviant.showFeedback("success", "Done", "Password reset email sent.");
        } else {
          proviant.showFeedback("error", "Error", response.message || "Could not reset password.");
        }
      });
    },
    "Reset",
    "warning"
  );
}

function handleDeleteUser(btn) {
  const userID = btn.dataset.id;
  proviant.showConfirm(
    "Delete User",
    "Permanently delete this user? This cannot be undone.",
    function () {
      proviant.deleteHouseholdUser(userID).then((response) => {
        if (response.code === 200) {
          const row = document.getElementById(`admin-user-${userID}`);
          if (row) row.remove();
        } else {
          showAdminUserAlert(`Error: ${response.message}`, false);
        }
      });
    },
    "Delete",
    "danger"
  );
}

/* ── PAT (Personal Access Token) helpers ──────────────────────── */
function showPATAlert(message, isSuccess) {
  const el = document.getElementById("patAlert");
  if (!el) return;
  el.className = `alert fade mt-3 ${isSuccess ? "alert-success" : "alert-danger"}`;
  const span = el.querySelector("span") || el;
  span.textContent = message;
  el.classList.remove("d-none");
  el.classList.add("show");
}

function renderPATRow(pat) {
  const expiresText = pat.expiresAt ? `Expires: ${new Date(pat.expiresAt).toLocaleDateString()}` : "No expiry";
  const lastUsedText = pat.lastUsedAt ? `Last used: ${new Date(pat.lastUsedAt).toLocaleString()}` : "Never used";
  return `
    <li class="list-group-item d-flex justify-content-between align-items-center" id="pat-${pat.id}">
      <span>
        <i class="bi bi-key me-2"></i>${pat.name}
        <small class="text-muted d-block">${lastUsedText} · ${expiresText}</small>
      </span>
      <button type="button" class="btn btn-sm btn-outline-danger btn-delete-pat" data-id="${pat.id}" title="Delete token">
        <i class="bi bi-trash"></i>
      </button>
    </li>
  `;
}

function loadPATs() {
  const list = document.getElementById("patList");
  const loading = document.getElementById("patLoading");
  if (!list) return;

  if (loading) loading.classList.remove("d-none");

  proviant.getPATs().then((response) => {
    if (loading) loading.classList.add("d-none");
    if (response.code === 200) {
      const pats = response.message;
      if (!pats || !Array.isArray(pats) || pats.length === 0) {
        list.innerHTML = '<li class="list-group-item text-center text-muted py-3">No tokens created yet.</li>';
        return;
      }
      list.innerHTML = pats.map((p) => renderPATRow(p)).join("");
    } else {
      showPATAlert(`Error: ${response.message || "Unknown error"}`, false);
    }
  });
}

function handleCreatePAT() {
  const modalEl = document.getElementById("createPatModal");
  const nameInput = document.getElementById("inputPatName");
  const expiryInput = document.getElementById("inputPatExpiry");
  const tokenDisplay = document.getElementById("newTokenDisplay");
  const tokenValue = document.getElementById("newTokenValue");

  nameInput.value = "";
  expiryInput.value = "";
  tokenDisplay.classList.add("d-none");
  tokenValue.textContent = "";

  if (window.bootstrap) {
    const modal = new bootstrap.Modal(modalEl);
    modal.show();
  }
}

function handleSavePAT() {
  const nameInput = document.getElementById("inputPatName");
  const expiryInput = document.getElementById("inputPatExpiry");
  const name = nameInput ? nameInput.value.trim() : "";

  if (!name) {
    if (nameInput) nameInput.classList.add("is-invalid");
    return;
  }
  if (nameInput) nameInput.classList.remove("is-invalid");

  const btn = document.getElementById("btnSavePAT");
  setButtonLoading(btn, true);

  let expiresAt = null;
  const expiryInputVal = expiryInput ? expiryInput.value : "";
  if (expiryInputVal) {
    expiresAt = new Date(expiryInputVal).toISOString();
  }

  proviant.createPAT(name, expiresAt).then((response) => {
    setButtonLoading(btn, false);
    if (response.code === 201) {
      const tokenDisplay = document.getElementById("newTokenDisplay");
      const tokenValue = document.getElementById("newTokenValue");
      tokenValue.textContent = response.message.token;
      tokenDisplay.classList.remove("d-none");
      loadPATs();
    } else {
      showPATAlert(`Error: ${response.message || "Failed to create token"}`, false);
    }
  });
}

function handleDeletePAT(btn) {
  const patID = btn.dataset.id;
  proviant.showConfirm(
    "Delete Token",
    "Permanently delete this API token? This cannot be undone.",
    function () {
      proviant.deletePAT(patID).then((response) => {
        if (response.code === 200) {
          const row = document.getElementById(`pat-${patID}`);
          if (row) row.remove();
        } else {
          showPATAlert(`Error: ${response.message || "Could not delete token"}`, false);
        }
      });
    },
    "Delete",
    "danger"
  );
}

/* ── Event delegation — clicks ───────────────────────────────────── */
document.addEventListener("click", function (event) {
  const target = event.target;

  if (target.closest("#btnUpdatePersonalDetails")) {
    event.preventDefault();
    UpdateSettings();
    return;
  }

  if (target.closest("#btnUpdatePassword")) {
    event.preventDefault();
    UpdatePassword();
    return;
  }

  if (target.closest("#btnUpdateNotificationSettings")) {
    event.preventDefault();
    UpdateNotificationSettings();
    return;
  }

  if (target.closest("#btnCreatePAT")) {
    event.preventDefault();
    handleCreatePAT();
    return;
  }

  if (target.closest("#btnSavePAT")) {
    event.preventDefault();
    handleSavePAT();
    return;
  }

  const deletePatBtn = target.closest(".btn-delete-pat");
  if (deletePatBtn) {
    event.preventDefault();
    handleDeletePAT(deletePatBtn);
    return;
  }

  if (target.closest("#btnLeaveHousehold")) {
    event.preventDefault();
    handleLeaveHousehold();
    return;
  }

  if (target.closest("#btnCreateHousehold")) {
    event.preventDefault();
    handleCreateHousehold();
    return;
  }

  if (target.closest("#btnApplyHousehold")) {
    event.preventDefault();
    handleApplyHousehold();
    return;
  }

  if (target.closest("#btnUpdateHouseholdName")) {
    event.preventDefault();
    handleUpdateHouseholdName();
    return;
  }

  if (target.closest("#btnSendInvitation")) {
    event.preventDefault();
    handleSendInvitation();
    return;
  }

  if (target.closest("#btnRefreshUsers")) {
    event.preventDefault();
    handleRefreshUsers();
    return;
  }

  const editBtn = target.closest(".btn-edit-user");
  if (editBtn) {
    event.preventDefault();
    handleEditUser(editBtn);
    return;
  }

  if (target.closest("#btnSaveUserEdit")) {
    event.preventDefault();
    handleSaveUserEdit();
    return;
  }

  const resetBtn = target.closest(".btn-reset-password");
  if (resetBtn) {
    event.preventDefault();
    handleResetPassword(resetBtn);
    return;
  }

  const deleteBtn = target.closest(".btn-delete-user");
  if (deleteBtn) {
    event.preventDefault();
    handleDeleteUser(deleteBtn);
    return;
  }

  const approveBtn = target.closest(".btn-approve-application");
  if (approveBtn) {
    event.preventDefault();
    handleApproveApplication(approveBtn);
    return;
  }

  const rejectBtn = target.closest(".btn-reject-application");
  if (rejectBtn) {
    event.preventDefault();
    handleRejectApplication(rejectBtn);
    return;
  }

  const cancelBtn = target.closest(".btn-cancel-application");
  if (cancelBtn) {
    event.preventDefault();
    handleCancelApplication(cancelBtn);
    return;
  }

  const removeBtn = target.closest(".btn-remove-member");
  if (removeBtn) {
    event.preventDefault();
    handleRemoveMember(removeBtn);
    return;
  }

  const cancelInviteBtn = target.closest(".btn-cancel-invitation");
  if (cancelInviteBtn) {
    event.preventDefault();
    const invitationID = cancelInviteBtn.dataset.id;
    proviant.showConfirm('Cancel Invitation', 'Cancel this invitation? The recipient will no longer be able to use it.', function () {
      proviant.cancelInvitation(invitationID).then((response) => {
        if (response.code === 200) {
          const row = document.getElementById(`invitation-${invitationID}`);
          if (row) row.remove();
        } else {
          proviant.showFeedback('error', 'Error', response.message || 'Could not cancel invitation.');
        }
      });
    });
    return;
  }
});

/* ── Event delegation — inputs ───────────────────────────────────── */
document.addEventListener("input", function (event) {
  const target = event.target;
  const clearInvalid = (el) => {
    if (el && el.classList.contains("is-invalid")) el.classList.remove("is-invalid");
  };

  switch (target.id) {
    case "inputUsername":
    case "inputMailAddress":
      clearInvalid(target);
      break;
    case "inputPassword":
    case "inputPasswordVerification":
      clearInvalid(target);
      break;
    case "inputNtfyUrl":
    case "inputNtfyTopic":
    case "inputNotificationThreshold":
      clearInvalid(target);
      break;
    case "editUsername":
    case "editMailAddress":
      clearInvalid(target);
      break;
  }
});

/* ── Event delegation — checkbox changes ─────────────────────────── */
document.addEventListener("change", function (event) {
  if (event.target.id === "toggleNtfyNotifications") {
    toggleNtfySettings();
  }
});

/* ── Auto-load admin users on page load ─────────────────────────── */
document.addEventListener("DOMContentLoaded", function () {
  loadAdminUsers();
  loadPATs();
});
