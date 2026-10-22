/* ── Button loading helper ────────────────────────────────────────── */
function setButtonLoading(btn, loading) {
  if (!btn) return;
  if (loading) {
    btn.disabled = true;
    btn.dataset.originalHtml = btn.innerHTML;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span>' + proviant.t("common.saving");
  } else {
    btn.disabled = false;
    btn.innerHTML = btn.dataset.originalHtml || btn.innerHTML;
  }
}

function ShowSuccessModal(message, btnFunction) {
  proviant.showFeedback('success', proviant.t("js.settings.done"), message, btnFunction);
}

function validatePasswordRequirements(password) {
    const pwInput = document.getElementById("inputPassword");
    const minLen = pwInput ? Number.parseInt(pwInput.dataset.minLength || "12", 10) : 12;
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

  const inputLanguage = document.getElementById("inputLanguage");
  proviant.updateUser(inputDisplayName ? inputDisplayName.value : "", inputMailAddress.value, inputLanguage ? inputLanguage.value : undefined).then((response) => {
    setButtonLoading(btn, false);
    switch (response.code) {
      case 200:
        ShowSuccessModal(
          proviant.t("js.settings.userUpdateComplete"),
          function (event) {
            event.preventDefault();
            location.reload();
          },
        );
        break;
      case 401:
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.updateFailed", response.message));
        break;
      default:
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.updateFailed", response.message));
        break;
    }
  }).catch((err) => {
    setButtonLoading(btn, false);
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
  const minLength = Number.parseInt(inputPassword.dataset.minLength || "12", 10);
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
          proviant.t("js.settings.passwordUpdateComplete"),
          function (event) {
            event.preventDefault();
            void proviant.logoutUser();
            location.reload();
          },
        );
        break;
      case 401:
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.updateFailed", response.message));
        break;
      default:
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.updateFailed", response.message));
        break;
    }
  }).catch((err) => {
    setButtonLoading(btn, false);
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
    const parsed = raw === undefined || raw === "" ? Number.NaN : Number.parseInt(raw, 10);
    if (Number.isNaN(parsed) || parsed < 0) {
      proviant.showFeedback("error", proviant.t("js.settings.invalidGoalTitle"), proviant.t("js.settings.goalCountError"));
      return;
    }
    count = parsed;
  } else if (type === "percent") {
    const raw = document.getElementById("inputMonthlyGoalPercent")?.value;
    const parsed = raw === undefined || raw === "" ? Number.NaN : Number.parseFloat(raw);
    if (Number.isNaN(parsed) || parsed < 0 || parsed > 100) {
      proviant.showFeedback("error", proviant.t("js.settings.invalidGoalTitle"), proviant.t("js.settings.goalPercentError"));
      return;
    }
    percent = parsed;
  }

  const btn = document.getElementById("btnSaveMonthlyGoal");
  setButtonLoading(btn, true);
  const languageSelect = document.getElementById("inputHouseholdLanguage");
  proviant.updateHouseholdSettings({
    monthlyWasteGoalType: type,
    monthlyWasteGoalCount: count,
    monthlyWasteGoalPercent: percent,
    language: languageSelect ? languageSelect.value : undefined,
  }).then((response) => {
    setButtonLoading(btn, false);
    if (response.code === 200) {
      ShowSuccessModal(proviant.t("js.settings.monthlyGoalUpdated"), function (e) {
        e.preventDefault();
        location.reload();
      });
    } else {
      proviant.showFeedback("error", proviant.t("js.settings.updateFailedTitle"), response.message || proviant.t("js.settings.couldNotSaveGoal"));
    }
  }).catch((err) => {
    setButtonLoading(btn, false);
    proviant.showFeedback("error", proviant.t("js.settings.networkErrorTitle"), err.message);
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
      showTelegramLinkAlert(proviant.t("js.settings.generateTokenFailed"), false);
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
    showTelegramLinkAlert(proviant.t("js.settings.networkErrorRetry"), false);
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
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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

  const thresholdDays = inputNotificationThreshold ? Number.parseInt(inputNotificationThreshold.value, 10) : 0;
  if (Number.isNaN(thresholdDays) || thresholdDays < 0) {
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
            proviant.t("js.settings.notificationSettingsUpdated"),
            function (event) {
              event.preventDefault();
              location.reload();
            },
          );
          break;
        case 400:
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.updateFailed", response.message));
          break;
        case 401:
          proviant.showFeedback('error', proviant.t("js.settings.unauthorized"), response.message);
          break;
        default:
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.notificationSettingsUpdateFailed", response.message));
          break;
      }
    })
    .catch((error) => {
      setButtonLoading(btn, false);
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", error.message));
    });
}

