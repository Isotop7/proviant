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

function validatePasswordRequirements(password) {
    const pwInput = document.getElementById("inputPassword");
    const minLen = pwInput ? parseInt(pwInput.dataset.minLength || "12", 10) : 12;
    const requirements = {
        length: password.length >= minLen,
        upper: /[A-Z]/.test(password),
        digit: /[0-9]/.test(password),
        special: /[^A-Za-z0-9]/.test(password)
    };

    const elements = {
        length: document.getElementById("req-length"),
        upper: document.getElementById("req-upper"),
        digit: document.getElementById("req-digit"),
        special: document.getElementById("req-special")
    };

    for (const [key, satisfied] of Object.entries(requirements)) {
        const el = elements[key];
        if (!el) continue;
        const icon = el.querySelector("i");
        if (!icon) continue;
        icon.className = satisfied
            ? "bi bi-check-circle text-success me-1"
            : "bi bi-x-circle text-danger me-1";
        el.classList.toggle("text-success", satisfied);
        el.classList.toggle("text-danger", !satisfied);
    }
}

/* ── Update personal details ─────────────────────────────────────── */
function UpdateSettings() {
  let formIsValid = true;
  const inputDisplayName = document.getElementById("inputDisplayName");
  const inputMailAddress = document.getElementById("inputMailAddress");
  if (!inputMailAddress) return;

  if (!inputMailAddress.value) {
    inputMailAddress.classList.add("is-invalid");
    formIsValid = false;
  }
  if (!formIsValid) return;

  const btn = document.getElementById("btnUpdatePersonalDetails");
  setButtonLoading(btn, true);

  proviant.updateUser(inputDisplayName ? inputDisplayName.value : "", inputMailAddress.value).then((response) => {
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
  const minLength = parseInt(inputPassword.dataset.minLength || "12", 10);
  if (!password) {
    inputPassword.classList.add("is-invalid");
    formIsValid = false;
  }
  if (password && password.length < minLength) {
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

/* ── Monthly waste goal ──────────────────────────────────────────── */

function updateMonthlyGoalVisibility() {
  const countRow = document.getElementById("monthlyGoalCountRow");
  const percentRow = document.getElementById("monthlyGoalPercentRow");
  const countChecked = document.getElementById("monthlyGoalTypeCount")?.checked;
  const percentChecked = document.getElementById("monthlyGoalTypePercent")?.checked;
  if (countRow) countRow.style.display = countChecked ? "" : "none";
  if (percentRow) percentRow.style.display = percentChecked ? "" : "none";
}

function saveMonthlyGoal() {
  const typeEl = document.querySelector('input[name="monthlyGoalType"]:checked');
  const type = typeEl ? typeEl.value : "";
  let count = null;
  let percent = null;
  if (type === "count") {
    const raw = document.getElementById("inputMonthlyGoalCount")?.value;
    const parsed = raw === undefined || raw === "" ? NaN : parseInt(raw, 10);
    if (Number.isNaN(parsed) || parsed < 0) {
      proviant.showFeedback("error", "Invalid goal", "Count must be a non-negative integer.");
      return;
    }
    count = parsed;
  } else if (type === "percent") {
    const raw = document.getElementById("inputMonthlyGoalPercent")?.value;
    const parsed = raw === undefined || raw === "" ? NaN : parseFloat(raw);
    if (Number.isNaN(parsed) || parsed < 0 || parsed > 100) {
      proviant.showFeedback("error", "Invalid goal", "Percent must be between 0 and 100.");
      return;
    }
    percent = parsed;
  }

  const btn = document.getElementById("btnSaveMonthlyGoal");
  setButtonLoading(btn, true);
  proviant.updateHouseholdSettings({
    monthlyWasteGoalType: type,
    monthlyWasteGoalCount: count,
    monthlyWasteGoalPercent: percent,
  }).then((response) => {
    setButtonLoading(btn, false);
    if (response.code === 200) {
      ShowSuccessModal("Monthly waste goal updated.", function (e) {
        e.preventDefault();
        location.reload();
      });
    } else {
      proviant.showFeedback("error", "Update Failed", response.message || "Could not save goal.");
    }
  }).catch((err) => {
    setButtonLoading(btn, false);
    proviant.showFeedback("error", "Network error", err.message);
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

function toggleEmailSettings() {
  const toggle = document.getElementById("toggleEmailNotifications");
  const settings = document.getElementById("emailSettings");
  if (toggle && settings) {
    settings.classList.toggle("d-none", !toggle.checked);
  }
}

function toggleMonthlyWasteReportSettings() {
  const toggle = document.getElementById("toggleMonthlyWasteReport");
  const settings = document.getElementById("monthlyWasteReportSettings");
  if (toggle && settings) {
    settings.classList.toggle("d-none", !toggle.checked);
  }
}

function toggleTelegramSettings() {
  const toggle = document.getElementById("toggleTelegramNotifications");
  const settings = document.getElementById("telegramSettings");
  if (toggle && settings) {
    settings.classList.toggle("d-none", !toggle.checked);
  }
}

function toggleWebPushSettings() {
  const toggle = document.getElementById("togglePushNotifications");
  const settings = document.getElementById("pushSettings");
  if (toggle && settings) {
    settings.classList.toggle("d-none", !toggle.checked);
  }
}

function toggleWebhookCreate() {
  const toggle = document.getElementById("toggleWebhookCreate");
  const settings = document.getElementById("webhookCreateSettings");
  if (toggle && settings) {
    settings.classList.toggle("d-none", !toggle.checked);
  }
}

function showTelegramLinkAlert(message, isSuccess) {
  const el = document.getElementById("telegramLinkAlert");
  if (!el) return;
  el.className = `alert fade mt-3 ${isSuccess ? "alert-success" : "alert-danger"}`;
  const span = el.querySelector("span") || el;
  span.textContent = message;
  el.classList.remove("d-none");
  el.classList.add("show");
}

function handleGenerateTelegramToken() {
  const btn = document.getElementById("btnGenerateTelegramToken");
  setButtonLoading(btn, true);

  proviant.generateTelegramLinkToken().then((response) => {
    setButtonLoading(btn, false);
    if (response.code !== 200) {
      showTelegramLinkAlert("Failed to generate token. Please try again.", false);
      return;
    }

    const cmdEl = document.getElementById("telegramLinkCommand");
    const deepLinkEl = document.getElementById("telegramDeepLink");
    const cmdTextEl = document.getElementById("telegramLinkCommandText");
    const manualEl = document.getElementById("telegramManualCommand");
    const copiedEl = document.getElementById("telegramCommandCopied");

    const token = response.token;
    const botUsername = response.botUsername;

    cmdEl.classList.remove("d-none");
    if (copiedEl) copiedEl.classList.add("d-none");

    if (cmdTextEl) cmdTextEl.value = `/start ${token}`;
    if (manualEl) manualEl.classList.remove("d-none");

    if (botUsername) {
      const deepLinkUrl = `https://t.me/${botUsername}?start=${token}`;
      deepLinkEl.href = deepLinkUrl;
      deepLinkEl.classList.remove("d-none");
    } else {
      deepLinkEl.classList.add("d-none");
    }
  }).catch(() => {
    setButtonLoading(btn, false);
    showTelegramLinkAlert("Network error. Please try again.", false);
  });
}

function handleCopyTelegramCommand() {
  const cmdTextEl = document.getElementById("telegramLinkCommandText");
  const copiedEl = document.getElementById("telegramCommandCopied");
  if (!cmdTextEl || !cmdTextEl.value) return;
  proviant.copyToClipboard(cmdTextEl.value).then((success) => {
    if (success && copiedEl) {
      copiedEl.classList.remove("d-none");
      setTimeout(() => copiedEl.classList.add("d-none"), 2000);
    }
  });
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
    monthlyWasteReportEnabled: document.getElementById("toggleMonthlyWasteReport")?.checked ?? false,
    telegramEnabled: document.getElementById("toggleTelegramNotifications")?.checked ?? false,
    telegramBotToken: document.getElementById("inputTelegramBotToken")?.value ?? "",
    digestFrequency: document.querySelector('input[name="digestFrequency"]:checked')?.value ?? "disabled",
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
          proviant.showFeedback('error', 'Error', `Error: ${response.message}`);
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
      proviant.showFeedback('error', 'Error', `Error: ${response.message}`);
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
      proviant.showFeedback('warning', 'Already Applied', "You already have a pending application for this household.");
    } else if (response.code === 404) {
      proviant.showFeedback('error', 'Not Found', "Household not found.");
    } else {
      proviant.showFeedback('error', 'Apply Failed', `Error: ${response.message}`);
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
       proviant.showFeedback('error', 'Error', `Error: ${response.message}`);
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
       proviant.showFeedback('error', 'Error', `Error: ${response.message}`);
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
      proviant.showFeedback('error', 'Rename Failed', `Error: ${response.message}`);
    }
  });
}

/* Cancel own pending application */
function handleCancelApplication(btn) {
  const applicationID = btn.dataset.id;
  proviant.showConfirm('Cancel Application', 'Cancel this pending application to join the household?', function () {
    proviant.cancelApplication(applicationID).then((response) => {
    if (response.code === 200) {
      const row = document.getElementById(`application-${applicationID}`);
      if (row) row.remove();
    } else {
      proviant.showFeedback('error', 'Error', `Error: ${response.message}`);
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
      proviant.showFeedback('warning', 'Already Invited', response.message, false);
    } else if (response.code === 400) {
      proviant.showFeedback('error', 'Invalid Email', response.message, false);
    } else {
      proviant.showFeedback('error', 'Error', `Error: ${response.message}`);
    }
  });
}

/* ── Admin User Management ─────────────────────────────────────── */

function renderAdminUserRow(user, currentUserID, currentUserIsAdmin, householdAdminID) {
  if (!user || !user.username) return "";
  const isSelf = user.id === currentUserID;
  const isAdmin = user.id === householdAdminID;

  const roleLabel = isAdmin ? "Admin" : (user.role === "viewer" ? "Viewer" : (user.role === "member" ? "Member" : user.role || "Member"));
  const roleBadgeClass = isAdmin ? "bg-primary" : (user.role === "viewer" ? "bg-warning" : "bg-secondary");
  const roleIcon = isAdmin ? "bi-shield-check" : (user.role === "viewer" ? "bi-eye" : "bi-person");

  return `
    <li class="list-group-item d-flex justify-content-between align-items-center" id="admin-user-${user.id}">
      <span>
        <i class="bi bi-person me-2"></i>${user.username}
        <span class="text-secondary-custom ms-1">&lt;${user.mailAddress || ""}&gt;</span>
        ${isSelf ? '<span class="badge bg-secondary ms-1">You</span>' : ""}
        <span class="badge ${roleBadgeClass} ms-1"><i class="bi ${roleIcon} me-1"></i>${roleLabel}</span>
      </span>
      <div class="d-flex align-items-center gap-2">
        ${!isSelf ? `
          <button type="button" class="btn btn-outline-primary btn-edit-user" data-id="${user.id}" data-username="${user.username}" data-email="${user.mailAddress || ""}" data-role="${user.role || 'member'}" data-is-admin="${isAdmin}" data-is-self="${isSelf}" title="Edit user">
            <i class="bi bi-pencil"></i>
          </button>` : ""}
          <button type="button" class="btn btn-outline-warning btn-reset-password" data-id="${user.id}" title="Reset password">
            <i class="bi bi-key"></i>
          </button>
          ${!isSelf ? `
          <button type="button" class="btn btn-outline-danger btn-delete-user" data-id="${user.id}" title="Delete user">
            <i class="bi bi-trash"></i>
          </button>` : ""}
        </div>
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
        list.innerHTML = '<li class="list-group-item text-center text-secondary-custom py-3">No users in household.</li>';
        return;
      }
      const currentUserID = parseInt(list.dataset.currentUserId || "0", 10) || 0;
      const householdAdminID = parseInt(list.dataset.adminId || "0", 10) || 0;
      const rendered = users.map((u) => renderAdminUserRow(u, currentUserID, currentUserID === householdAdminID, householdAdminID)).join("");
      if (!rendered) {
        list.innerHTML = '<li class="list-group-item text-center text-secondary-custom py-3">No users in household.</li>';
        return;
      }
      list.innerHTML = rendered;
    } else if (response.code === 403) {
      list.innerHTML = '<li class="list-group-item text-danger py-3">Access denied.</li>';
    } else {
      proviant.showFeedback('error', 'Error', `Error: ${response.message || "Unknown error"}`);
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
  const role = btn.dataset.role;
  const isAdmin = btn.dataset.isAdmin === 'true';
  const isSelf = btn.dataset.isSelf === 'true';
  document.getElementById("editUserID").value = userID;
  document.getElementById("editUsername").value = username;
  document.getElementById("editMailAddress").value = email;
  const roleSelect = document.getElementById("editUserRoleSelect");
  roleSelect.value = role || 'member';
  roleSelect.dataset.currentRole = role || 'member';
  const roleRow = document.getElementById("editUserRoleRow");
  if (roleRow) {
    roleRow.classList.toggle('d-none', isAdmin || isSelf);
    roleRow.dataset.isSelf = isSelf ? 'true' : 'false';
  }
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
  const newRole = document.getElementById("editUserRoleSelect").value;
  const roleRow = document.getElementById("editUserRoleRow");
  const isSelf = roleRow && roleRow.dataset.isSelf === 'true';
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

  const currentRole = document.getElementById("editUserRoleSelect").dataset.currentRole || newRole;
  if (isSelf && newRole !== 'admin' && newRole !== currentRole) {
    proviant.showConfirm(
      'Change Your Role',
      `You are about to change your own role from ${currentRole} to ${newRole}. You will lose admin privileges. Continue?`,
      function () { doSaveUserEdit(userID, username, email, newRole); },
      'Change Role',
      'warning'
    );
  } else {
    doSaveUserEdit(userID, username, email, newRole);
  }
}

function doSaveUserEdit(userID, username, email, newRole) {
  const btn = document.getElementById("btnSaveUserEdit");
  setButtonLoading(btn, true);

  proviant.updateHouseholdUser(userID, username, email).then((response) => {
    if (response.code === 200) {
      if (newRole) {
        proviant.updateMemberRole(userID, newRole).then((roleResponse) => {
          setButtonLoading(btn, false);
          if (roleResponse.code === 200) {
            const modalEl = document.getElementById("editUserModal");
            if (window.bootstrap) {
              bootstrap.Modal.getInstance(modalEl)?.hide();
            }
            loadAdminUsers();
          } else {
            proviant.showFeedback('error', 'Error', `User updated but role change failed: ${roleResponse.message}`);
          }
        });
      } else {
        setButtonLoading(btn, false);
        const modalEl = document.getElementById("editUserModal");
        if (window.bootstrap) {
          bootstrap.Modal.getInstance(modalEl)?.hide();
        }
        loadAdminUsers();
      }
    } else {
      setButtonLoading(btn, false);
      proviant.showFeedback('error', 'Error', `Error: ${response.message}`);
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
          proviant.showFeedback('error', 'Error', `Error: ${response.message}`);
        }
      });
    },
    "Delete",
    "danger"
  );
}

/* ── PAT (Personal Access Token) helpers ──────────────────────── */

function renderPATRow(pat) {
  const expiresText = pat.expiresAt ? `Expires: ${new Date(pat.expiresAt).toLocaleDateString()}` : "No expiry";
  const lastUsedText = pat.lastUsedAt ? `Last used: ${new Date(pat.lastUsedAt).toLocaleString()}` : "Never used";
  return `
    <li class="list-group-item d-flex justify-content-between align-items-center" id="pat-${pat.id}">
      <span>
        <i class="bi bi-key me-2"></i>${pat.name}
        <small class="text-secondary-custom d-block">${lastUsedText} · ${expiresText}</small>
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
        list.innerHTML = '<li class="list-group-item text-center text-secondary-custom py-3">No tokens created yet.</li>';
        return;
      }
      list.innerHTML = pats.map((p) => renderPATRow(p)).join("");
    } else {
      proviant.showFeedback('error', 'Error', `Error: ${response.message || "Unknown error"}`);
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
      proviant.showFeedback('error', 'Error', `Error: ${response.message || "Failed to create token"}`);
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
          proviant.showFeedback('error', 'Error', `Error: ${response.message || "Could not delete token"}`);
        }
      });
    },
    "Delete",
    "danger"
  );
}

