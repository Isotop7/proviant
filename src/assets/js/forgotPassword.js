document.addEventListener("DOMContentLoaded", function () {
  const form = document.getElementById("forgot-password-form");
  if (!form) {
    return;
  }

  const emailInput = document.getElementById("forgot-email");
  const errorBox = document.getElementById("forgot-email-error");
  const errorText = document.getElementById("forgot-email-error-text");
  const submitBtn = document.getElementById("btn-forgot-submit");
  const submitLabel = document.getElementById("btn-forgot-label");
  const submitIcon = document.getElementById("btn-forgot-icon");
  const routeAuth = form.dataset.routeAuth || "/web/auth";

  function showError(message) {
    if (!errorBox || !errorText) {
      return;
    }
    errorText.textContent = message;
    errorBox.style.display = "flex";
  }

  function clearError() {
    if (errorBox) {
      errorBox.style.display = "none";
    }
  }

  if (emailInput) {
    emailInput.addEventListener("input", clearError);
  }

  form.addEventListener("submit", function (event) {
    event.preventDefault();
    clearError();

    const mailAddress = (emailInput?.value || "").trim();
    if (!mailAddress) {
      showError("Please enter your email address.");
      if (emailInput) {
        emailInput.focus();
      }
      return;
    }

    if (submitBtn) {
      submitBtn.disabled = true;
    }
    if (submitLabel) {
      submitLabel.textContent = "Sending...";
    }
    if (submitIcon) {
      submitIcon.className = "bi bi-hourglass-split";
    }

    fetch("/auth/forgot-password", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mailAddress: mailAddress }),
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
              '<div class="form-heading-title">Check your email</div>' +
              '<div class="form-heading-sub">If an account with that email address exists, we just sent a reset link.</div>' +
              "</div>" +
              '<div class="alert alert-success" role="alert">' +
              '<i class="bi bi-check-circle me-2"></i>' +
              (result.data && result.data.message
                ? result.data.message
                : "If an account with that email address exists, a password reset link has been sent.") +
              "</div>" +
              '<div class="text-center mt-3">' +
              '<a href="' + routeAuth + '" class="btn btn-proviant-secondary">Back to Sign In</a>' +
              "</div>";
          }
          return;
        }
        const message =
          result.data && result.data.message
            ? result.data.message
            : "Something went wrong. Please try again.";
        showError(message);
        if (submitBtn) {
          submitBtn.disabled = false;
        }
        if (submitLabel) {
          submitLabel.textContent = "Send Reset Link";
        }
        if (submitIcon) {
          submitIcon.className = "bi bi-envelope";
        }
      })
      .catch(function () {
        showError("Network error. Please try again.");
        if (submitBtn) {
          submitBtn.disabled = false;
        }
        if (submitLabel) {
          submitLabel.textContent = "Send Reset Link";
        }
        if (submitIcon) {
          submitIcon.className = "bi bi-envelope";
        }
      });
  });
});
