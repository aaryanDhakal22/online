import os

from client import ApiClient

client = ApiClient("api/v1")
client.load_env()


def prod_test():
    print("Running in prod mode")


def one_test():
    print("Sending one test order")
    passcode = os.getenv("ADMIN_PASSCODE") or os.getenv("ADMIN_PASS")
    if not passcode:
        raise RuntimeError("ADMIN_PASSCODE not set in env")
    resp = client.get("/getKey", headers={"X-Admin-Passcode": passcode})
    if resp.status_code != 200:
        raise RuntimeError(f"getKey failed: {resp.status_code} {resp.text}")
    key = resp.json()["key"]
    print(f"active key (first 6 chars): {key[:6]}…")
