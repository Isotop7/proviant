const expiro = {};

// Define a public method
expiro.debug = function () {
    console.log('Expiro loaded');
};

expiro.createProduct = async function (barcode, expireAt) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/products`
    let data = JSON.stringify({ barcode, expireAt })
    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: data
    })
    return response;
}

expiro.deleteProduct = async function (productID) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/products/${productID}`
    const response = await fetch(url, {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json'
        }
    })
    return response;
}

expiro.loginUser = async function (username, password) {
    let url = `${window.location.protocol}//${window.location.host}/auth/login`
    const apiCall = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, password })
    });
    let body = await apiCall.json();
    let response = {
        code: apiCall.status,
        body: body.message
    }
    return response;
}

expiro.signupUser = async function (username, mailAddress, password) {
    let url = `${window.location.protocol}//${window.location.host}/auth/signup`
    const apiCall = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, mailAddress, password })
    });

    const body = await apiCall.json();
    let response = {
        code: apiCall.status,
        body: body.message
    }
    return response;
}

expiro.updateUser = async function (username, mailAddress) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/user`
    const apiCall = await fetch(url, {
        method: 'PATCH',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, mailAddress })
    });

    const body = await apiCall.json();
    let response = {
        code: apiCall.status,
        message: body.message
    }
    return response;
}

expiro.updateUserPassword = async function (username, password) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/user/password`
    const apiCall = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, password })
    });

    const body = await apiCall.json();
    let response = {
        code: apiCall.status,
        message: body.message
    }
    return response;
}

expiro.logoutUser = function () {
    document.cookie = "jwt=; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT;"
}