/* ── Webhook helpers ─────────────────────────────────────────────── */
let currentWebhooks = [];

function LoadWebhooks() {
  proviant.getWebhooks().then((response) => {
    if (response.code === 200) {
      currentWebhooks = response.webhooks || [];
      RenderWebhookList();
    } else {
      proviant.showFeedback('error', 'Error', `Failed to load webhooks: ${response.message}`);
    }
  });
}

function RenderWebhookList() {
  const container = document.getElementById('webhookList');
  if (!container) return;

  if (currentWebhooks.length === 0) {
    container.innerHTML = '<div class="text-center text-secondary-custom py-4">No webhooks configured. Create one below.</div>';
    return;
  }

  let html = '';
  for (const wh of currentWebhooks) {
    const eventsList = wh.events.join(', ');
    const toggleChecked = wh.active ? 'checked' : '';
    html += `
      <div class="card mb-3 webhook-card" data-id="${wh.id}">
        <div class="card-body">
          <div class="d-flex justify-content-between align-items-start mb-2">
            <div>
              <h6 class="mb-1">${escapeHtml(wh.url)}</h6>
              <small class="text-secondary-custom">Events: ${escapeHtml(eventsList)}</small>
            </div>
            <div class="form-check form-switch">
              <input class="form-check-input webhook-toggle-active" type="checkbox" ${toggleChecked} data-id="${wh.id}">
              <label class="form-check-label">Active</label>
            </div>
          </div>
          <div class="btn-group btn-group-sm">
            <button type="button" class="btn btn-outline-primary btn-view-deliveries" data-id="${wh.id}">View Deliveries</button>
            <button type="button" class="btn btn-outline-secondary btn-copy-secret" data-id="${wh.id}">Copy Secret</button>
            <button type="button" class="btn btn-outline-danger btn-delete-webhook" data-id="${wh.id}">Delete</button>
          </div>
        </div>
      </div>
    `;
  }
  container.innerHTML = html;
  attachWebhookEventHandlers();
}