/* ── Receipt scan settings ────────────────────────────────────────── */
function UpdateReceiptScanSettings() {
  const toggleOverride = document.getElementById("toggleReceiptScanOverride");
  const inputEndpoint = document.getElementById("inputReceiptScanEndpoint");
  const inputModel = document.getElementById("inputReceiptScanModel");
  const inputApiKey = document.getElementById("inputReceiptScanApiKey");
  const clearApiKeyInput = document.getElementById("clearReceiptScanApiKey");
  const inputTimeout = document.getElementById("inputReceiptScanTimeout");
  if (!toggleOverride) return;

  const endpointVal = inputEndpoint ? inputEndpoint.value.trim() : "";
  const timeoutVal = inputTimeout ? inputTimeout.value.trim() : "";

  if (endpointVal) {
    let valid;
    try {
      const parsed = new URL(endpointVal);
      valid = (parsed.protocol === "http:" || parsed.protocol === "https:") && parsed.host !== "";
    } catch (_e) { valid = false; }
    if (!valid) {
      if (inputEndpoint) inputEndpoint.classList.add("is-invalid");
      return;
    }
  }

  let timeoutNum = 0;
  if (timeoutVal) {
    timeoutNum = Number(timeoutVal);
    if (!Number.isInteger(timeoutNum) || timeoutNum < 1 || timeoutNum > 900) {
      if (inputTimeout) inputTimeout.classList.add("is-invalid");
      return;
    }
  }

  const btn = document.getElementById("btnUpdateReceiptScanSettings");
  setButtonLoading(btn, true);

  const settingsData = {
    overrideEnabled: toggleOverride.checked,
    endpoint: endpointVal,
    model: inputModel ? inputModel.value.trim() : "",
    apiKey: inputApiKey ? inputApiKey.value : "",
    clearApiKey: Boolean(clearApiKeyInput?.checked && !inputApiKey?.value),
    timeout: timeoutNum,
  };

  proviant
    .updateReceiptScanSettings(settingsData)
    .then((response) => {
      setButtonLoading(btn, false);
      if (response.code === 200) {
        ShowSuccessModal(proviant.t("js.settings.receiptScanUpdated"), function (event) {
          event.preventDefault();
          location.reload();
        });
      } else {
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.updateFailed", response.message));
      }
    })
    .catch((error) => {
      setButtonLoading(btn, false);
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", error.message));
    });
}

function ResetReceiptScanSettings() {
  proviant.showConfirm(
    proviant.t("js.settings.resetReceiptScanTitle"),
    proviant.t("js.settings.resetReceiptScanConfirm"),
    function () {
      const btn = document.getElementById("btnResetReceiptScanSettings");
      setButtonLoading(btn, true);

      const settingsData = {
        overrideEnabled: false,
        endpoint: "",
        model: "",
        apiKey: "",
        clearApiKey: true,
        timeout: 0,
      };

      proviant
        .updateReceiptScanSettings(settingsData)
        .then((response) => {
          setButtonLoading(btn, false);
          if (response.code === 200) {
            ShowSuccessModal(proviant.t("js.settings.receiptScanReset"), function (event) {
              event.preventDefault();
              location.reload();
            });
          } else {
            proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.resetFailed", response.message));
          }
        })
        .catch((error) => {
          setButtonLoading(btn, false);
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", error.message));
        });
    },
    proviant.t("js.settings.reset"),
    'danger'
  );
}

function toggleReceiptScanOverrideFields() {
  const toggle = document.getElementById("toggleReceiptScanOverride");
  const fields = document.getElementById("receiptScanOverrideFields");
  if (!toggle || !fields) return;
  fields.classList.toggle("d-none", !toggle.checked);
}

/* ── Household management helpers ────────────────────────────────── */

