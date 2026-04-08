document.addEventListener("DOMContentLoaded", function () {
  const token = document.querySelector('meta[name="csrf_token"]')?.content;
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
    .then(function (data) {
      const container = document.querySelector(".card-body");
      container.innerHTML =
        '<div class="text-center mb-4">' +
        '<div class="d-inline-flex align-items-center justify-content-center rounded-3" style="width:64px;height:64px;background-color:#198754;">' +
        '<i class="bi bi-check-circle fs-1 text-white"></i>' +
        "</div>" +
        '<h3 class="mt-3 mb-1">Email Verified!</h3>' +
        "</div>" +
        '<div class="alert alert-success" role="alert">' +
        '<i class="bi bi-check-circle me-2"></i>Your email address has been verified successfully.' +
        "</div>" +
        '<div class="text-center mt-3">' +
        '<a href="/web/auth" class="btn btn-primary">Log In</a>' +
        "</div>";
    })
    .catch(function (error) {
      const container = document.querySelector(".card-body");
      container.innerHTML =
        '<div class="text-center mb-4">' +
        '<div class="d-inline-flex align-items-center justify-content-center rounded-3" style="width:64px;height:64px;background-color:#dc3545;">' +
        '<i class="bi bi-x-circle fs-1 text-white"></i>' +
        "</div>" +
        '<h3 class="mt-3 mb-1">Verification Failed</h3>' +
        "</div>" +
        '<div class="alert alert-danger" role="alert">' +
        '<i class="bi bi-exclamation-triangle me-2"></i>An error occurred while verifying your email.' +
        "</div>" +
        '<div class="text-center mt-3">' +
        '<a href="/web" class="btn btn-outline-primary">Go to Dashboard</a>' +
        "</div>";
    });
});
