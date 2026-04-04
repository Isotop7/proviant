/* Accept invitation page handler */
document.addEventListener("submit", function (event) {
  const target = event.target;
  if (target.id === "acceptInviteForm") {
    event.preventDefault();
    const tokenInput = document.getElementById("inviteToken");
    const token = tokenInput ? tokenInput.value : "";
    if (!token) return;

    proviant.acceptInvitation(token).then((response) => {
      if (response.code === 200) {
        proviant.showFeedback('success', 'Invitation Accepted', 'Welcome! You will be redirected to the dashboard.', function () {
          globalThis.location.href = "/web";
        });
        setTimeout(() => { globalThis.location.href = "/web"; }, 4000);
      } else {
        proviant.showFeedback('error', 'Could Not Accept', response.message || 'Unknown error');
      }
    });
  }
});
