function showLoginError(message) {
    proviant.showFeedback("error", "Login Failed", message);
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

function validatePasswordRequirements(password) {
    const requirements = {
        length: password.length >= 8,
        upper: /[A-Z]/.test(password),
        digit: /[0-9]/.test(password),
        special: /[^A-Za-z0-9]/.test(password)
    };

    const elements = {
        length: document.getElementById("req-length"),
        upper: document.getElementById("req-upper"),
        digit: document.getElementById("req-digit"),
        special: document.getElementById("req-special")
    };

    for (const [key, satisfied] of Object.entries(requirements)) {
        const el = elements[key];
        if (!el) continue;
        const icon = el.querySelector("i");
        if (!icon) continue;
        icon.className = satisfied
            ? "bi bi-check-circle text-success me-1"
            : "bi bi-x-circle text-danger me-1";
        el.classList.toggle("text-success", satisfied);
        el.classList.toggle("text-danger", !satisfied);
    }
}

function showSignupError(message) {
    proviant.showFeedback("error", "Signup Failed", message);
}

function showSignupSuccess(mailAddress) {
    showEmailVerificationModal(mailAddress);
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
                showLoginError(response.body || "Invalid username or password.");
                clearLoginInputs();
                break;
            case 403:
                if (response.body?.toLowerCase().includes("verify")) {
                    showEmailVerificationModal(username);
                } else {
                    showLoginError(response.body || "Access denied.");
                }
                clearLoginInputs();
                break;
            case 429: {
                const mins = response.retryAfter ? Math.ceil(Number.parseInt(response.retryAfter, 10) / 60) : null;
                const s = mins !== 1 ? "s" : "";
                const lockMsg = mins
                    ? `Too many failed attempts. Account locked — try again in ${mins} minute${s}.`
                    : (response.body || "Too many failed attempts. Account is temporarily locked.");
                showLoginError(lockMsg);
                clearLoginInputs();
                break;
            }
            default:
                showLoginError(response.body || "Login failed. Please try again.");
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
                showSignupSuccess(mailAddress);
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
        return;
    }

    // Password input
    if (target.name === "inputPassword" || target.id === "inputPassword") {
        if (target.value.length > 0 && target.classList.contains("is-invalid")) {
            target.classList.toggle("is-invalid");
        }
        validatePasswordRequirements(target.value);
        return;
    }

    // Mail address input
    if (target.name === "inputMailAddress" || target.id === "inputMailAddress") {
        if (target.value.length > 0 && target.classList.contains("is-invalid")) {
            target.classList.toggle("is-invalid");
        }
        return;
    }
});
