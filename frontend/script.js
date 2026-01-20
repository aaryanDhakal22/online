document.addEventListener("DOMContentLoaded", function() {
    const generateButton = document.querySelector("#generate-button");

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
            }
        }).then((data) => {
            console.log(data);
            const apiKey = data.key;
            const apiKeyInput = document.querySelector("#api-key-input");
            apiKeyInput.value = apiKey;
            console.log(apiKey);
        }).catch((error) => {
            console.error(error);
        });
    });
});