function attachWebhookEventHandlers() {
  document.querySelectorAll('.webhook-toggle-active').forEach(function(toggle) {
    toggle.addEventListener('change', function() {
      const id = parseInt(this.dataset.id);
      const wh = currentWebhooks.find(w => w.id === id);
      if (wh) {
        ToggleWebhookActive(id, !wh.active);
      }
    });
  });

  document.querySelectorAll('.btn-view-deliveries').forEach(function(btn) {
    btn.addEventListener('click', function() {
      const id = parseInt(this.dataset.id);
      ShowWebhookDeliveries(id);
    });
  });

  document.querySelectorAll('.btn-copy-secret').forEach(function(btn) {
    btn.addEventListener('click', function() {
      const id = parseInt(this.dataset.id);
      CopyWebhookSecret(id);
    });
  });

  document.querySelectorAll('.btn-delete-webhook').forEach(function(btn) {
    btn.addEventListener('click', function() {
      const id = parseInt(this.dataset.id);
      DeleteWebhook(id);
    });
  });
}

function ToggleWebhookActive(id, active) {
  const wh = currentWebhooks.find(w => w.id === id);
  if (!wh) return;

  proviant.updateWebhook(id, wh.url, null, wh.events, active).then((response) => {
    if (response.code === 200) {
      wh.active = active;
      RenderWebhookList();
    } else {
      proviant.showFeedback('error', 'Error', `Failed to update webhook: ${response.message}`);
      RenderWebhookList();
    }
  });
}

