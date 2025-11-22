function showLoginError(message) {
    loginAlert.innerText = message;
    loginAlert.style.display = "";
}

function hideLoginError() {
    loginAlert.innerText = "";
    loginAlert.style.display = "none";
}

function showSignupError(message) {
    signupAlert.innerText = message;
    signupAlert.style.display = "";
}

function hideSignupError() {
    signupAlert.innerText = "";
    signupAlert.style.display = "none";
}

function showSignupSuccess(username) {
    let toastBootstrap = bootstrap.Toast.getOrCreateInstance(infoToast)
    infoToast.getElementsByClassName("toast-body")[0].innerHTML = `Hello <span class="fw-bold">${username}</span>!<br><br>Your signup succeeded and you should be able to log in and use proviant`
    toastBootstrap.show()
}

function clearLoginInputs() {
    usernameInput.value = "";
    passwordInput.value = "";
}

function clearSignupInputs() {
    inputMailAddress.value = "";
}

function Login() {
    let formIsValid = true;
    let username = authForm.elements.inputUsername.value;
    let password = authForm.elements.inputPassword.value;

    if (username == "") {
        if (!authForm.elements.inputUsername.classList.contains("is-invalid")) {
            authForm.elements.inputUsername.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (password == "") {
        if (!authForm.elements.inputPassword.classList.contains("is-invalid")) {
            authForm.elements.inputPassword.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }

    if (!formIsValid) {
        return;
    }

    proviant.loginUser(username, password).then((response) => {
        switch (response.code) {
            case 200:
                globalThis.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/web`;
                break;
            case 401:
                showLoginError("Authentication failed!");
                clearLoginInputs();
                break;
            default:
                showLoginError(`Undefined authentication error: ${response.message}`)
                clearLoginInputs();
                break;
        }
    });
}

function Signup() {
    let formIsValid = true;
    let username = authForm.elements.inputUsername.value;
    let password = authForm.elements.inputPassword.value;
    let mailAddress = authForm.elements.inputMailAddress.value;

    if (username == "") {
        if (!authForm.elements.inputUsername.classList.contains("is-invalid")) {
            authForm.elements.inputUsername.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (password == "") {
        if (!authForm.elements.inputPassword.classList.contains("is-invalid")) {
            authForm.elements.inputPassword.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (mailAddress == "") {
        if (!authForm.elements.inputMailAddress.classList.contains("is-invalid")) {
            authForm.elements.inputMailAddress.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }

    if (!formIsValid) {
        return;
    }

    proviant.signupUser(username, mailAddress, password).then((response) => {
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

let btnAuth = document.getElementById("btnAuth")
let authForm = document.forms["authData"]
let usernameInput = authForm.elements.inputUsername;
let passwordInput = authForm.elements.inputPassword;
let loginAlert = document.getElementById("loginAlert");
let signupAlert = document.getElementById("signupAlert");
let btnSignup = document.getElementById("btnSignup");
let infoToast = document.getElementById("infoToast");

usernameInput.oninput = function () {
    if (usernameInput.value.length > 0 && usernameInput.classList.contains("is-invalid")) {
        usernameInput.classList.toggle("is-invalid");
    }
    if (loginAlert.style.display == "") {
        hideLoginError();
    }
    if (signupAlert.style.display == "") {
        hideSignupError();
    }
}
passwordInput.oninput = function () {
    if (passwordInput.value.length > 0 && passwordInput.classList.contains("is-invalid")) {
        passwordInput.classList.toggle("is-invalid");
    }
    if (loginAlert.style.display == "") {
        hideLoginError();
    }
    if (signupAlert.style.display == "") {
        hideSignupError();
    }
}
inputMailAddress.oninput = function () {
    if (inputMailAddress.value.length > 0 && inputMailAddress.classList.contains("is-invalid")) {
        inputMailAddress.classList.toggle("is-invalid");
    }
    if (signupAlert.style.display == "") {
        hideSignupError();
    }
}
btnAuth.onclick = function (event) {
    event.preventDefault();
    Login();
}
authForm.onsubmit = function (event) {
    event.preventDefault();
    Login();
}
btnSignup.onclick = function (event) {
    event.preventDefault();
    Signup();
}
btnSignup.onsubmit = function (event) {
    event.preventDefault();
    Signup();
}
