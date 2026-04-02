function showUpdateError(message) {
  const updateAlert = document.getElementById("updateAlert");
  if (updateAlert) {
    updateAlert.innerText = message;
    updateAlert.style.display = "";
  }
}

function hideUpdateError() {
  const updateAlert = document.getElementById("updateAlert");
  if (updateAlert) {
    updateAlert.innerText = "";
    updateAlert.style.display = "none";
  }
}

function showPasswordError(message) {
  const passwordAlert = document.getElementById("passwordAlert");
  if (passwordAlert) {
    passwordAlert.innerText = message;
    passwordAlert.style.display = "";
  }
}

function hidePasswordError() {
  const passwordAlert = document.getElementById("passwordAlert");
  if (passwordAlert) {
    passwordAlert.innerText = "";
    passwordAlert.style.display = "none";
  }
}

function ShowSuccessModal(message, btnFunction) {
  const modalBody = document.getElementById("modalBody");
  const btnModal = document.getElementById("btnModal");
  if (modalBody) modalBody.innerText = message;
  if (btnModal) btnModal.onclick = btnFunction;
  const successModal = new bootstrap.Modal(document.getElementById("modal"));
  successModal.show();
}

/* Update user settings */
function UpdateSettings() {
  let formIsValid = true;
  const inputUsername = document.getElementById("inputUsername");
  const inputMailAddress = document.getElementById("inputMailAddress");
  if (!inputUsername || !inputMailAddress) return;

  const username = inputUsername.value;
  const mailAddress = inputMailAddress.value;

  if (username == "") {
    if (!inputUsername.classList.contains("is-invalid")) {
      inputUsername.classList.toggle("is-invalid");
    }
    formIsValid = false;
  }
  if (mailAddress == "") {
    if (!inputMailAddress.classList.contains("is-invalid")) {
      inputMailAddress.classList.toggle("is-invalid");
    }
    formIsValid = false;
  }

  if (!formIsValid) {
    return;
  }

  proviant.updateUser(username, mailAddress).then((response) => {
    switch (response.code) {
      case 200:
        console.error(response.message);
        ShowSuccessModal(
          "User update complete. Please reload page to show changed values.",
          function (event) {
            event.preventDefault();
            location.reload();
          },
        );
        break;
      case 401:
        console.error(response.message);
        showUpdateError(`Update failed: ${response.message}`);
        break;
      default:
        console.error(response.message);
        showUpdateError(`Undefined update error: ${response.message}`);
        break;
    }
  });
}

/* Update password */
function UpdatePassword() {
  let formIsValid = true;
  const inputPassword = document.getElementById("inputPassword");
  const inputPasswordVerification = document.getElementById("inputPasswordVerification");
  const inputUsername = document.getElementById("inputUsername");
  if (!inputPassword || !inputPasswordVerification || !inputUsername) return;

  const password = inputPassword.value;
  const passwordVerification = inputPasswordVerification.value;
  const username = inputUsername.value;

  if (username == "") {
    if (!inputUsername.classList.contains("is-invalid")) {
      inputUsername.classList.toggle("is-invalid");
    }
    formIsValid = false;
  }
  if (password == "") {
    if (!inputPassword.classList.contains("is-invalid")) {
      inputPassword.classList.toggle("is-invalid");
    }
    formIsValid = false;
  }
  if (passwordVerification == "") {
    if (!inputPasswordVerification.classList.contains("is-invalid")) {
      inputPasswordVerification.classList.toggle("is-invalid");
    }
    formIsValid = false;
  }

  if (password != passwordVerification) {
    if (!inputPassword.classList.contains("is-invalid")) {
      inputPassword.classList.toggle("is-invalid");
    }
    if (!inputPasswordVerification.classList.contains("is-invalid")) {
      inputPasswordVerification.classList.toggle("is-invalid");
    }
    formIsValid = false;
  }

  if (!formIsValid) {
    return;
  }

  proviant.updateUserPassword(username, password).then((response) => {
    switch (response.code) {
      case 200:
        console.error(response.message);
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
        console.error(response.message);
        showPasswordError(`Update failed: ${response.message}`);
        break;
      default:
        console.error(response.message);
        showPasswordError(`Undefined update error: ${response.message}`);
        break;
    }
  });
}

