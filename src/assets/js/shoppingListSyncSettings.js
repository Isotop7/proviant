"use strict";

(function () {
  function getProviderLabel(provider) {
    if (provider === "bring") return "Bring!";
    if (provider === "todoist") return "Todoist";
    return provider;
  }

  function renderIntegrationStatus(provider, data, errorMsg) {
    const statusEl = document.getElementById("integrationStatus-" + provider);
    if (!statusEl) return;
    statusEl.textContent = "";

    if (data && data.lastSyncAt) {
      const syncDiv = document.createElement("div");
      syncDiv.className = "text-secondary-custom";
      syncDiv.textContent = proviant.t("js.syncSettings.lastSynced", data.lastSyncAt);
      statusEl.appendChild(syncDiv);
    }

    const activeError = errorMsg || (data && data.lastError) || "";
    if (activeError) {
      const alertDiv = document.createElement("div");
      alertDiv.className = "alert alert-danger py-2 px-3 mt-2 mb-0 small";
      alertDiv.setAttribute("role", "alert");
      alertDiv.textContent = activeError;
      statusEl.appendChild(alertDiv);
    }
  }

  function updateTokenHint(provider, configured) {
    const hintEl = document.getElementById("tokenHint-" + provider);
    if (!hintEl) return;
    if (configured) {
      hintEl.textContent = proviant.t("js.syncSettings.tokenStored");
    } else {
      hintEl.textContent = proviant.t("js.syncSettings.noToken");
    }
  }

  function saveIntegration(provider, triggerBtn) {
    const toggleEl = document.getElementById("toggleIntegration-" + provider);
    const emailEl = document.getElementById("inputIntegrationEmail-" + provider);
    const listIdEl = document.getElementById("inputIntegrationListId-" + provider);
    const listNameEl = document.getElementById("inputIntegrationListName-" + provider);
    const tokenEl = document.getElementById("inputIntegrationToken-" + provider);
    const clearTokenEl = document.getElementById("clearIntegrationToken-" + provider);

    const enabled = toggleEl ? Boolean(toggleEl.checked) : false;
    const accountEmail = emailEl ? emailEl.value.trim() : "";
    const listId = listIdEl ? listIdEl.value.trim() : "";
    const listName = listNameEl ? listNameEl.value.trim() : "";
    const accessToken = tokenEl ? tokenEl.value.trim() : "";
    const clearAccessToken = clearTokenEl ? Boolean(clearTokenEl.checked) : false;

    if (triggerBtn) triggerBtn.disabled = true;

    const createPayload = {
      enabled: enabled,
      accountEmail: accountEmail,
      listId: listId,
      listName: listName,
      accessToken: clearAccessToken ? "" : accessToken,
      clearAccessToken: clearAccessToken,
    };

    fetch("/api/v1/household/integrations/" + encodeURIComponent(provider), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(createPayload),
    })
      .then(function (res) {
        return res.json().then(function (body) {
          return { ok: res.ok, status: res.status, body: body };
        });
      })
      .then(function (result) {
        if (!result.ok) {
          const msg = (result.body && result.body.message) || proviant.t("js.syncSettings.saveFailed");
          renderIntegrationStatus(provider, null, msg);
          if (typeof proviant !== "undefined" && proviant.showFeedback) {
            proviant.showFeedback("error", proviant.t("common.error"), msg);
          }
          return;
        }

        if (tokenEl) tokenEl.value = "";
        if (clearTokenEl) clearTokenEl.checked = false;
        updateTokenHint(provider, Boolean(result.body && result.body.accessTokenConfigured));
        renderIntegrationStatus(provider, result.body, "");
        if (typeof proviant !== "undefined" && proviant.showFeedback) {
          proviant.showFeedback("success", proviant.t("js.syncSettings.savedTitle"), proviant.t("js.syncSettings.settingsSaved", getProviderLabel(provider)));
        }
      })
      .catch(function () {
        if (typeof proviant !== "undefined" && proviant.showFeedback) {
          proviant.showFeedback("error", proviant.t("common.error"), proviant.t("js.syncSettings.saveRequestFailed"));
        }
      })
      .finally(function () {
        if (triggerBtn) triggerBtn.disabled = false;
      });
  }

  function testIntegration(provider, triggerBtn) {
    if (triggerBtn) triggerBtn.disabled = true;
    fetch("/api/v1/household/integrations/" + encodeURIComponent(provider) + "/test", {
      method: "POST",
    })
      .then(function (res) {
        return res.json().then(function (body) {
          return { ok: res.ok, body: body };
        });
      })
      .then(function (result) {
        if (result.ok) {
          renderIntegrationStatus(provider, null, "");
          if (typeof proviant !== "undefined" && proviant.showFeedback) {
            proviant.showFeedback("success", proviant.t("js.syncSettings.connectedTitle"), proviant.t("js.syncSettings.connectionSucceeded", getProviderLabel(provider)));
          }
        } else {
          const msg = (result.body && result.body.message) || proviant.t("js.syncSettings.connectionTestFailed");
          renderIntegrationStatus(provider, null, msg);
          if (typeof proviant !== "undefined" && proviant.showFeedback) {
            proviant.showFeedback("error", proviant.t("js.syncSettings.connectionFailedTitle"), msg);
          }
        }
      })
      .catch(function () {
        if (typeof proviant !== "undefined" && proviant.showFeedback) {
          proviant.showFeedback("error", proviant.t("js.syncSettings.connectionFailedTitle"), proviant.t("js.syncSettings.requestFailed"));
        }
      })
      .finally(function () {
        if (triggerBtn) triggerBtn.disabled = false;
      });
  }

  function syncIntegrationNow(provider, triggerBtn) {
    if (triggerBtn) triggerBtn.disabled = true;
    fetch("/api/v1/household/integrations/" + encodeURIComponent(provider) + "/sync", {
      method: "POST",
    })
      .then(function (res) {
        return res.json().then(function (body) {
          return { ok: res.ok, body: body };
        });
      })
      .then(function (result) {
        if (result.ok) {
          renderIntegrationStatus(provider, result.body, "");
          if (typeof proviant !== "undefined" && proviant.showFeedback) {
            proviant.showFeedback("success", proviant.t("js.syncSettings.syncedTitle"), proviant.t("js.syncSettings.syncSucceeded", getProviderLabel(provider)));
          }
        } else {
          const msg = (result.body && result.body.message) || proviant.t("js.syncSettings.syncFailed");
          renderIntegrationStatus(provider, null, msg);
          if (typeof proviant !== "undefined" && proviant.showFeedback) {
            proviant.showFeedback("error", proviant.t("js.syncSettings.syncFailedTitle"), msg);
          }
        }
      })
      .catch(function () {
        if (typeof proviant !== "undefined" && proviant.showFeedback) {
          proviant.showFeedback("error", proviant.t("js.syncSettings.syncFailedTitle"), proviant.t("js.syncSettings.requestFailed"));
        }
      })
      .finally(function () {
        if (triggerBtn) triggerBtn.disabled = false;
      });
  }

  function performDeleteIntegration(provider, triggerBtn) {
    if (triggerBtn) triggerBtn.disabled = true;
    fetch("/api/v1/household/integrations/" + encodeURIComponent(provider), {
      method: "DELETE",
    })
      .then(function (res) {
        return res.json().then(function (body) {
          return { ok: res.ok, status: res.status, body: body };
        });
      })
      .then(function (result) {
        if (result.ok || result.status === 404) {
          const toggleEl = document.getElementById("toggleIntegration-" + provider);
          const fieldsEl = document.getElementById("integrationFields-" + provider);
          const emailEl = document.getElementById("inputIntegrationEmail-" + provider);
          const listIdEl = document.getElementById("inputIntegrationListId-" + provider);
          const listNameEl = document.getElementById("inputIntegrationListName-" + provider);
          const tokenEl = document.getElementById("inputIntegrationToken-" + provider);
          if (toggleEl) toggleEl.checked = false;
          if (fieldsEl) fieldsEl.classList.add("d-none");
          if (emailEl) emailEl.value = "";
          if (listIdEl) listIdEl.value = "";
          if (listNameEl) listNameEl.value = "";
          if (tokenEl) tokenEl.value = "";
          updateTokenHint(provider, false);
          renderIntegrationStatus(provider, null, "");
          if (typeof proviant !== "undefined" && proviant.showFeedback) {
            proviant.showFeedback("success", proviant.t("js.syncSettings.disconnectedTitle"), proviant.t("js.syncSettings.integrationRemoved", getProviderLabel(provider)));
          }
        } else {
          const msg = (result.body && result.body.message) || proviant.t("js.syncSettings.disconnectFailed");
          if (typeof proviant !== "undefined" && proviant.showFeedback) {
            proviant.showFeedback("error", proviant.t("common.error"), msg);
          }
        }
      })
      .catch(function () {
        if (typeof proviant !== "undefined" && proviant.showFeedback) {
          proviant.showFeedback("error", proviant.t("common.error"), proviant.t("js.syncSettings.disconnectRequestFailed"));
        }
      })
      .finally(function () {
        if (triggerBtn) triggerBtn.disabled = false;
      });
  }

  function deleteIntegration(provider, triggerBtn) {
    if (typeof proviant !== "undefined" && typeof proviant.showConfirm === "function") {
      proviant.showConfirm(
        proviant.t("js.syncSettings.disconnectTitle", getProviderLabel(provider)),
        proviant.t("js.syncSettings.disconnectConfirmBody", getProviderLabel(provider)),
        function () {
          performDeleteIntegration(provider, triggerBtn);
        },
        proviant.t("js.syncSettings.disconnectConfirmButton"),
        "danger"
      );
      return;
    }
    performDeleteIntegration(provider, triggerBtn);
  }

  document.addEventListener("change", function (event) {
    const toggle = event.target.closest("[data-integration-toggle]");
    if (!toggle) return;
    const provider = toggle.dataset.integrationToggle;
    const fieldsEl = document.getElementById("integrationFields-" + provider);
    if (!fieldsEl) return;
    if (toggle.checked) {
      fieldsEl.classList.remove("d-none");
    } else {
      fieldsEl.classList.add("d-none");
    }
  });

  document.addEventListener("click", function (event) {
    const actionBtn = event.target.closest("[data-integration-action][data-provider]");
    if (!actionBtn) return;
    const action = actionBtn.dataset.integrationAction;
    const provider = actionBtn.dataset.provider;
    if (!provider) return;

    if (action === "save") {
      saveIntegration(provider, actionBtn);
    } else if (action === "test") {
      testIntegration(provider, actionBtn);
    } else if (action === "sync") {
      syncIntegrationNow(provider, actionBtn);
    } else if (action === "delete") {
      deleteIntegration(provider, actionBtn);
    }
  });
})();
