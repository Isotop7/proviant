const proviant = {};

// Define a public method
proviant.debug = function () {
    console.log('Proviant loaded');
};

proviant.createProduct = async function (barcode, expireAt) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/products`
    let data = JSON.stringify({ barcode, expireAt })
    const apiCall = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: data
    })
    const body = await apiCall.json();
    let response = {
        code: apiCall.status,
        message: body.message
    }
    return response;
}

proviant.getProductsByBarcode = async function (barcode) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/products/byBarcode/${barcode}`
    const apiCall = await fetch(url, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json'
        }
    })
    const body = await apiCall.json();
    let response = {
        code: apiCall.status,
        message: body
    }
    return response;
}

proviant.editProduct = async function (product) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/products/${product.ID}`
    let data = JSON.stringify(product)
    const apiCall = await fetch(url, {
        method: 'PATCH',
        headers: {
            'Content-Type': 'application/json'
        },
        body: data
    })
    const body = await apiCall.json();
    let response = {
        code: apiCall.status,
        message: body.message
    }
    return response;
}

proviant.deleteProduct = async function (productID) {
    let url = `${window.location.protocol}//${window.location.host}/api/v1/products/${productID}`
    const apiCall = await fetch(url, {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json'
        }
    })
    const body = await apiCall.json();
    let response = {
        code: apiCall.status,
        message: body.message
    }
    return response;
}

proviant.loginUser = async function (username, password) {
    const url = `${window.location.protocol}//${window.location.host}/auth/login`
    const apiCall = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, password })
    });
    if (!apiCall.ok) {
        return {
            code: apiCall.status,
            body: "Error logging in"
        }
    }
    const body = await apiCall.json();
    const response = {
        code: apiCall.status,
        body: body.message
    }
    return response;
}

proviant.signupUser = async function (username, mailAddress, password) {
    const url = `${window.location.protocol}//${window.location.host}/auth/signup`
    const apiCall = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, mailAddress, password })
    });
    const body = await apiCall.json();
    const response = {
        code: apiCall.status,
        body: body.message
    }

    if (!apiCall.ok) {
        return {
            code: apiCall.status,
            body: `${body.message}`
        }
    }
    return response;
}

proviant.updateUser = async function (username, mailAddress) {
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

proviant.updateUserPassword = async function (username, password) {
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

proviant.logoutUser = function () {
    document.cookie = "jwt=; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT;"
}

proviant.formatDate = function (timestamp) {
    const date = new Date(timestamp);
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, "0"); // Months are 0-based
    const day = String(date.getDate()).padStart(2, "0");
    const hours = String(date.getHours()).padStart(2, "0");
    const minutes = String(date.getMinutes()).padStart(2, "0");
    const seconds = String(date.getSeconds()).padStart(2, "0");

    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}