/* Notification settings functions */
function toggleNtfySettings() {
  const toggleNtfyNotifications = document.getElementById("toggleNtfyNotifications");
  const ntfySettings = document.getElementById("ntfySettings");
  if (ntfySettings && toggleNtfyNotifications) {
    ntfySettings.style.display = toggleNtfyNotifications.checked ? "" : "none";
  }
}

function showNotificationError(message) {
  const notificationAlert = document.getElementById("notificationAlert");
  if (notificationAlert) {
    notificationAlert.innerText = message;
    notificationAlert.style.display = "";
  }
}

function hideNotificationError() {
  const notificationAlert = document.getElementById("notificationAlert");
  if (notificationAlert) {
    notificationAlert.innerText = "";
    notificationAlert.style.display = "none";
  }
}

function UpdateNotificationSettings() {
  const toggleEmailNotifications = document.getElementById("toggleEmailNotifications");
  const toggleNtfyNotifications = document.getElementById("toggleNtfyNotifications");
  const inputNtfyUrl = document.getElementById("inputNtfyUrl");
  const inputNtfyTopic = document.getElementById("inputNtfyTopic");
  const inputNtfyToken = document.getElementById("inputNtfyToken");
  if (!toggleEmailNotifications || !toggleNtfyNotifications || !inputNtfyUrl || !inputNtfyTopic || !inputNtfyToken) return;

  // Validate inputs
  if (toggleNtfyNotifications.checked) {
    if (!inputNtfyUrl.value) {
      showNotificationError(
        "ntfy.sh URL is required when ntfy notifications are enabled",
      );
      return;
    }

    if (!inputNtfyTopic.value) {
      showNotificationError(
        "ntfy.sh topic is required when ntfy notifications are enabled",
      );
      return;
    }
  }

  // Prepare data for API call
  const notificationData = {
    emailEnabled: toggleEmailNotifications.checked,
    ntfyEnabled: toggleNtfyNotifications.checked,
    ntfyUrl: inputNtfyUrl.value || "",
    ntfyTopic: inputNtfyTopic.value || "",
    ntfyToken: inputNtfyToken.value || "",
  };

  // Call backend API
  proviant
    .updateNotificationSettings(notificationData)
    .then((response) => {
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
          showNotificationError(`Update failed: ${response.message}`);
          break;
        case 401:
          showNotificationError(`Unauthorized: ${response.message}`);
          break;
        default:
          showNotificationError(
            `Error updating notification settings: ${response.message}`,
          );
          break;
      }
    })
    .catch((error) => {
      showNotificationError(`Network error: ${error.message}`);
    });
}

/* Household management helpers */
function showHouseholdAlert(elementId, message, isSuccess) {
  const el = document.getElementById(elementId);
  if (!el) return;
  el.textContent = message;
  el.className = `alert mt-3 ${isSuccess ? "alert-success" : "alert-danger"}`;
}

function hideHouseholdAlert(elementId) {
  const el = document.getElementById(elementId);
  if (el) el.classList.add("d-none");
}

/* Leave household */
function handleLeaveHousehold() {
  if (!confirm("Leave your current household? You will be assigned a new personal household.")) return;
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
  if (!confirm("Cancel this application?")) return;
  proviant.cancelApplication(applicationID).then((response) => {
    if (response.code === 200) {
      const row = document.getElementById(`my-application-${applicationID}`);
      if (row) row.remove();
    } else {
      showHouseholdAlert("myApplicationsAlert", `Error: ${response.message}`, false);
    }
  });
}

/* Remove household member */
function handleRemoveMember(btn) {
  const memberID = btn.dataset.id;
  if (!confirm("Remove this member from the household?")) return;
  proviant.removeMember(memberID).then((response) => {
    if (response.code === 200) {
      const row = document.getElementById(`member-${memberID}`);
      if (row) row.remove();
    } else {
      showHouseholdAlert("membersAlert", `Error: ${response.message}`, false);
    }
  });
}