function ShowWebhookDeliveries(id) {
  proviant.getWebhookDeliveries(id).then((response) => {
    if (response.code === 200) {
      ShowDeliveriesModal(response.deliveries || []);
    } else {
      proviant.showFeedback('error', 'Error', `Failed to load deliveries: ${response.message}`);
    }
  });
}

function ShowDeliveriesModal(deliveries) {
  let html = '<div class="table-responsive"><table class="table table-sm"><thead><tr><th>Time</th><th>Attempt</th><th>Status</th><th>Response</th><th>Error</th></tr></thead><tbody>';

  if (deliveries.length === 0) {
    html += '<tr><td colspan="5" class="text-center text-secondary-custom">No delivery attempts yet</td></tr>';
  } else {
    for (const d of deliveries) {
      const statusClass = d.statusCode >= 200 && d.statusCode < 300 ? 'text-success' : 'text-danger';
      html += `<tr>
        <td>${escapeHtml(d.createdAt)}</td>
        <td>${d.attempt}</td>
        <td class="${statusClass}">${d.statusCode || '-'}</td>
        <td><small>${escapeHtml(d.responseBody || '-')}</small></td>
        <td><small class="text-danger">${escapeHtml(d.error || '')}</small></td>
      </tr>`;
    }
  }
  html += '</tbody></table></div>';

  document.getElementById('deliveriesModalBody').innerHTML = html;
  const modal = new bootstrap.Modal(document.getElementById('webhookDeliveriesModal'));
  modal.show();
}

function CopyWebhookSecret(id) {
  const wh = currentWebhooks.find(w => w.id === id);
  if (!wh) return;
  document.getElementById('webhookSecretCopy').value = wh.secret || '';
  const modal = new bootstrap.Modal(document.getElementById('webhookSecretModal'));
  modal.show();
}

function DeleteWebhook(id) {
  proviant.showConfirm(
    'Delete Webhook',
    'Are you sure you want to delete this webhook? This cannot be undone.',
    function() {
      proviant.deleteWebhook(id).then((response) => {
        if (response.code === 200) {
          ShowSuccessModal('Webhook deleted', function() {
            LoadWebhooks();
          });
        } else {
          proviant.showFeedback('error', 'Error', `Failed to delete webhook: ${response.message}`);
        }
      });
    },
    'Delete',
    'danger'
  );
}

function CreateWebhook() {
  const url = document.getElementById('inputWebhookUrl').value.trim();
  const secret = document.getElementById('inputWebhookSecret').value;
  const events = getSelectedWebhookEvents();

  if (!url) {
    document.getElementById('inputWebhookUrl').classList.add('is-invalid');
    return;
  }
  if (!secret || secret.length < 16) {
    document.getElementById('inputWebhookSecret').classList.add('is-invalid');
    return;
  }
  if (events.length === 0) {
    proviant.showFeedback('error', 'Error', 'Please select at least one event');
    return;
  }

  const btn = document.getElementById('btnCreateWebhook');
  setButtonLoading(btn, true);

  proviant.createWebhook(url, secret, events, true).then((response) => {
    setButtonLoading(btn, false);
    if (response.code === 201) {
      document.getElementById('inputWebhookUrl').value = '';
      document.getElementById('inputWebhookSecret').value = '';
      uncheckAllWebhookEvents();
      const toggleWebhook = document.getElementById('toggleWebhookCreate');
      if (toggleWebhook) { toggleWebhook.checked = false; toggleWebhookCreate(); }
      ShowSuccessModal('Webhook created', function() {
        LoadWebhooks();
      });
    } else {
      proviant.showFeedback('error', 'Error', `Failed to create webhook: ${response.message}`);
    }
  });
}

function getSelectedWebhookEvents() {
  const events = [];
  document.querySelectorAll('.webhook-event-checkbox:checked').forEach(function(cb) {
    events.push(cb.dataset.event);
  });
  return events;
}

function uncheckAllWebhookEvents() {
  document.querySelectorAll('.webhook-event-checkbox').forEach(function(cb) {
    cb.checked = false;
  });
}

