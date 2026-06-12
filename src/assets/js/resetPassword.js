document.addEventListener("DOMContentLoaded", function () {
  const form = document.getElementById("reset-password-form");
  if (!form) {
    return;
  }

  const tokenInput = document.getElementById("reset-token");
  const passwordInput = document.getElementById("reset-password");
  const confirmInput = document.getElementById("reset-password-confirm");
  const errorBox = document.getElementById("reset-password-error");
  const errorText = document.getElementById("reset-password-error-text");
  const confirmErrorBox = document.getElementById("reset-password-confirm-error");
  const confirmErrorText = document.getElementById("reset-password-confirm-error-text");
  const submitBtn = document.getElementById("btn-reset-submit");
  const submitLabel = document.getElementById("btn-reset-label");
  const submitIcon = document.getElementById("btn-reset-icon");
  const routeAuth = form.dataset.routeAuth || "/web/auth";

  const pwdToggleBtn = document.getElementById("pwd-toggle-btn");
  const pwdEyeIcon = document.getElementById("pwd-eye-icon");
  if (pwdToggleBtn) {
    pwdToggleBtn.addEventListener("click", function () {
      if (!passwordInput) {
        return;
      }
      const showing = passwordInput.type === "text";
      const nextType = showing ? "password" : "text";
      passwordInput.type = nextType;
      if (confirmInput) {
        confirmInput.type = nextType;
      }
      if (pwdEyeIcon) {
        pwdEyeIcon.className = showing ? "bi bi-eye" : "bi bi-eye-slash";
      }
      pwdToggleBtn.setAttribute("aria-pressed", String(!showing));
    });
  }

  function showError(box, text, message) {
    if (!box || !text) {
      return;
    }
    text.textContent = message;
    box.style.display = "flex";
  }

  function clearErrors() {
    if (errorBox) errorBox.style.display = "none";
    if (confirmErrorBox) confirmErrorBox.style.display = "none";
  }

  if (passwordInput) {
    passwordInput.addEventListener("input", function () {
      if (errorBox) errorBox.style.display = "none";
    });
  }
  if (confirmInput) {
    confirmInput.addEventListener("input", function () {
      if (confirmErrorBox) confirmErrorBox.style.display = "none";
    });
  }

  form.addEventListener("submit", function (event) {
    event.preventDefault();
    clearErrors();

    const token = (tokenInput?.value || "").trim();
    const password = passwordInput?.value || "";
    const confirm = confirmInput?.value || "";

    if (!password || password.length < 12) {
      showError(errorBox, errorText, "Password must be at least 12 characters long.");
      if (passwordInput) passwordInput.focus();
      return;
    }

    if (password !== confirm) {
      showError(confirmErrorBox, confirmErrorText, "Passwords do not match.");
      if (confirmInput) confirmInput.focus();
      return;
    }

    if (submitBtn) submitBtn.disabled = true;
    if (submitLabel) submitLabel.textContent = "Resetting...";
    if (submitIcon) submitIcon.className = "bi bi-hourglass-split";

    fetch("/auth/reset-password", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token: token, password: password }),
    })
      .then(function (response) {
        return response.json().then(function (data) {
          return { ok: response.ok, data: data };
        });
      })
      .then(function (result) {
        if (result.ok) {
          const formInner = document.querySelector(".login-form-inner");
          if (formInner) {
            formInner.innerHTML =
              '<div class="form-heading">' +
              '<div class="form-heading-title">Password reset</div>' +
              '<div class="form-heading-sub">You can now sign in with your new password.</div>' +
              "</div>" +
              '<div class="alert alert-success" role="alert">' +
              '<i class="bi bi-check-circle me-2"></i>' +
              (result.data && result.data.message
                ? result.data.message
                : "Your password has been reset successfully.") +
              "</div>" +
              '<div class="text-center mt-3">' +
              '<a href="' + routeAuth + '" class="btn btn-proviant-primary">Log In</a>' +
              "</div>";
          }
          return;
        }
        const message =
          result.data && result.data.message
            ? result.data.message
            : "Something went wrong. Please try again.";
        showError(errorBox, errorText, message);
        if (submitBtn) submitBtn.disabled = false;
        if (submitLabel) submitLabel.textContent = "Reset Password";
        if (submitIcon) submitIcon.className = "bi bi-check2";
      })
      .catch(function () {
        showError(errorBox, errorText, "Network error. Please try again.");
        if (submitBtn) submitBtn.disabled = false;
        if (submitLabel) submitLabel.textContent = "Reset Password";
        if (submitIcon) submitIcon.className = "bi bi-check2";
      });
  });
});