/* Event delegation for all clicks */
document.addEventListener("click", function (event) {
  const target = event.target;

  // Update personal details
  if (target.closest("#btnUpdatePersonalDetails")) {
    event.preventDefault();
    UpdateSettings();
    return;
  }

  // Update password
  if (target.closest("#btnUpdatePassword")) {
    event.preventDefault();
    UpdatePassword();
    return;
  }

  // Update notification settings
  if (target.closest("#btnUpdateNotificationSettings")) {
    event.preventDefault();
    UpdateNotificationSettings();
    return;
  }

  // Leave household
  if (target.closest("#btnLeaveHousehold")) {
    event.preventDefault();
    handleLeaveHousehold();
    return;
  }

  // Create household
  if (target.closest("#btnCreateHousehold")) {
    event.preventDefault();
    handleCreateHousehold();
    return;
  }

  // Apply to join household
  if (target.closest("#btnApplyHousehold")) {
    event.preventDefault();
    handleApplyHousehold();
    return;
  }

  // Update household name
  if (target.closest("#btnUpdateHouseholdName")) {
    event.preventDefault();
    handleUpdateHouseholdName();
    return;
  }

  // Approve application
  const approveBtn = target.closest(".btn-approve-application");
  if (approveBtn) {
    event.preventDefault();
    handleApproveApplication(approveBtn);
    return;
  }

  // Reject application
  const rejectBtn = target.closest(".btn-reject-application");
  if (rejectBtn) {
    event.preventDefault();
    handleRejectApplication(rejectBtn);
    return;
  }

  // Cancel application
  const cancelBtn = target.closest(".btn-cancel-application");
  if (cancelBtn) {
    event.preventDefault();
    handleCancelApplication(cancelBtn);
    return;
  }

  // Remove member
  const removeBtn = target.closest(".btn-remove-member");
  if (removeBtn) {
    event.preventDefault();
    handleRemoveMember(removeBtn);
    return;
  }
});

/* Event delegation for input changes */
document.addEventListener("input", function (event) {
  const target = event.target;

  // Username input
  if (target.id === "inputUsername") {
    if (target.value.length > 0 && target.classList.contains("is-invalid")) {
      target.classList.toggle("is-invalid");
    }
    const updateAlert = document.getElementById("updateAlert");
    if (updateAlert && updateAlert.style.display == "") {
      hideUpdateError();
    }
    return;
  }

  // Mail address input
  if (target.id === "inputMailAddress") {
    if (target.value.length > 0 && target.classList.contains("is-invalid")) {
      target.classList.toggle("is-invalid");
    }
    const updateAlert = document.getElementById("updateAlert");
    if (updateAlert && updateAlert.style.display == "") {
      hideUpdateError();
    }
    return;
  }

  // Password input
  if (target.id === "inputPassword") {
    if (target.value.length > 0 && target.classList.contains("is-invalid")) {
      target.classList.toggle("is-invalid");
    }
    const passwordAlert = document.getElementById("passwordAlert");
    if (passwordAlert && passwordAlert.style.display == "") {
      hidePasswordError();
    }
    return;
  }

  // Password verification input
  if (target.id === "inputPasswordVerification") {
    if (target.value.length > 0 && target.classList.contains("is-invalid")) {
      target.classList.toggle("is-invalid");
    }
    const passwordAlert = document.getElementById("passwordAlert");
    if (passwordAlert && passwordAlert.style.display == "") {
      hidePasswordError();
    }
    return;
  }
});

/* Event delegation for checkbox changes */
document.addEventListener("change", function (event) {
  const target = event.target;
  if (target.id === "toggleNtfyNotifications") {
    toggleNtfySettings();
  }
});
/* Invitation management helpers */
function showInviteAlert(elementId, message, isSuccess) {
  const el = document.getElementById(elementId);
  if (!el) return;
  el.textContent = message;
  el.className = `alert mt-3 ${isSuccess ? "alert-success" : "alert-danger"}`;
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

/* Cancel invitation — event delegation */
document.addEventListener("click", function (event) {
  const cancelInviteBtn = event.target.closest(".btn-cancel-invitation");
  if (cancelInviteBtn) {
    const invitationID = cancelInviteBtn.dataset.id;
    if (!confirm("Cancel this invitation?")) return;
    proviant.cancelInvitation(invitationID).then((response) => {
      if (response.code === 200) {
        const row = document.getElementById(`invitation-${invitationID}`);
        if (row) row.remove();
      } else {
        showInviteAlert("invitationsAlert", `Error: ${response.message}`, false);
      }
    });
    return;
  }

  // Send invitation button
  if (event.target.closest("#btnSendInvitation")) {
    event.preventDefault();
    handleSendInvitation();
    return;
  }
});