function escapeHtml(str) {
  if (!str) return '';
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

/* ── Audit Log (Admin) ───────────────────────────────────────────── */

const AUDIT_ACTION_ICONS = {
  login_success: "bi bi-check-circle text-success",
  login_failure: "bi bi-x-circle text-danger",
  password_change: "bi bi-key text-primary",
  password_change_failed: "bi bi-x-octagon text-danger",
  member_added: "bi bi-person-plus text-success",
  member_removed: "bi bi-person-dash text-warning",
  member_left: "bi bi-box-arrow-left text-muted",
  admin_changed: "bi bi-shield-check text-primary",
  account_deleted: "bi bi-trash text-danger",
};

const AUDIT_ACTION_LABELS = {
  login_success: "Login successful",
  login_failure: "Login failed",
  password_change: "Password changed",
  password_change_failed: "Password change failed",
  member_added: "Member joined",
  member_removed: "Member removed",
  member_left: "Member left",
  admin_changed: "Admin changed",
  account_deleted: "Account deleted",
};

function renderAuditLogRow(log) {
  const iconClass = AUDIT_ACTION_ICONS[log.action] || "bi bi-shield text-secondary";
  const label = AUDIT_ACTION_LABELS[log.action] || log.action || "Unknown";
  const time = log.timestamp ? new Date(log.timestamp).toLocaleTimeString() : "";
  const userText = log.userId ? `#${log.userId}` : "System";
  const details = log.details ? escapeHtml(log.details) : "";

  return `
    <li class="list-group-item px-4 py-2">
      <div class="d-flex align-items-start gap-2">
        <i class="${iconClass}" aria-hidden="true" style="margin-top: 2px;"></i>
        <div class="flex-grow-1 min-w-0">
          <div class="d-flex justify-content-between align-items-start gap-2">
            <span class="fw-medium small">${label}</span>
            <small class="text-secondary-custom flex-shrink-0" style="font-size: var(--text-xs);">${time}</small>
          </div>
          <div class="d-flex gap-2 mt-1">
            <small class="text-secondary-custom" style="font-size: var(--text-xs);">User ${userText}</small>
            ${log.ipAddress ? `<small class="text-secondary-custom" style="font-size: var(--text-xs);">· ${escapeHtml(log.ipAddress)}</small>` : ""}
          </div>
          ${details ? `<small class="text-secondary-custom d-block mt-1" style="font-size: var(--text-xs);">${details}</small>` : ""}
        </div>
      </div>
    </li>
  `;
}

function renderAuditLog(logs) {
  const list = document.getElementById("auditLogList");
  const loading = document.getElementById("auditLogLoading");
  const meta = document.getElementById("auditLogMeta");
  if (!list) return;

  if (loading) loading.remove();

  if (!logs || logs.length === 0) {
    list.innerHTML = '<li class="list-group-item text-center text-secondary-custom py-4">No activity logged today.</li>';
    if (meta) meta.textContent = "";
    return;
  }

  const rows = logs.map((log) => renderAuditLogRow(log)).join("");
  list.innerHTML = rows;
  if (meta) meta.textContent = `${logs.length} event${logs.length !== 1 ? "s" : ""} · Auto-refreshes every 30s`;
}

function loadAuditLog(date) {
  const list = document.getElementById("auditLogList");
  const loading = document.getElementById("auditLogLoading");
  if (!list) return;

  if (loading) {
    list.innerHTML = `<li class="list-group-item text-center text-secondary-custom py-4" id="auditLogLoading">
      <span class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
      <span>Loading…</span>
    </li>`;
  }

  proviant.getAuditLogs(date).then((response) => {
    if (response.code === 200) {
      renderAuditLog(response.message);
    } else {
      list.innerHTML = `<li class="list-group-item text-danger text-center py-4">Failed to load activity log.</li>`;
      if (document.getElementById("auditLogMeta")) {
        document.getElementById("auditLogMeta").textContent = `Error: ${response.message || "Unknown"}`;
      }
    }
  }).catch(() => {
    list.innerHTML = `<li class="list-group-item text-danger text-center py-4">Network error.</li>`;
  });
}

let auditLogRefreshTimer = null;

function handleRefreshAuditLog() {
  const today = new Date().toISOString().split("T")[0];
  loadAuditLog(today);
  if (auditLogRefreshTimer) clearInterval(auditLogRefreshTimer);
  auditLogRefreshTimer = setInterval(() => {
    loadAuditLog(today);
  }, 30000);
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

  if (target.closest("#btnSaveMonthlyGoal")) {
    event.preventDefault();
    saveMonthlyGoal();
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

  if (target.closest("#btnCalendarCreate")) {
    event.preventDefault();
    handleCalendarCreate();
    return;
  }

  if (target.closest("#btnCalendarRegenerate")) {
    event.preventDefault();
    handleCalendarRegenerate();
    return;
  }

  if (target.closest("#btnCalendarCopy")) {
    event.preventDefault();
    handleCalendarCopy();
    return;
  }

  if (target.closest("#btnCalendarDownload")) {
    event.preventDefault();
    handleCalendarDownload();
    return;
  }

  if (target.closest("#btnCalendarRemove")) {
    event.preventDefault();
    handleCalendarRemove();
    return;
  }

  if (target.closest("#btnCreateWebhook")) {
    event.preventDefault();
    CreateWebhook();
    return;
  }

  if (target.closest("#btnGenerateTelegramToken")) {
    event.preventDefault();
    handleGenerateTelegramToken();
    return;
  }

  if (target.closest("#btnCopyTelegramCommand")) {
    event.preventDefault();
    handleCopyTelegramCommand();
    return;
  }

  if (target.closest("#btnCloseWebhookSecret")) {
    document.getElementById('webhookSecretCopy').value = '';
    return;
  }

  if (target.closest("#btnCopySecret")) {
    event.preventDefault();
    const secret = document.getElementById('webhookSecretCopy').value;
    proviant.copyToClipboard(secret).then(function(success) {
      if (success) {
        proviant.showFeedback('success', 'Copied', 'Secret copied to clipboard');
      }
    });
    return;
  }

  if (target.closest("#btnRefreshAuditLog")) {
    event.preventDefault();
    handleRefreshAuditLog();
    return;
  }

  // Storage location — add
  if (target.closest("#btnAddStorageLocation")) {
    event.preventDefault();
    openStorageLocationModal(null);
    return;
  }

  // Storage location — edit
  if (target.closest(".btn-edit-sl")) {
    event.preventDefault();
    const btn = target.closest(".btn-edit-sl");
    openStorageLocationModal({
      id: btn.dataset.id,
      name: btn.dataset.name,
      icon: btn.dataset.icon,
      sortOrder: btn.dataset.sortorder,
    });
    return;
  }

  // Storage location — delete
  if (target.closest(".btn-delete-sl")) {
    event.preventDefault();
    const btn = target.closest(".btn-delete-sl");
    proviant.showConfirm(
      "Delete Location",
      `Delete "${btn.dataset.name}"? Products in this location will become unassigned.`,
      function () {
        deleteStorageLocation(btn.dataset.id);
      },
      "Delete",
      "danger"
    );
    return;
  }

  // Storage location — save modal
  if (target.closest("#btnSaveStorageLocation")) {
    event.preventDefault();
    saveStorageLocation();
    return;
  }

  // Push notification — unsubscribe button
  if (target.closest("#btnPushUnsubscribe")) {
    event.preventDefault();
    handleWebPushUnsubscribe();
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
    case "inputDisplayName":
      clearInvalid(target);
      break;
    case "inputMailAddress":
      if (target.dataset.touched) {
        const valid = target.validity.valid && target.value.trim() !== "";
        target.classList.toggle("is-invalid", !valid);
        target.classList.toggle("is-valid", valid);
      } else {
        clearInvalid(target);
      }
      break;
    case "inputPassword": {
      if (target.dataset.touched) {
        const minLen = parseInt(target.dataset.minLength || "12", 10);
        const valid = target.value.length >= minLen;
        target.classList.toggle("is-invalid", !valid);
        target.classList.toggle("is-valid", valid);
      } else {
        clearInvalid(target);
      }
      validatePasswordRequirements(target.value);
      const inputPasswordVerification = document.getElementById("inputPasswordVerification");
      if (inputPasswordVerification && inputPasswordVerification.dataset.touched) {
        const match = inputPasswordVerification.value === target.value && inputPasswordVerification.value !== "";
        inputPasswordVerification.classList.toggle("is-invalid", !match);
        inputPasswordVerification.classList.toggle("is-valid", match);
      }
      break;
    }
    case "inputPasswordVerification": {
      if (target.dataset.touched) {
        const inputPassword = document.getElementById("inputPassword");
        const match = inputPassword && target.value === inputPassword.value && target.value !== "";
        target.classList.toggle("is-invalid", !match);
        target.classList.toggle("is-valid", match);
      } else {
        clearInvalid(target);
      }
      break;
    }
    case "inputNtfyUrl":
    case "inputNtfyTopic":
    case "inputNotificationThreshold":
      clearInvalid(target);
      break;
    case "inputWebhookUrl":
    case "inputWebhookSecret":
      clearInvalid(target);
      break;
    case "inputMonthlyGoalCount":
    case "inputMonthlyGoalPercent":
      clearInvalid(target);
      break;
    case "editUsername":
    case "editMailAddress":
      clearInvalid(target);
      break;
  }
});

/* ── Event delegation — radio changes ────────────────────────────── */
document.addEventListener("change", function (event) {
  const target = event.target;
  if (target && target.name === "monthlyGoalType") {
    updateMonthlyGoalVisibility();
  }
});

/* ── Event delegation — checkbox changes ─────────────────────────── */
document.addEventListener("change", function (event) {
  if (event.target.id === "toggleNtfyNotifications") {
    toggleNtfySettings();
  }
  if (event.target.id === "toggleEmailNotifications") {
    toggleEmailSettings();
  }
  if (event.target.id === "toggleMonthlyWasteReport") {
    toggleMonthlyWasteReportSettings();
  }
  if (event.target.id === "toggleTelegramNotifications") {
    toggleTelegramSettings();
  }
  if (event.target.id === "toggleWebhookCreate") {
    toggleWebhookCreate();
  }
  if (event.target.id === "togglePushNotifications") {
    toggleWebPushSettings();
    handleWebPushToggle();
  }

});

/* ── Auto-load admin users on page load ─────────────────────────── */
document.addEventListener("DOMContentLoaded", function () {
  loadAdminUsers();
  loadPATs();
  loadCalendarTokenStatus();
  LoadWebhooks();
  initWebPushNotifications();

  const auditLogSection = document.getElementById("auditLogSection");
  if (auditLogSection) {
    handleRefreshAuditLog();
  }
});

/* ── Blur validation ─────────────────────────────────────────────── */
document.addEventListener("focusout", function (event) {
  const target = event.target;
  switch (target.id) {
    case "inputMailAddress": {
      const valid = target.validity.valid && target.value.trim() !== "";
      target.dataset.touched = "1";
      target.classList.toggle("is-invalid", !valid);
      target.classList.toggle("is-valid", valid);
      break;
    }
    case "inputPassword": {
      const minLen = parseInt(target.dataset.minLength || "12", 10);
      const valid = target.value.length >= minLen;
      target.dataset.touched = "1";
      target.classList.toggle("is-invalid", !valid);
      target.classList.toggle("is-valid", valid);
      break;
    }
    case "inputPasswordVerification": {
      const inputPassword = document.getElementById("inputPassword");
      const match = inputPassword && target.value === inputPassword.value && target.value !== "";
      target.dataset.touched = "1";
      target.classList.toggle("is-invalid", !match);
      target.classList.toggle("is-valid", match);
      break;
    }
  }
});

/* ── Calendar Token Management ──────────────────────────────────── */

function loadCalendarTokenStatus() {
  proviant.getCalendarTokenStatus().then((response) => {
    if (response.code === 200) {
      const hasToken = response.message.hasToken;
      const url = response.message.url;

      const noTokenEl = document.getElementById("calendarNoToken");
      const hasTokenEl = document.getElementById("calendarHasToken");
      const urlInput = document.getElementById("calendarUrl");

      if (hasToken && url) {
        noTokenEl.classList.add("d-none");
        hasTokenEl.classList.remove("d-none");
        urlInput.value = url;

        const expiresAt = response.message.expiresAt;
        const isExpiringSoon = response.message.isExpiringSoon;
        const expirySection = document.getElementById("calendarExpirySection");
        const expiryBadge = document.getElementById("calendarExpiryBadge");
        const expiryDate = document.getElementById("calendarExpiryDate");

        if (expirySection) {
          expirySection.classList.remove("d-none");
          if (isExpiringSoon && expiryBadge) {
            expiryBadge.classList.remove("d-none");
          } else if (expiryBadge) {
            expiryBadge.classList.add("d-none");
          }
          if (expiryDate && expiresAt) {
            const expiryDateObj = new Date(expiresAt);
            expiryDate.textContent = "Expires: " + expiryDateObj.toLocaleDateString();
          }
        }
      } else {
        noTokenEl.classList.remove("d-none");
        hasTokenEl.classList.add("d-none");
      }
    }
  });
}

function handleCalendarCreate() {
  const btn = document.getElementById("btnCalendarCreate");
  if (btn) {
    setButtonLoading(btn, true);
  }

  proviant.createCalendarToken().then((response) => {
    if (btn) setButtonLoading(btn, false);
    if (response.code === 201) {
      const url = response.message.url;
      const noTokenEl = document.getElementById("calendarNoToken");
      const hasTokenEl = document.getElementById("calendarHasToken");
      const urlInput = document.getElementById("calendarUrl");

      noTokenEl.classList.add("d-none");
      hasTokenEl.classList.remove("d-none");
      urlInput.value = url;

      proviant.showFeedback('success', 'Calendar Token Created', 'Calendar token created. Subscribe using the URL above.');
    } else {
      proviant.showFeedback('error', 'Error', `Error: ${response.message || "Failed to create calendar token"}`);
    }
  });
}

function handleCalendarRegenerate() {
  proviant.showConfirm(
    "Regenerate Token",
    "This will invalidate your current calendar URL and create a new one. Update your calendar subscription with the new URL.",
    function () {
      const btn = document.getElementById("btnCalendarRegenerate");
      if (btn) setButtonLoading(btn, true);

      proviant.rotateCalendarToken().then((response) => {
        if (btn) setButtonLoading(btn, false);
        if (response.code === 201) {
          const url = response.message.url;
          const urlInput = document.getElementById("calendarUrl");
          urlInput.value = url;

          const expiresAt = response.message.expiresAt;
          const expirySection = document.getElementById("calendarExpirySection");
          const expiryBadge = document.getElementById("calendarExpiryBadge");
          const expiryDate = document.getElementById("calendarExpiryDate");

          if (expirySection) {
            expirySection.classList.remove("d-none");
            if (expiryBadge) expiryBadge.classList.add("d-none");
            if (expiryDate && expiresAt) {
              const expiryDateObj = new Date(expiresAt);
              expiryDate.textContent = "Expires: " + expiryDateObj.toLocaleDateString();
            }
          }

          proviant.showFeedback('success', 'Token Regenerated', 'Calendar token regenerated with new URL.');
        } else {
          proviant.showFeedback('error', 'Error', `Error: ${response.message || "Failed to regenerate calendar token"}`);
        }
      });
    },
    "Regenerate",
    "warning"
  );
}

function handleCalendarCopy() {
  const urlInput = document.getElementById("calendarUrl");
  if (!urlInput || !urlInput.value) return;

  proviant.copyToClipboard(urlInput.value).then((success) => {
    const copiedEl = document.getElementById("calendarUrlCopied");
    if (copiedEl) {
      if (success) {
        copiedEl.classList.remove("d-none");
        setTimeout(() => {
          copiedEl.classList.add("d-none");
        }, 3000);
      }
    }
  });
}

function handleCalendarDownload() {
  const urlInput = document.getElementById("calendarUrl");
  if (!urlInput || !urlInput.value) return;

  const url = urlInput.value;
  const tokenMatch = url.match(/token=([^&]+)/);
  if (tokenMatch && tokenMatch[1]) {
    proviant.downloadCalendarICS(tokenMatch[1]);
  }
}

function handleCalendarRemove() {
  proviant.showConfirm(
    "Remove Calendar Sync",
    "This will delete your calendar token and invalidate the subscription URL. Your calendar app will stop receiving updates.",
    function () {
      proviant.deleteCalendarToken().then((response) => {
        if (response.code === 200) {
          const noTokenEl = document.getElementById("calendarNoToken");
          const hasTokenEl = document.getElementById("calendarHasToken");
          const urlInput = document.getElementById("calendarUrl");

          noTokenEl.classList.remove("d-none");
          hasTokenEl.classList.add("d-none");
          urlInput.value = "";

          proviant.showFeedback('success', 'Calendar Sync Removed', 'Calendar token removed.');
        } else {
          proviant.showFeedback('error', 'Error', `Error: ${response.message || "Failed to remove calendar token"}`);
        }
      });
    },
    "Remove",
    "danger"
  );
}

/* ── Storage Location CRUD ───────────────────────────────────────── */
function openStorageLocationModal(data) {
  const modalEl = document.getElementById("storageLocationModal");
  if (!modalEl) return;
  const idInput       = document.getElementById("slModalID");
  const nameInput     = document.getElementById("slModalName");
  const iconInput     = document.getElementById("slModalIcon");
  const sortInput     = document.getElementById("slModalSortOrder");
  const titleEl       = document.getElementById("storageLocationModalLabel");

  if (data) {
    if (idInput)    idInput.value    = data.id        || "";
    if (nameInput)  nameInput.value  = data.name      || "";
    if (iconInput)  iconInput.value  = data.icon      || "";
    if (sortInput)  sortInput.value  = data.sortOrder != null ? data.sortOrder : 0;
    if (titleEl)    titleEl.textContent = "Edit Storage Location";
  } else {
    if (idInput)    idInput.value    = "";
    if (nameInput)  nameInput.value  = "";
    if (iconInput)  iconInput.value  = "";
    if (sortInput)  sortInput.value  = "0";
    if (titleEl)    titleEl.textContent = "Add Storage Location";
  }

  const bsModal = bootstrap.Modal.getOrCreateInstance(modalEl);
  bsModal.show();
}

function saveStorageLocation() {
  const idInput   = document.getElementById("slModalID");
  const nameInput = document.getElementById("slModalName");
  const iconInput = document.getElementById("slModalIcon");
  const sortInput = document.getElementById("slModalSortOrder");
  const saveBtn   = document.getElementById("btnSaveStorageLocation");

  if (!nameInput || !nameInput.value.trim()) {
    if (nameInput) nameInput.classList.add("is-invalid");
    return;
  }
  if (nameInput) nameInput.classList.remove("is-invalid");

  const id        = idInput ? idInput.value.trim() : "";
  const name      = nameInput.value.trim();
  const icon      = iconInput ? iconInput.value.trim() : "";
  const sortOrder = sortInput ? parseInt(sortInput.value, 10) || 0 : 0;

  setButtonLoading(saveBtn, true);

  const isEdit = id !== "";
  const url    = isEdit ? `/api/v1/household/storage-locations/${id}` : "/api/v1/household/storage-locations";
  const method = isEdit ? "PATCH" : "POST";

  fetch(url, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, icon, sortOrder }),
  })
    .then((r) => r.json())
    .then((response) => {
      setButtonLoading(saveBtn, false);
      const code = response.code || response.Code;
      if (code === 200 || code === 201) {
        const loc = response.message || response.Message || {};
        const locId   = loc.id   || loc.ID   || id;
        const locName = loc.name || loc.Name || name;
        const locIcon = loc.icon || loc.Icon || icon;
        const locSort = loc.sortOrder != null ? loc.sortOrder : (loc.SortOrder != null ? loc.SortOrder : sortOrder);

        const modalEl = document.getElementById("storageLocationModal");
        if (modalEl) bootstrap.Modal.getOrCreateInstance(modalEl).hide();

        const list = document.getElementById("storageLocationList");
        if (!list) { location.reload(); return; }

        const existing = document.getElementById(`sl-${locId}`);
        const li = existing || document.createElement("li");
        li.id = `sl-${locId}`;
        li.className = "list-group-item d-flex justify-content-between align-items-center";
        li.innerHTML = `
          <span>${locIcon} <strong>${locName}</strong></span>
          <div class="btn-group btn-group-sm">
            <button type="button" class="btn btn-proviant-secondary btn-edit-sl"
                    data-id="${locId}" data-name="${locName}" data-icon="${locIcon}"
                    data-sortorder="${locSort}" aria-label="Edit ${locName}">
              <i class="bi bi-pencil" aria-hidden="true"></i>
            </button>
            <button type="button" class="btn btn-proviant-danger btn-delete-sl"
                    data-id="${locId}" data-name="${locName}" aria-label="Delete ${locName}">
              <i class="bi bi-trash" aria-hidden="true"></i>
            </button>
          </div>`;

        if (!existing) list.appendChild(li);

        proviant.showFeedback("success", isEdit ? "Location Updated" : "Location Added",
          `"${locName}" ${isEdit ? "updated" : "added"} successfully.`);
      } else {
        const msg = response.message || response.Message || "Unknown error";
        proviant.showFeedback("error", "Error", `Failed to save location: ${msg}`);
      }
    })
    .catch((err) => {
      setButtonLoading(saveBtn, false);
      proviant.showFeedback("error", "Error", `Request failed: ${err}`);
    });
}

