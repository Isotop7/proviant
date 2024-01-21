function showError(message) {
    loginAlert.innerText = message;
    loginAlert.style.display = "";
}

function hideError() {
    loginAlert.innerText = "";
    loginAlert.style.display = "none";
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
                showError("Authentication failed!");
                break;
            default:
                showError(`Undefined authentication error: ${response}`)
                break;
        }
    });
}

let authButton = document.getElementById("btnAuth")
let authForm = document.forms["authData"]
let usernameInput = authForm.elements.inputUsername;
let passwordInput = authForm.elements.inputPassword;
let loginAlert = document.getElementById("loginAlert");

usernameInput.oninput = function () {
    if (usernameInput.value.length > 0 && usernameInput.classList.contains("is-invalid")) {
        usernameInput.classList.toggle("is-invalid");
    }
    if (loginAlert.style.display == "") {
        hideError();
    }
}
passwordInput.oninput = function () {
    if (passwordInput.value.length > 0 && passwordInput.classList.contains("is-invalid")) {
        passwordInput.classList.toggle("is-invalid");
    }
    if (loginAlert.style.display == "") {
        hideError();
    }
}
authButton.onclick = function (event) {
    event.preventDefault();
    Login();
}
authForm.onsubmit = function (event) {
    event.preventDefault();
    Login();
}