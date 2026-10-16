// Possible routes:
// const apiUrl = "https://quicconline.rndhkl.com/api/v1";
const apiUrl = "/api/v1";

function copyAndNotify(apiKeyInput) {
    apiKeyInput.select();
    apiKeyInput.setSelectionRange(0, 99999);

    // navigator.clipboard is only available in secure contexts (HTTPS,
    // localhost, file://). Tailscale 100.x.x.x served over plain HTTP
    // doesn't qualify, so the property is undefined there. Feature-detect
    // and fall back to the legacy execCommand path, which predates the
    // secure-context requirement and still works.
    const useAsync = typeof navigator !== "undefined"
        && navigator.clipboard
        && typeof navigator.clipboard.writeText === "function";

    const finish = (ok) => {
        try { window.getSelection().removeAllRanges(); } catch (_) {}
        if (ok) {
            notify("Copied to clipboard !", "success");
        } else {
            notify("Copy not supported here — copy manually from the box above.", "warning");
        }
    };

    if (useAsync) {
        navigator.clipboard.writeText(apiKeyInput.value)
            .then(() => finish(true))
            .catch(() => fallbackCopy(apiKeyInput, finish));
    } else {
        fallbackCopy(apiKeyInput, finish);
    }
}

function fallbackCopy(apiKeyInput, done) {
    let ok = false;
    try {
        apiKeyInput.focus();
        apiKeyInput.select();
        ok = !!(document.execCommand && document.execCommand("copy"));
    } catch (_) {
        ok = false;
    }
    done(ok);
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
    const apiKeyInput = document.querySelector("#api-key-input");
    const ownKeyToggle = document.querySelector("#own-key-toggle");

    // Healthcheck on load
    fetch(apiUrl + "/healthz", {
        method: "GET",
        headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
        },
    })
        .then((response) => response.text())
        .then((data) => {
            if (data !== "OK") {
                notify("Server is down", "error");
            }
        })
        .catch((error) => {
            console.error(error);
            notify("Something went wrong !", "error");
        });

    // Toggle: checkbox swaps between "Generate then Set" and
    // "Paste then Set" flows. We keep the same Set button — its click
    // handler picks the right endpoint based on the checkbox state.
    function syncOwnKeyMode() {
        const own = ownKeyToggle.checked;
        apiKeyInput.readOnly = !own;
        apiKeyInput.value = "";
        if (own) {
            apiKeyInput.placeholder = "Paste your key here";
            generateButton.classList.add("hidden");
        } else {
            apiKeyInput.placeholder = "API Key";
            generateButton.classList.remove("hidden");
        }
    }
    ownKeyToggle.addEventListener("change", syncOwnKeyMode);

    // Generate (standard flow): GET /generate stages a Primed key.
    generateButton.addEventListener("click", function() {
        fetch(apiUrl + "/generate", {
            method: "GET",
            headers: {
                "Content-Type": "application/json",
                Accept: "application/json",
            },
        })
            .then((response) => {
                if (!response.ok) {
                    notify("Something went wrong !", "error");
                    throw new Error("generate failed");
                }
                return response.json();
            })
            .then((data) => {
                apiKeyInput.value = data.key;
                copyAndNotify(apiKeyInput);
            })
            .catch((error) => console.error(error));
    });

    // Set: branch on the toggle.
    //   - unchecked → GET /set, activates whatever's primed
    //   - checked   → POST /setKey with the pasted plaintext
    setButton.addEventListener("click", function() {
        if (!password.value) {
            notify("Passcode is required", "warning");
            return;
        }

        const own = ownKeyToggle.checked;
        let request;

        if (own) {
            const pasted = apiKeyInput.value.trim();
            if (!pasted) {
                notify("Paste a key first", "warning");
                return;
            }
            request = fetch(apiUrl + "/setKey", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Accept: "application/json",
                    "X-Admin-Passcode": password.value,
                },
                body: JSON.stringify({ key: pasted }),
            });
        } else {
            request = fetch(apiUrl + "/set", {
                method: "GET",
                headers: {
                    "Content-Type": "application/json",
                    Accept: "application/json",
                    "X-Admin-Passcode": password.value,
                },
            });
        }

        request
            .then((response) => {
                if (!response.ok) {
                    if (response.status === 401) {
                        notify("Invalid admin passcode", "error");
                    } else {
                        notify("Something went wrong !", "error");
                    }
                    throw new Error("set failed");
                }
                return response.json();
            })
            .then((data) => {
                apiKeyInput.value = data.key;
                copyAndNotify(apiKeyInput);
            })
            .catch((error) => console.error(error));
    });
});
