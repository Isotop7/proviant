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
        proviant.showFeedback('success', proviant.t('js.auth.invitationAccepted'), proviant.t('js.auth.invitationAcceptedMessage'), function () {
          globalThis.location.href = "/web";
        });
        setTimeout(() => { globalThis.location.href = "/web"; }, 4000);
      } else {
        proviant.showFeedback('error', proviant.t('js.auth.invitationAcceptFailed'), response.message || proviant.t('js.auth.unknownError'));
      }
    });
  }
});
