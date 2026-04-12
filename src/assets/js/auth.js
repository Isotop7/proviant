function showLoginError(message) {
    const loginAlert = document.getElementById("loginAlert");
    if (loginAlert) {
        document.getElementById("loginAlertMessage").textContent = message;
        loginAlert.classList.remove("d-none");
        loginAlert.classList.add("show");
    }
}

function hideLoginError() {
    const loginAlert = document.getElementById("loginAlert");
    if (loginAlert) {
        loginAlert.classList.remove("show");
        loginAlert.classList.add("d-none");
    }
}

function showPasswordRequirementsModal() {
    const modal = document.getElementById("passwordRequirementsModal");
    if (modal) {
        const modalBootstrap = new bootstrap.Modal(modal);
        modalBootstrap.show();
    }
}

function showEmailVerificationModal(email) {
    const modal = document.getElementById("emailVerificationModal");
    if (modal) {
        const addressEl = document.getElementById("emailVerificationAddress");
        if (addressEl) addressEl.textContent = email || "your email address";
        const modalBootstrap = new bootstrap.Modal(modal);
        modalBootstrap.show();
    }
}

function showSignupError(message) {
    const signupAlert = document.getElementById("signupAlert");
    if (signupAlert) {
        document.getElementById("signupAlertMessage").textContent = message;
        signupAlert.classList.remove("d-none");
        signupAlert.classList.add("show");
    }
}

function hideSignupError() {
    const signupAlert = document.getElementById("signupAlert");
    if (signupAlert) {
        signupAlert.classList.remove("show");
        signupAlert.classList.add("d-none");
    }
}

function showSignupSuccess(username) {
    // Show success message on the auth page — user must log in before accessing onboarding
    const infoToast = document.getElementById("infoToast");
    if (infoToast) {
        let toastBootstrap = bootstrap.Toast.getOrCreateInstance(infoToast);
        const toastBody = document.getElementById("infoToastBody");
        if (toastBody) {
            toastBody.textContent = `Hello ${username}! Your account was created. Please log in to continue.`;
        }
        toastBootstrap.show();
    }
    // Clear signup fields so the user sees the login form
    clearSignupInputs();
}

function clearLoginInputs() {
    const authForm = document.forms["authData"];
    if (authForm) {
        const usernameInput = authForm.elements.inputUsername;
        const passwordInput = authForm.elements.inputPassword;
        if (usernameInput) usernameInput.value = "";
        if (passwordInput) passwordInput.value = "";
    }
}

function clearSignupInputs() {
    const authForm = document.forms["authData"];
    if (authForm) {
        const inputMailAddress = authForm.elements.inputMailAddress;
        if (inputMailAddress) inputMailAddress.value = "";
    }
}