/* Leave household */
function handleLeaveHousehold() {
  proviant.showConfirm(
    proviant.t("household.leaveHousehold"),
    proviant.t("js.settings.leaveHouseholdConfirm"),
    function () {
      proviant.leaveHousehold().then((response) => {
        if (response.code === 200) {
          ShowSuccessModal(proviant.t("js.settings.leftHousehold"), function (e) {
            e.preventDefault();
            location.reload();
          });
        } else {
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message));
        }
      }).catch((err) => {
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
      });
    },
    proviant.t("household.leave"),
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
      ShowSuccessModal(proviant.t("js.settings.householdCreated"), function (e) {
        e.preventDefault();
        location.reload();
      });
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message));
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

/* Apply to join household */
function handleApplyHousehold() {
  const idInput = document.getElementById("inputApplyHouseholdID");
  const householdID = idInput ? Number.parseInt(idInput.value, 10) : Number.NaN;
  if (!householdID || householdID < 1) {
    if (idInput) idInput.classList.add("is-invalid");
    return;
  }
  if (idInput) idInput.classList.remove("is-invalid");
  proviant.applyForHousehold(householdID).then((response) => {
    if (response.code === 200) {
      location.reload();
    } else if (response.code === 409) {
      proviant.showFeedback('warning', proviant.t("js.settings.alreadyAppliedTitle"), proviant.t("js.settings.alreadyApplied"));
    } else if (response.code === 404) {
      proviant.showFeedback('error', proviant.t("js.settings.notFoundTitle"), proviant.t("js.settings.householdNotFound"));
    } else {
      proviant.showFeedback('error', proviant.t("js.settings.applyFailedTitle"), proviant.t("js.settings.errorWithMessage", response.message));
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
       proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message));
     }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
       proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message));
     }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
      proviant.showFeedback('error', proviant.t("js.settings.renameFailedTitle"), proviant.t("js.settings.errorWithMessage", response.message));
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

/* Cancel own pending application */
function handleCancelApplication(btn) {
  const applicationID = btn.dataset.id;
  proviant.showConfirm(proviant.t("js.settings.cancelApplicationTitle"), proviant.t("js.settings.cancelApplicationConfirm"), function () {
    proviant.cancelApplication(applicationID).then((response) => {
    if (response.code === 200) {
      const row = document.getElementById(`application-${applicationID}`);
      if (row) row.remove();
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message));
    }
    }).catch((err) => {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
    });
  });
}

