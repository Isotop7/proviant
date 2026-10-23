document.addEventListener("DOMContentLoaded", function () {
  const urlParams = new URLSearchParams(window.location.search);
  const verifyToken = urlParams.get("token");

  if (!verifyToken) {
    return;
  }

  fetch("/auth/verify-email?token=" + encodeURIComponent(verifyToken), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then(function (response) {
      return response.json();
    })
    .then(function () {
      const container = document.querySelector(".card-body");
      container.innerHTML =
        '<div class="text-center mb-7">' +
        '<div class="d-inline-flex align-items-center justify-content-center rounded-5" style="width:64px;height:64px;background-color:var(--status-fresh);">' +
        '<i class="bi bi-check-circle fs-4xl fg-white"></i>' +
        "</div>" +
        '<h3 class="mt-5 mb-1">' + proviant.t("js.auth.emailVerifiedTitle") + "</h3>" +
        "</div>" +
        '<div class="alert alert theme-success" role="alert">' +
        '<i class="bi bi-check-circle me-2"></i>' + proviant.t("js.auth.emailVerifiedMessage") +
        "</div>" +
        '<div class="text-center mt-5">' +
        '<a href="/web/auth" class="btn btn-solid theme-primary">' + proviant.t("auth.logIn") + "</a>" +
        "</div>";
    })
    .catch(function () {
      const container = document.querySelector(".card-body");
      container.innerHTML =
        '<div class="text-center mb-7">' +
        '<div class="d-inline-flex align-items-center justify-content-center rounded-5" style="width:64px;height:64px;background-color:var(--status-expired);">' +
        '<i class="bi bi-x-circle fs-4xl fg-white"></i>' +
        "</div>" +
        '<h3 class="mt-5 mb-1">' + proviant.t("js.auth.verificationFailedTitle") + "</h3>" +
        "</div>" +
        '<div class="alert alert theme-danger" role="alert">' +
        '<i class="bi bi-exclamation-triangle me-2"></i>' + proviant.t("js.auth.verificationFailedMessage") +
        "</div>" +
        '<div class="text-center mt-5">' +
        '<a href="/web" class="btn btn-outline theme-primary">' + proviant.t("auth.goToDashboard") + "</a>" +
        "</div>";
    });
});
