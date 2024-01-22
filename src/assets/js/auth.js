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

    expiro.loginUser(username, password).then((response) => {
        switch (response.status) {
            case 200:
                window.location.href = `${window.location.protocol}//${window.location.host}/web`;
                break;
            case 401:
                showLoginError("Authentication failed!");
                break;
            default:
                showLoginError(`Undefined authentication error: ${response}`)
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

    expiro.signupUser(username, mailAddress, password).then((response) => {
        switch (response.code) {
            case 200:
                console.log(response.body);
                // TODO: Add success info and button for reload
                break;
            default:
                showSignupError(`Signup error: ${response.body}`)
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

usernameInput.oninput = function () {
    if (usernameInput.value.length > 0 && usernameInput.classList.contains("is-invalid")) {
        usernameInput.classList.toggle("is-invalid");
    }
    if (loginAlert.style.display == "") {
        hideError();
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
        hideError();
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