/* Remove household member */
function handleRemoveMember(btn) {
  const memberID = btn.dataset.id;
  proviant.showConfirm(proviant.t("js.settings.removeMemberTitle"), proviant.t("js.settings.removeMemberConfirm"), function () {
    proviant.removeMember(memberID).then((response) => {
      if (response.code === 200) {
        const row = document.getElementById(`member-${memberID}`);
        if (row) row.remove();
      } else {
        proviant.showFeedback('error', proviant.t("common.error"), response.message || proviant.t("js.settings.couldNotRemoveMember"));
      }
    }).catch((err) => {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
      ShowSuccessModal(proviant.t("js.settings.invitationSent"), function (e) {
        e.preventDefault();
        location.reload();
      });
    } else if (response.code === 409) {
      proviant.showFeedback('warning', proviant.t("js.settings.alreadyInvitedTitle"), response.message, false);
    } else if (response.code === 400) {
      proviant.showFeedback('error', proviant.t("js.settings.invalidEmailTitle"), response.message, false);
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message));
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

/* ── Admin User Management ─────────────────────────────────────── */

function renderAdminUserRow(user, currentUserID, currentUserIsAdmin, householdAdminID) {
  if (!user || !user.username) return "";
  const isSelf = user.id === currentUserID;
  const isAdmin = user.id === householdAdminID;

  const roleLabel = isAdmin ? proviant.t("household.admin") : (user.role === "viewer" ? proviant.t("household.viewer") : (user.role === "member" ? proviant.t("household.member") : user.role || proviant.t("household.member")));
  const roleBadgeClass = isAdmin ? "bg-primary" : (user.role === "viewer" ? "bg-warning" : "bg-secondary");
  const roleIcon = isAdmin ? "bi-shield-check" : (user.role === "viewer" ? "bi-eye" : "bi-person");

  return `
    <li class="list-group-item d-flex justify-content-between align-items-center" id="admin-user-${user.id}">
      <span>
        <i class="bi bi-person me-2"></i>${user.username}
        <span class="text-body-secondary ms-1">&lt;${user.mailAddress || ""}&gt;</span>
        ${isSelf ? '<span class="badge bg-secondary ms-1">' + proviant.t("js.settings.you") + '</span>' : ""}
        <span class="badge ${roleBadgeClass} ms-1"><i class="bi ${roleIcon} me-1"></i>${roleLabel}</span>
      </span>
      <div class="d-flex align-items-center gap-2">
        ${!isSelf ? `
          <button type="button" class="btn btn-outline-primary btn-edit-user" data-id="${user.id}" data-username="${user.username}" data-email="${user.mailAddress || ""}" data-role="${user.role || 'member'}" data-is-admin="${isAdmin}" data-is-self="${isSelf}" title="${proviant.t("js.settings.editUserTitle")}">
            <i class="bi bi-pencil"></i>
          </button>` : ""}
          <button type="button" class="btn btn-outline-warning btn-reset-password" data-id="${user.id}" title="${proviant.t("js.settings.resetPasswordTooltip")}">
            <i class="bi bi-key"></i>
          </button>
          ${!isSelf ? `
          <button type="button" class="btn btn-outline-danger btn-delete-user" data-id="${user.id}" title="${proviant.t("js.settings.deleteUserTitle")}">
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
        list.innerHTML = '<li class="list-group-item text-center text-body-secondary py-3">' + proviant.t("js.settings.noUsers") + '</li>';
        return;
      }
      const currentUserID = Number.parseInt(list.dataset.currentUserId || "0", 10) || 0;
      const householdAdminID = Number.parseInt(list.dataset.adminId || "0", 10) || 0;
      const rendered = users.map((u) => renderAdminUserRow(u, currentUserID, currentUserID === householdAdminID, householdAdminID)).join("");
      if (!rendered) {
        list.innerHTML = '<li class="list-group-item text-center text-body-secondary py-3">' + proviant.t("js.settings.noUsers") + '</li>';
        return;
      }
      list.innerHTML = rendered;
    } else if (response.code === 403) {
      list.innerHTML = '<li class="list-group-item text-danger py-3">' + proviant.t("js.settings.accessDenied") + '</li>';
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message || proviant.t("js.settings.unknownError")));
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
      proviant.t("js.settings.changeRoleTitle"),
      proviant.t("js.settings.changeRoleConfirm", currentRole, newRole),
      function () { doSaveUserEdit(userID, username, email, newRole); },
      proviant.t("js.settings.changeRoleButton"),
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
            proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.userUpdatedRoleFailed", roleResponse.message));
          }
        }).catch((err) => {
          setButtonLoading(btn, false);
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message));
    }
  }).catch((err) => {
    setButtonLoading(btn, false);
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

function handleResetPassword(btn) {
  const userID = btn.dataset.id;
  proviant.showConfirm(
    proviant.t("js.settings.resetPasswordTitle"),
    proviant.t("js.settings.resetPasswordConfirm"),
    function () {
      proviant.resetHouseholdUserPassword(userID).then((response) => {
        if (response.code === 200) {
          proviant.showFeedback("success", proviant.t("js.settings.done"), proviant.t("js.settings.passwordResetSent"));
        } else {
          proviant.showFeedback("error", proviant.t("common.error"), response.message || proviant.t("js.settings.couldNotResetPassword"));
        }
      }).catch((err) => {
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
      });
    },
    proviant.t("js.settings.reset"),
    "warning"
  );
}

function handleDeleteUser(btn) {
  const userID = btn.dataset.id;
  proviant.showConfirm(
    proviant.t("js.settings.deleteUserConfirmTitle"),
    proviant.t("js.settings.deleteUserConfirm"),
    function () {
      proviant.deleteHouseholdUser(userID).then((response) => {
        if (response.code === 200) {
          const row = document.getElementById(`admin-user-${userID}`);
          if (row) row.remove();
        } else {
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message));
        }
      }).catch((err) => {
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
      });
    },
    proviant.t("common.delete"),
    "danger"
  );
}

/* ── PAT (Personal Access Token) helpers ──────────────────────── */