function deleteStorageLocation(id) {
  fetch(`/api/v1/household/storage-locations/${id}`, {
    method: "DELETE",
  })
    .then((r) => r.json())
    .then((response) => {
      const code = response.code || response.Code;
      if (code === 200) {
        const li = document.getElementById(`sl-${id}`);
        if (li) li.remove();
        proviant.showFeedback("success", "Location Deleted", "Storage location removed.");
      } else {
        const msg = response.message || response.Message || "Unknown error";
        proviant.showFeedback("error", "Error", `Failed to delete location: ${msg}`);
      }
    })
    .catch((err) => {
      proviant.showFeedback("error", "Error", `Request failed: ${err}`);
    });
}

/* ── Web Push Notification Management ─────────────────────────────────────── */
let cachedWebPushVAPIDKey = null;

async function initWebPushNotifications() {
  if (!('Notification' in window) || !('serviceWorker' in navigator)) {
    const toggle = document.getElementById("togglePushNotifications");
    const helpBtn = document.getElementById("btnPushHelp");
    if (toggle) toggle.disabled = true;
    if (helpBtn) helpBtn.disabled = true;
    return;
  }

  const permission = Notification.permission;
  const toggle = document.getElementById("togglePushNotifications");
  const pushSettings = document.getElementById("pushSettings");

  if (permission === 'granted') {
    if (toggle) toggle.checked = true;
    if (pushSettings) pushSettings.classList.remove("d-none");
  } else if (permission === 'denied') {
    if (toggle) toggle.checked = false;
    if (toggle) toggle.disabled = true;
  }

  try {
    const keyResponse = await proviant.getWebPushVAPIDPublicKey();
    if (keyResponse.code === 200) {
      cachedWebPushVAPIDKey = keyResponse.publicKey;
    }
  } catch (e) {
    console.error('Failed to fetch VAPID key:', e);
  }
}

