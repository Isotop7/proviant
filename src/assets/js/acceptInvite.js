/* Accept invitation page handler */
document.addEventListener("submit", function (event) {
  const target = event.target;
  if (target.id === "acceptInviteForm") {
    event.preventDefault();
    const tokenInput = document.getElementById("inviteToken");
    const token = tokenInput ? tokenInput.value : "";
    if (!token) return;

    proviant.acceptInvitation(token).then((response) => {
      const alertEl = document.getElementById("acceptAlert");
      if (response.code === 200) {
        if (alertEl) {
          alertEl.textContent = "Invitation accepted! Redirecting...";
          alertEl.className = "alert alert-success mt-3";
        }
        setTimeout(() => {
          globalThis.location.href = "/web";
        }, 1500);
      } else {
        if (alertEl) {
          alertEl.textContent = `Error: ${response.message}`;
          alertEl.className = "alert alert-danger mt-3";
        }
      }
    });
  }
});