function renderPATRow(pat) {
  const expiresText = pat.expiresAt ? proviant.t("household.expiresAt", new Date(pat.expiresAt).toLocaleDateString()) : proviant.t("js.settings.noExpiry");
  const lastUsedText = pat.lastUsedAt ? proviant.t("js.settings.lastUsed", new Date(pat.lastUsedAt).toLocaleString()) : proviant.t("js.settings.neverUsed");
  return `
    <li class="list-group-item d-flex justify-content-between align-items-center" id="pat-${pat.id}">
      <span>
        <i class="bi bi-key me-2"></i>${pat.name}
        <small class="text-body-secondary d-block">${lastUsedText} · ${expiresText}</small>
      </span>
      <button type="button" class="btn btn-sm btn-outline-danger btn-delete-pat" data-id="${pat.id}" title="${proviant.t("js.settings.deleteTokenTitle")}">
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
        list.innerHTML = '<li class="list-group-item text-center text-body-secondary py-3">' + proviant.t("js.settings.noTokens") + '</li>';
        return;
      }
      list.innerHTML = pats.map((p) => renderPATRow(p)).join("");
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message || proviant.t("js.settings.unknownError")));
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message || proviant.t("js.settings.failedToCreateToken")));
    }
  }).catch((err) => {
    setButtonLoading(btn, false);
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

function handleDeletePAT(btn) {
  const patID = btn.dataset.id;
  proviant.showConfirm(
    proviant.t("js.settings.deleteTokenConfirmTitle"),
    proviant.t("js.settings.deleteTokenConfirm"),
    function () {
      proviant.deletePAT(patID).then((response) => {
        if (response.code === 200) {
          const row = document.getElementById(`pat-${patID}`);
          if (row) row.remove();
        } else {
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message || proviant.t("js.settings.couldNotDeleteToken")));
        }
      }).catch((err) => {
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
      });
    },
    proviant.t("common.delete"),
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
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.failedToLoadWebhooks", response.message));
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

function RenderWebhookList() {
  const container = document.getElementById('webhookList');
  if (!container) return;

  if (currentWebhooks.length === 0) {
    container.innerHTML = '<div class="text-center text-body-secondary py-4">' + proviant.t("js.settings.noWebhooks") + '</div>';
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
              <small class="text-body-secondary">${proviant.t("js.settings.webhookEvents", escapeHtml(eventsList))}</small>
            </div>
            <div class="form-check form-switch">
              <input class="form-check-input webhook-toggle-active" type="checkbox" ${toggleChecked} data-id="${wh.id}">
              <label class="form-check-label">${proviant.t("js.settings.webhookActive")}</label>
            </div>
          </div>
          <div class="btn-group btn-group-sm">
            <button type="button" class="btn btn-outline-primary btn-view-deliveries" data-id="${wh.id}">${proviant.t("js.settings.viewDeliveries")}</button>
            <button type="button" class="btn btn-outline-secondary btn-copy-secret" data-id="${wh.id}">${proviant.t("js.settings.copySecret")}</button>
            <button type="button" class="btn btn-outline-danger btn-delete-webhook" data-id="${wh.id}">${proviant.t("common.delete")}</button>
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
      const id = Number.parseInt(this.dataset.id);
      const wh = currentWebhooks.find(w => w.id === id);
      if (wh) {
        ToggleWebhookActive(id, !wh.active);
      }
    });
  });

  document.querySelectorAll('.btn-view-deliveries').forEach(function(btn) {
    btn.addEventListener('click', function() {
      const id = Number.parseInt(this.dataset.id);
      ShowWebhookDeliveries(id);
    });
  });

  document.querySelectorAll('.btn-copy-secret').forEach(function(btn) {
    btn.addEventListener('click', function() {
      const id = Number.parseInt(this.dataset.id);
      CopyWebhookSecret(id);
    });
  });

  document.querySelectorAll('.btn-delete-webhook').forEach(function(btn) {
    btn.addEventListener('click', function() {
      const id = Number.parseInt(this.dataset.id);
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
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.failedToUpdateWebhook", response.message));
      RenderWebhookList();
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

function ShowWebhookDeliveries(id) {
  proviant.getWebhookDeliveries(id).then((response) => {
    if (response.code === 200) {
      ShowDeliveriesModal(response.deliveries || []);
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.failedToLoadDeliveries", response.message));
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

function ShowDeliveriesModal(deliveries) {
  let html = '<div class="table-responsive"><table class="table table-sm"><thead><tr><th>' + proviant.t("js.settings.colTime") + '</th><th>' + proviant.t("js.settings.colAttempt") + '</th><th>' + proviant.t("js.settings.colStatus") + '</th><th>' + proviant.t("js.settings.colResponse") + '</th><th>' + proviant.t("common.error") + '</th></tr></thead><tbody>';

  if (deliveries.length === 0) {
    html += '<tr><td colspan="5" class="text-center text-body-secondary">' + proviant.t("js.settings.noDeliveries") + '</td></tr>';
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
    proviant.t("js.settings.deleteWebhookTitle"),
    proviant.t("js.settings.deleteWebhookConfirm"),
    function() {
      proviant.deleteWebhook(id).then((response) => {
        if (response.code === 200) {
          ShowSuccessModal(proviant.t("js.settings.webhookDeleted"), function() {
            LoadWebhooks();
          });
        } else {
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.failedToDeleteWebhook", response.message));
        }
      }).catch((err) => {
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
      });
    },
    proviant.t("common.delete"),
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
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.selectEvent"));
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
      ShowSuccessModal(proviant.t("js.settings.webhookCreated"), function() {
        LoadWebhooks();
      });
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.failedToCreateWebhook", response.message));
    }
  }).catch((err) => {
    setButtonLoading(btn, false);
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
  login_success: "js.settings.auditActionLoginSuccess",
  login_failure: "js.settings.auditActionLoginFailure",
  password_change: "js.settings.auditActionPasswordChange",
  password_change_failed: "js.settings.auditActionPasswordChangeFailed",
  member_added: "js.settings.auditActionMemberAdded",
  member_removed: "js.settings.auditActionMemberRemoved",
  member_left: "js.settings.auditActionMemberLeft",
  admin_changed: "js.settings.auditActionAdminChanged",
  account_deleted: "js.settings.auditActionAccountDeleted",
};

function renderAuditLogRow(log) {
  const iconClass = AUDIT_ACTION_ICONS[log.action] || "bi bi-shield text-secondary";
  const label = AUDIT_ACTION_LABELS[log.action] ? proviant.t(AUDIT_ACTION_LABELS[log.action]) : (log.action || proviant.t("js.settings.unknown"));
  const time = log.timestamp ? new Date(log.timestamp).toLocaleTimeString() : "";
  const userText = log.userId ? `#${log.userId}` : proviant.t("js.settings.system");
  const details = log.details ? escapeHtml(log.details) : "";

  return `
    <li class="list-group-item px-4 py-2">
      <div class="d-flex align-items-start gap-2">
        <i class="${iconClass}" aria-hidden="true" style="margin-top: 2px;"></i>
        <div class="flex-grow-1 min-w-0">
          <div class="d-flex justify-content-between align-items-start gap-2">
            <span class="fw-medium small">${label}</span>
            <small class="text-body-secondary flex-shrink-0" style="font-size: var(--text-xs);">${time}</small>
          </div>
          <div class="d-flex gap-2 mt-1">
            <small class="text-body-secondary" style="font-size: var(--text-xs);">${proviant.t("js.settings.userLabel", userText)}</small>
            ${log.ipAddress ? `<small class="text-body-secondary" style="font-size: var(--text-xs);">· ${escapeHtml(log.ipAddress)}</small>` : ""}
          </div>
          ${details ? `<small class="text-body-secondary d-block mt-1" style="font-size: var(--text-xs);">${details}</small>` : ""}
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
    list.innerHTML = '<li class="list-group-item text-center text-body-secondary py-4">' + proviant.t("js.settings.noActivity") + '</li>';
    if (meta) meta.textContent = "";
    return;
  }

  const rows = logs.map((log) => renderAuditLogRow(log)).join("");
  list.innerHTML = rows;
  if (meta) meta.textContent = proviant.tn("js.settings.auditMeta", logs.length);
}

function loadAuditLog(date) {
  const list = document.getElementById("auditLogList");
  const loading = document.getElementById("auditLogLoading");
  if (!list) return;

  if (loading) {
    list.innerHTML = `<li class="list-group-item text-center text-body-secondary py-4" id="auditLogLoading">
      <span class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
      <span>${proviant.t("common.loading")}</span>
    </li>`;
  }

  proviant.getAuditLogs(date).then((response) => {
    if (response.code === 200) {
      renderAuditLog(response.message);
    } else {
      list.innerHTML = `<li class="list-group-item text-danger text-center py-4">${proviant.t("js.settings.failedToLoadAuditLog")}</li>`;
      if (document.getElementById("auditLogMeta")) {
        document.getElementById("auditLogMeta").textContent = proviant.t("js.settings.errorWithMessage", response.message || proviant.t("js.settings.unknown"));
      }
    }
  }).catch(() => {
    list.innerHTML = `<li class="list-group-item text-danger text-center py-4">${proviant.t("js.settings.networkError")}</li>`;
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

  if (target.closest("#btnUpdateReceiptScanSettings")) {
    event.preventDefault();
    UpdateReceiptScanSettings();
    return;
  }

  if (target.closest("#btnResetReceiptScanSettings")) {
    event.preventDefault();
    ResetReceiptScanSettings();
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
    proviant.showConfirm(proviant.t("js.settings.cancelInvitationTitle"), proviant.t("js.settings.cancelInvitationConfirm"), function () {
      proviant.cancelInvitation(invitationID).then((response) => {
        if (response.code === 200) {
          const row = document.getElementById(`invitation-${invitationID}`);
          if (row) row.remove();
        } else {
          proviant.showFeedback('error', proviant.t("common.error"), response.message || proviant.t("js.settings.couldNotCancelInvitation"));
        }
      }).catch((err) => {
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
        proviant.showFeedback('success', proviant.t("js.settings.copiedTitle"), proviant.t("js.settings.secretCopied"));
      }
    }).catch((err) => {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
      proviant.t("js.settings.deleteLocationTitle"),
      proviant.t("js.settings.deleteLocationConfirm", btn.dataset.name),
      function () {
        deleteStorageLocation(btn.dataset.id);
      },
      proviant.t("common.delete"),
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
    void handleWebPushUnsubscribe();
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
        const minLen = Number.parseInt(target.dataset.minLength || "12", 10);
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
    case "inputReceiptScanEndpoint":
    case "inputReceiptScanTimeout":
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
    void handleWebPushToggle();
  }

  if (event.target.id === "toggleReceiptScanOverride") {
    toggleReceiptScanOverrideFields();
  }

});

/* ── Auto-load admin users on page load ─────────────────────────── */
document.addEventListener("DOMContentLoaded", function () {
  loadAdminUsers();
  loadPATs();
  loadCalendarTokenStatus();
  LoadWebhooks();
  void initWebPushNotifications();

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
      const minLen = Number.parseInt(target.dataset.minLength || "12", 10);
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
            expiryDate.textContent = proviant.t("household.expiresAt", expiryDateObj.toLocaleDateString());
          }
        }
      } else {
        noTokenEl.classList.remove("d-none");
        hasTokenEl.classList.add("d-none");
      }
    }
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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

      proviant.showFeedback('success', proviant.t("js.settings.calendarTokenCreatedTitle"), proviant.t("js.settings.calendarTokenCreated"));
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message || proviant.t("js.settings.failedToCreateCalendarToken")));
    }
  }).catch((err) => {
    setButtonLoading(btn, false);
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
  });
}

function handleCalendarRegenerate() {
  proviant.showConfirm(
    proviant.t("js.settings.regenerateTokenTitle"),
    proviant.t("js.settings.regenerateTokenConfirm"),
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
              expiryDate.textContent = proviant.t("household.expiresAt", expiryDateObj.toLocaleDateString());
            }
          }

          proviant.showFeedback('success', proviant.t("js.settings.tokenRegeneratedTitle"), proviant.t("js.settings.calendarTokenRegenerated"));
        } else {
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message || proviant.t("js.settings.failedToRegenerateCalendarToken")));
        }
      }).catch((err) => {
        setButtonLoading(btn, false);
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
      });
    },
    proviant.t("js.settings.regenerate"),
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
  }).catch((err) => {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
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
    proviant.t("js.settings.removeCalendarSyncTitle"),
    proviant.t("js.settings.removeCalendarSyncConfirm"),
    function () {
      proviant.deleteCalendarToken().then((response) => {
        if (response.code === 200) {
          const noTokenEl = document.getElementById("calendarNoToken");
          const hasTokenEl = document.getElementById("calendarHasToken");
          const urlInput = document.getElementById("calendarUrl");

          noTokenEl.classList.remove("d-none");
          hasTokenEl.classList.add("d-none");
          urlInput.value = "";

          proviant.showFeedback('success', proviant.t("js.settings.calendarSyncRemovedTitle"), proviant.t("js.settings.calendarTokenRemoved"));
        } else {
          proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.errorWithMessage", response.message || proviant.t("js.settings.failedToRemoveCalendarToken")));
        }
      }).catch((err) => {
        proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.networkErrorWithMessage", err.message));
      });
    },
    proviant.t("js.settings.remove"),
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
    if (titleEl)    titleEl.textContent = proviant.t("js.settings.editStorageLocation");
  } else {
    if (idInput)    idInput.value    = "";
    if (nameInput)  nameInput.value  = "";
    if (iconInput)  iconInput.value  = "";
    if (sortInput)  sortInput.value  = "0";
    if (titleEl)    titleEl.textContent = proviant.t("js.settings.addStorageLocation");
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
  const sortOrder = sortInput ? Number.parseInt(sortInput.value, 10) || 0 : 0;

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
            <button type="button" class="btn btn-outline-secondary btn-edit-sl"
                    data-id="${locId}" data-name="${locName}" data-icon="${locIcon}"
                    data-sortorder="${locSort}" aria-label="${proviant.t("household.editStorageLocationAria", locName)}">
              <i class="bi bi-pencil" aria-hidden="true"></i>
            </button>
            <button type="button" class="btn btn-danger btn-delete-sl"
                    data-id="${locId}" data-name="${locName}" aria-label="${proviant.t("household.deleteStorageLocationAria", locName)}">
              <i class="bi bi-trash" aria-hidden="true"></i>
            </button>
          </div>`;

        if (!existing) list.appendChild(li);

        proviant.showFeedback("success", isEdit ? proviant.t("js.settings.locationUpdated") : proviant.t("js.settings.locationAdded"),
          isEdit ? proviant.t("js.settings.locationUpdatedMessage", locName) : proviant.t("js.settings.locationAddedMessage", locName));
      } else {
        const msg = response.message || response.Message || proviant.t("js.settings.unknownError");
        proviant.showFeedback("error", proviant.t("common.error"), proviant.t("js.settings.failedToSaveLocation", msg));
      }
    })
    .catch((err) => {
      setButtonLoading(saveBtn, false);
      proviant.showFeedback("error", proviant.t("common.error"), proviant.t("js.settings.requestFailed", err));
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
        proviant.showFeedback("success", proviant.t("js.settings.locationDeleted"), proviant.t("js.settings.locationRemoved"));
      } else {
        const msg = response.message || response.Message || proviant.t("js.settings.unknownError");
        proviant.showFeedback("error", proviant.t("common.error"), proviant.t("js.settings.failedToDeleteLocation", msg));
      }
    })
    .catch((err) => {
      proviant.showFeedback("error", proviant.t("common.error"), proviant.t("js.settings.requestFailed", err));
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
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.pushInitFailed"));
      return;
    }
  }

  const permission = Notification.permission;
  if (permission === 'denied') {
    proviant.showFeedback('warning', proviant.t("js.settings.pushBlockedTitle"), proviant.t("js.settings.pushBlocked"));
    const toggle = document.getElementById("togglePushNotifications");
    if (toggle) toggle.checked = false;
    return;
  }

  if (permission !== 'granted') {
    const result = await Notification.requestPermission();
    if (result !== 'granted') {
      proviant.showFeedback('warning', proviant.t("js.settings.pushNotAllowedTitle"), proviant.t("js.settings.pushPermissionDenied"));
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
      proviant.showFeedback('success', proviant.t("js.settings.pushEnabledTitle"), proviant.t("js.settings.pushEnabled"));
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.pushSubscribeFailed"));
    }
  } catch (e) {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.pushSubscribeError", e.message));
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
      proviant.showFeedback('success', proviant.t("js.settings.pushDisabledTitle"), proviant.t("js.settings.pushDisabled"));
    } else {
      proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.pushDisableFailed"));
    }
  } catch (e) {
    proviant.showFeedback('error', proviant.t("common.error"), proviant.t("js.settings.unsubscribeFailed", e.message));
  }
}
