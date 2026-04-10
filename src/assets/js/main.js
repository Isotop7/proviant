/* Event delegation for logout button */
document.addEventListener("click", function (event) {
    const target = event.target;

    // Logout button
    if (target.closest("#btnLogout")) {
        event.preventDefault();
        console.log('Logout clicked');
        proviant.logoutUser().then(() => {
            console.log('Logout completed, redirecting');
            window.location.href = '/web';
        });
        return;
    }
});