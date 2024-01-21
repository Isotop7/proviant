function Login() {
    let username = authForm.elements.inputUsername.value;
    let password = authForm.elements.inputPassword.value;
    expiro.loginUser(username, password).then((response) => {
        switch (response.status) {
            case 200:
                window.location.href = `${window.location.protocol}//${window.location.host}/web`
                break;
            case 401:
                console.log("Unauthorized");
            // TODO: Add html box and show result
            // TODO: Add optional signup method
                break;
            default:
                break
        }
    });
}

let authButton = document.getElementById("btnAuth")
let authForm = document.forms["authData"]

authButton.onclick = function (event) {
    event.preventDefault();
    Login();
}
authForm.onsubmit = function (event) {
    event.preventDefault();
    Login();
}