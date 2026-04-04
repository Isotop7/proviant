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
    ntfySettings.style.display = toggleNtfyNotifications.checked ? "" : "none";
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
  }
});

/* ── Event delegation — checkbox changes ─────────────────────────── */
document.addEventListener("change", function (event) {
  if (event.target.id === "toggleNtfyNotifications") {
    toggleNtfySettings();
  }
});