async function handleWebPushToggle() {
  if (!cachedWebPushVAPIDKey) {
    try {
      const keyResponse = await proviant.getWebPushVAPIDPublicKey();
      if (keyResponse.code === 200) {
        cachedWebPushVAPIDKey = keyResponse.publicKey;
      }
    } catch {
      proviant.showFeedback('error', 'Error', 'Failed to initialize web push notifications.');
      return;
    }
  }

  const permission = Notification.permission;
  if (permission === 'denied') {
    proviant.showFeedback('warning', 'Blocked', 'Push notifications are blocked. Please enable them in your browser settings.');
    const toggle = document.getElementById("togglePushNotifications");
    if (toggle) toggle.checked = false;
    return;
  }

  if (permission !== 'granted') {
    const result = await Notification.requestPermission();
    if (result !== 'granted') {
      proviant.showFeedback('warning', 'Not Allowed', 'Push notification permission denied.');
      return;
    }
  }

  try {
    const sw = await navigator.serviceWorker.ready;
    const subscription = await sw.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: cachedWebPushVAPIDKey,
    });

    const subJSON = subscription.toJSON();
    const response = await proviant.subscribeWebPush({
      endpoint: subJSON.endpoint,
      keys: {
        p256dh: subJSON.keys.p256dh,
        auth: subJSON.keys.auth,
      },
    });

    if (response.code === 200) {
      const pushSettings = document.getElementById("pushSettings");
      if (pushSettings) pushSettings.classList.remove("d-none");
      proviant.showFeedback('success', 'Enabled', 'Push notifications enabled.');
    } else {
      proviant.showFeedback('error', 'Error', 'Failed to save push subscription.');
    }
  } catch (e) {
    proviant.showFeedback('error', 'Error', `Push subscription failed: ${e.message}`);
  }
}

async function handleWebPushUnsubscribe() {
  try {
    const sw = await navigator.serviceWorker.ready;
    const subscription = await sw.pushManager.getSubscription();
    if (subscription) {
      await subscription.unsubscribe();
    }

    const response = await proviant.unsubscribeWebPush();
    if (response.code === 200) {
      const toggle = document.getElementById("togglePushNotifications");
      const pushSettings = document.getElementById("pushSettings");
      if (toggle) toggle.checked = false;
      if (pushSettings) pushSettings.classList.add("d-none");
      proviant.showFeedback('success', 'Disabled', 'Push notifications disabled.');
    } else {
      proviant.showFeedback('error', 'Error', 'Failed to disable push notifications.');
    }
  } catch (e) {
    proviant.showFeedback('error', 'Error', `Unsubscribe failed: ${e.message}`);
  }
}
