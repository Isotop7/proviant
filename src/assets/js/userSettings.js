function showUpdateError(message) {
    updateAlert.innerText = message;
    updateAlert.style.display = "";
}

function hideUpdateError() {
    updateAlert.innerText = "";
    updateAlert.style.display = "none";
}

function showPasswordError(message) {
    passwordAlert.innerText = message;
    passwordAlert.style.display = "";
}

function hidePasswordError() {
    passwordAlert.innerText = "";
    passwordAlert.style.display = "none";
}

function ShowSuccessModal(message, btnFunction) {
    modalBody.innerText = message;
    document.getElementById('btnModal').onclick = btnFunction;
    let successModal = new bootstrap.Modal(document.getElementById('modal'));
    successModal.show();
}

/* Update user settings */
function UpdateSettings() {
    let formIsValid = true;
    let inputUsername = document.getElementById('inputUsername');
    let inputMailAddress = document.getElementById('inputMailAddress');
    let username = inputUsername.value;
    let mailAddress = inputMailAddress.value;

    if (username == "") {
        if (!inputUsername.classList.contains("is-invalid")) {
            inputUsername.classList.toggle("is-invalid");
        }
        formIsValid = false;
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

    expiro.updateUser(username, mailAddress).then((response) => {
        switch (response.code) {
            case 200:
                console.error(response.message)
                ShowSuccessModal("User update complete. Please reload page to show changed values.", function (event) {
                    event.preventDefault();
                    location.reload();
                });
                break;
            case 401:
                console.error(response.message)
                showUpdateError(`Update failed: ${response.message}`);
                break;
            default:
                console.error(response.message)
                showUpdateError(`Undefined update error: ${response.message}`)
                break;
        }
    });
}

/* Update password */
function UpdatePassword() {
    let formIsValid = true;
    let inputPassword = document.getElementById('inputPassword');
    let inputPasswordVerification = document.getElementById('inputPasswordVerification');
    let password = inputPassword.value;
    let passwordVerification = inputPasswordVerification.value;
    let inputUsername = document.getElementById('inputUsername');
    let username = inputUsername.value;

    if (username == "") {
        if (!inputUsername.classList.contains("is-invalid")) {
            inputUsername.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (password == "") {
        if (!inputPassword.classList.contains("is-invalid")) {
            inputPassword.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }
    if (passwordVerification == "") {
        if (!inputPasswordVerification.classList.contains("is-invalid")) {
            inputPasswordVerification.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }

    if (password != passwordVerification) {
        if (!inputPassword.classList.contains("is-invalid")) {
            inputPassword.classList.toggle("is-invalid");
        }
        if (!inputPasswordVerification.classList.contains("is-invalid")) {
            inputPasswordVerification.classList.toggle("is-invalid");
        }
        formIsValid = false;
    }

    if (!formIsValid) {
        return;
    }

    expiro.updateUserPassword(username, password).then((response) => {
        switch (response.code) {
            case 200:
                console.error(response.message)
                ShowSuccessModal("Password update complete. Please log out to use changed password.", function (event) {
                    event.preventDefault();
                    expiro.logoutUser();
                    location.reload();
                })
                break;
            case 401:
                console.error(response.message)
                showPasswordError(`Update failed: ${response.message}`);
                break;
            default:
                console.error(response.message)
                showPasswordError(`Undefined update error: ${response.message}`)
                break;
        }
    });
}

/* Inputs */
let inputMailAddress = document.getElementById('inputMailAddress');
let inputUsername = document.getElementById('inputUsername');
let inputPassword = document.getElementById('inputPassword');
let inputPasswordVerification = document.getElementById('inputPasswordVerification');
/* Alert boxes */
let updateAlert = document.getElementById('updateAlert');
let passwordAlert = document.getElementById('passwordAlert');
/* Buttons */
let btnUpdatePersonalDetails = document.getElementById('btnUpdatePersonalDetails');
let btnUpdatePassword = document.getElementById('btnUpdatePassword');
let btnModal = document.getElementById('btnModal');
/* Modal */
let modalBody = document.getElementById('modalBody');

/* Input change handlers */
inputMailAddress.oninput = function () {
    if (inputMailAddress.value.length > 0 && inputMailAddress.classList.contains("is-invalid")) {
        inputMailAddress.classList.toggle("is-invalid");
    }
    if (updateAlert.style.display == "") {
        hideUpdateError();
    }
}
inputUsername.oninput = function () {
    if (inputUsername.value.length > 0 && inputUsername.classList.contains("is-invalid")) {
        inputUsername.classList.toggle("is-invalid");
    }
    if (updateAlert.style.display == "") {
        hideUpdateError();
    }
}
inputPassword.oninput = function () {
    if (inputPassword.value.length > 0 && inputPassword.classList.contains("is-invalid")) {
        inputPassword.classList.toggle("is-invalid");
    }
    if (updateAlert.style.display == "") {
        hidePasswordError();
    }
}
inputPasswordVerification.oninput = function () {
    if (inputPasswordVerification.value.length > 0 && inputPasswordVerification.classList.contains("is-invalid")) {
        inputPasswordVerification.classList.toggle("is-invalid");
    }
    if (updateAlert.style.display == "") {
        hidePasswordError();
    }
}

/* Button handlers */
btnUpdatePersonalDetails.onclick = function (event) {
    event.preventDefault();
    UpdateSettings();
}
btnUpdatePersonalDetails.onsubmit = function (event) {
    event.preventDefault();
    UpdateSettings();
}
btnUpdatePassword.onclick = function (event) {
    event.preventDefault();
    UpdatePassword();
}
btnUpdatePassword.onsubmit = function (event) {
    event.preventDefault();
    UpdatePassword();
}