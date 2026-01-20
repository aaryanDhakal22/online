function copyAndNotify(apiKeyInput) {
    // Copy the API key to the clipboard
    apiKeyInput.select();
    apiKeyInput.setSelectionRange(0, 99999);
    navigator.clipboard.writeText(apiKeyInput.value);
    window.getSelection().removeAllRanges();
    notify("Copied to clipboard !", "success");
}
function notify(message, type) {
    let color;
    if (type === "success") {
        color = "0a7a5a";
    } else if (type === "error") {
        color = "a50000";
    } else if (type === "warning") {
        color = "f5a623";
    }
    Toastify({
        text: message,
        duration: 3000,
        newWindow: true,
        gravity: "bottom",
        position: "right",
        stopOnFocus: true,
        style: {
            background: "#" + color,
            fontFamily: "'Instrument Serif', serif",
            color: "#fff",
            padding: "20px",
            borderRadius: "5px",
            boxShadow: "0 5px 20px rgba(0, 0, 0, 0.25)",
        },
    }).showToast();
}
document.addEventListener("DOMContentLoaded", function() {
    const generateButton = document.querySelector("#generate-button");
    const setButton = document.querySelector("#set-button");
    const password = document.querySelector("#password");

    setButton.addEventListener("click", function() {
        const api = fetch("http://localhost:1323/api/v1/set", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Access-Control-Allow-Origin": "*",
                "X-Admin-Passcode": password.value,
            }
        });
        api.then((response) => {
            console.log(response);
            if (response.ok) {
                return response.json();
            } else {
                throw new Error("Something went wrong");
                notify("Something went wrong !", "error");
            }
        }).then((data) => {
            console.log(data);
            const apiKey = data.key;
            const apiKeyInput = document.querySelector("#api-key-input");
            apiKeyInput.value = apiKey;
            console.log(apiKey);
            copyAndNotify(apiKeyInput);
        }).catch((error) => {
            console.error(error);
            notify("Something went wrong !", "error");
        });
    });
    generateButton.addEventListener("click", function() {
        const api = fetch("http://localhost:1323/api/v1/generate", {
            method: "GET",
            headers: {
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Access-Control-Allow-Origin": "*",
            },
        });
        api.then((response) => {
            console.log(response);
            if (response.ok) {
                return response.json();
            } else {
                throw new Error("Something went wrong");
                notify("Something went wrong !", "error");
            }
        }).then((data) => {
            console.log(data);
            const apiKey = data.key;
            const apiKeyInput = document.querySelector("#api-key-input");
            apiKeyInput.value = apiKey;
            console.log(apiKey);
            copyAndNotify(apiKeyInput);
        }).catch((error) => {
            console.error(error);
            notify("Something went wrong !", "error");
        });
    });
});
