"""Interactive harness for online/. Useful for poking the dev stack by hand;
for CI use the pytest suite in test_api.py instead.
"""

import os
import random as rd
import secrets
import time

from client import ApiClient
from data import order_requests

client = ApiClient("api/v1")
client.load_env()

ADMIN_PASSCODE = os.getenv("ADMIN_PASSCODE") or os.getenv("ADMIN_PASS")
if not ADMIN_PASSCODE:
    raise RuntimeError(
        "ADMIN_PASSCODE not set. Run via `task test-script` (Doppler injects it) "
        "or export ADMIN_PASSCODE before invoking this script."
    )

response = client.get("/healthz")
print("server up" if response.status_code == 200 else "server down")

dev_art = """
    =========================
    ██████╗ ███████╗██╗   ██╗
    ██╔══██╗██╔════╝██║   ██║
    ██║  ██║█████╗  ██║   ██║
    ██║  ██║██╔══╝  ╚██╗ ██╔╝
    ██████╔╝███████╗ ╚████╔╝
    ╚═════╝ ╚══════╝  ╚═══╝
    =========================
    """

prod_art = """
    =========================
    ██████╗ ██████╗  ██████╗ ██████╗
    ██╔══██╗██╔══██╗██╔═══██╗██╔══██╗
    ██████╔╝██████╔╝██║   ██║██║  ██║
    ██╔═══╝ ██╔══██╗██║   ██║██║  ██║
    ██║     ██║  ██║╚██████╔╝██████╔╝
    ╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚═════╝
    =========================
    """


def testing_key_generation():
    successes = 0
    for _ in range(10):
        rk = client.get("/generate")
        if rk.status_code == 200 and rk.json().get("key"):
            successes += 1
    print(f"## key generation: {successes}/10 succeeded")


def testing_key_setting():
    rk1 = client.get("/generate")
    key1 = rk1.json()["key"]
    rpset = client.get("/set", headers={"X-Admin-Passcode": ADMIN_PASSCODE})
    print("set:", "OK" if rpset.status_code == 200 else f"FAIL {rpset.status_code}")
    rp_verify = client.get("/verify", headers={"Authorization": f"Bearer {key1}"})
    print("verify:", rp_verify.json())


def testing_key_setting_with_2_generations():
    # The point of this test: a second /generate unprimes the first. After
    # /set, only the most recent key should verify — the earlier one must
    # be rejected.
    rk2 = client.get("/generate")
    key2 = rk2.json()["key"]
    rk3 = client.get("/generate")
    key3 = rk3.json()["key"]
    rpset = client.get("/set", headers={"X-Admin-Passcode": ADMIN_PASSCODE})
    print("set:", "OK" if rpset.status_code == 200 else f"FAIL {rpset.status_code}")

    rp_verify = client.get("/verify", headers={"Authorization": f"Bearer {key2}"})
    print("verify(earlier):", rp_verify.json(), "(should be false)")

    rp_verify = client.get("/verify", headers={"Authorization": f"Bearer {key3}"})
    print("verify(latest):", rp_verify.json(), "(should be true)")


def testing_custom_key_setting(key):
    rpset = client.post(
        "/setKey",
        headers={"X-Admin-Passcode": ADMIN_PASSCODE},
        json_data={"key": key},
    )
    print("setKey status:", rpset.status_code)
    rp = client.get("/getKey", headers={"X-Admin-Passcode": ADMIN_PASSCODE})
    server_key = rp.json().get("key")
    print("server key match:", server_key == key)


def get_and_set_key():
    rk = client.get("/generate")
    key = rk.json()["key"]
    rpset = client.get("/set", headers={"X-Admin-Passcode": ADMIN_PASSCODE})
    if rpset.status_code != 200:
        print("Key not set")
        return None
    verify = client.get("/verify", headers={"Authorization": f"Bearer {key}"})
    if verify.status_code == 200 and verify.json().get("match"):
        print("Key set:", key)
        return key
    print("Unable to verify key")
    return None


def fresh_order_id() -> int:
    return int(time.time() * 1000) * 1000 + rd.randint(0, 999)


def send_one_order_with_key_reset(order):
    key = get_and_set_key()
    if key is None:
        return
    order["order_id"] = fresh_order_id()
    response = client.post(
        "/order",
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        json_data=order,
    )
    print(response.status_code, response.text)


def send_one_order_with_key_set(order):
    rp = client.get("/getKey", headers={"X-Admin-Passcode": ADMIN_PASSCODE})
    key = rp.json()["key"]
    order["order_id"] = fresh_order_id()
    response = client.post(
        "/order",
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        json_data=order,
    )
    print(response.status_code, response.text)


def testing_key_getter():
    rp = client.get("/getKey", headers={"X-Admin-Passcode": ADMIN_PASSCODE})
    if rp.status_code != 200:
        print("Key not retrieved:", rp.status_code, rp.text)
        return
    print("Key retrieved:", rp.json())


while True:
    print(dev_art if client.env == "dev" else prod_art)
    print("Tests : ")
    print("1. Send one order with key reset")
    print("2. Send one order with key already set")
    print("3. Test key getter")
    print("4. Test key generation")
    print("5. Test key setting")
    print("6. Test key setting with 2 generations")
    print("7. Test custom key setting")
    type_ = input("Enter : ")

    match type_:
        case "1":
            sure = input("Are you sure? [y/n] ")
            if sure == "y":
                send_one_order_with_key_reset(order_requests["basic_pickup_order"])
            else:
                print("Cancelled")
        case "2":
            send_one_order_with_key_set(order_requests["basic_aaryan_order"])
        case "3":
            testing_key_getter()
        case "4":
            testing_key_generation()
        case "5":
            testing_key_setting()
        case "6":
            testing_key_setting_with_2_generations()
        case "7":
            input_key = input("Enter key: ")
            if len(input_key) < 5:
                input_key = secrets.token_urlsafe(32)
                print(f"Using random key: {input_key}")
            testing_custom_key_setting(input_key)
        case _:
            print("Invalid test")
    print("\n\n\n")