function Login() {
    const authForm = document.forms["authData"];
    if (!authForm) return;

    let formIsValid = true;
    const usernameInput = authForm.elements.inputUsername;
    const passwordInput = authForm.elements.inputPassword;

    if (!usernameInput || !passwordInput) return;

    const username = usernameInput.value;
    const password = passwordInput.value;

    if (username == "") {
        if (!usernameInput.classList.contains("is-invalid")) {
            usernameInput.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (password == "") {
        if (!passwordInput.classList.contains("is-invalid")) {
            passwordInput.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (password != "" && password.length < 12) {
        if (!passwordInput.classList.contains("is-invalid")) {
            passwordInput.classList.toggle("is-invalid");
        }
        formIsValid = false;
        showPasswordRequirementsModal();
        return;
    }

    if (!formIsValid) {
        return;
    }

    proviant.loginUser(username, password).then((response) => {
        switch (response.code) {
            case 200:
                proviant.getOnboardingState().then((stateResponse) => {
                    if (stateResponse.code === 200 && stateResponse.body && !stateResponse.body.onboardingCompleted) {
                        globalThis.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/web/onboarding`;
                    } else {
                        globalThis.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/web`;
                    }
                }).catch(() => {
                    globalThis.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/web`;
                });
                break;
            case 401:
                showLoginError("Authentication failed!");
                clearLoginInputs();
                break;
            case 403:
                if (response.body && response.body.toLowerCase().includes("verify")) {
                    showEmailVerificationModal(username);
                } else {
                    showLoginError(response.message || "Access denied.");
                }
                clearLoginInputs();
                break;
            default:
                showLoginError(`Undefined authentication error: ${response.message}`);
                clearLoginInputs();
                break;
        }
    });
}

function Signup() {
    const authForm = document.forms["authData"];
    if (!authForm) return;

    let formIsValid = true;
    const usernameInput = authForm.elements.inputUsername;
    const passwordInput = authForm.elements.inputPassword;
    const inputMailAddress = authForm.elements.inputMailAddress;

    if (!usernameInput || !passwordInput || !inputMailAddress) return;

    const username = usernameInput.value;
    const password = passwordInput.value;
    const mailAddress = inputMailAddress.value;
    const inviteTokenInput = authForm.elements.inviteToken;
    const inviteToken = inviteTokenInput ? inviteTokenInput.value : "";

    if (username == "") {
        if (!usernameInput.classList.contains("is-invalid")) {
            usernameInput.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (password == "") {
        if (!passwordInput.classList.contains("is-invalid")) {
            passwordInput.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (password != "" && password.length < 12) {
        if (!passwordInput.classList.contains("is-invalid")) {
            passwordInput.classList.toggle("is-invalid");
        }
        formIsValid = false;
        showPasswordRequirementsModal();
        return;
    }
    if (mailAddress == "") {
        if (!inputMailAddress.classList.contains("is-invalid")) {
            inputMailAddress.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }

    if (!formIsValid) {
        return;
    }

    proviant.signupUser(username, mailAddress, password, inviteToken).then((response) => {
        switch (response.code) {
            case 200:
                showSignupSuccess(username);
                clearSignupInputs();
                break;
            case 400:
                showSignupError(`Invalid user data: ${response.body}`);
                clearLoginInputs();
                clearSignupInputs();
                break;
            default:
                showSignupError(`Signup error: ${response.body}`);
                clearLoginInputs();
                clearSignupInputs();
                break;
        }
    });
}

/* Event delegation for all clicks */
document.addEventListener("click", function (event) {
    const target = event.target;

    // Login button
    if (target.closest("#btnAuth")) {
        event.preventDefault();
        Login();
        return;
    }

    // Signup button
    if (target.closest("#btnSignup")) {
        event.preventDefault();
        Signup();
        return;
    }

    // Logout button
    if (target.closest("#btnLogout")) {
        event.preventDefault();
        proviant.logoutUser().then(() => {
            window.location.href = '/web';
        });
        return;
    }
});

/* Event delegation for form submissions */
document.addEventListener("submit", function (event) {
    const target = event.target;

    // Auth form submission
    if (target.id === "authData" || target.name === "authData") {
        event.preventDefault();
        Login();
        return;
    }
});

/* Event delegation for input changes */
document.addEventListener("input", function (event) {
    const target = event.target;

    // Username input
    if (target.name === "inputUsername" || target.id === "inputUsername") {
        if (target.value.length > 0 && target.classList.contains("is-invalid")) {
            target.classList.toggle("is-invalid");
        }
        const loginAlert = document.getElementById("loginAlert");
        if (loginAlert && loginAlert.classList.contains("show")) {
            hideLoginError();
        }
        const signupAlert = document.getElementById("signupAlert");
        if (signupAlert && signupAlert.classList.contains("show")) {
            hideSignupError();
        }
        return;
    }

    // Password input
    if (target.name === "inputPassword" || target.id === "inputPassword") {
        if (target.value.length > 0 && target.classList.contains("is-invalid")) {
            target.classList.toggle("is-invalid");
        }
        const loginAlert = document.getElementById("loginAlert");
        if (loginAlert && loginAlert.classList.contains("show")) {
            hideLoginError();
        }
        const signupAlert = document.getElementById("signupAlert");
        if (signupAlert && signupAlert.classList.contains("show")) {
            hideSignupError();
        }
        return;
    }

    // Mail address input
    if (target.name === "inputMailAddress" || target.id === "inputMailAddress") {
        if (target.value.length > 0 && target.classList.contains("is-invalid")) {
            target.classList.toggle("is-invalid");
        }
        const signupAlert = document.getElementById("signupAlert");
        if (signupAlert && signupAlert.classList.contains("show")) {
            hideSignupError();
        }
        return;
    }
});