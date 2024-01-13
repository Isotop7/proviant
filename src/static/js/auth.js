async function LoginRequest() {
    let username =  authForm.elements.floatingInput.value;
    let password =  authForm.elements.floatingPassword.value;
    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, password })
    });
    return response.json();
};

function Login(event) {
    LoginRequest().then((response) => {
        if (response.code == 401) {
            console.log("Unauthorized");
            // TODO: Add html box and show result
            // TODO: Add optional signup method
        } else if (response.code == 200) {
            window.location.href = `${window.location.protocol}//${window.location.host}/web`
        } else {
            // TODO: Show unknown error in box
        }
    });
}

let authButton = document.querySelector("#auth")
let authForm = document.forms["authData"]
let url = `${window.location.protocol}//${window.location.host}/auth/login`

authButton.onclick = function(event) {
    event.preventDefault();
    Login();
}
authForm.onsubmit = function(event) {
    event.preventDefault();
    Login();
}