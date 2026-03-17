# Test if server is running
import secrets
import random as rd
from client import ApiClient
from data import order_requests
from pprint import pprint

client = ApiClient("api/v1")
client.load_env()

response = client.get("/healthz")

if response.status_code == 200:
    print("Server is running")
else:
    print("Server is not running")

#### Testing Key service
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


# Testing key generation
def testing_key_generation():
    t = []
    for _ in range(10):
        rk = client.get("/generate")
        rpk = rk.json()
        key = rpk["key"]
        if key is not None:
            t.append(1)
        # print("Key Generated:",key)

    print("## Testing key generation ## ")
    if sum(t) == 10:
        print("All keys generated")
    else:
        print("Not all keys generated")


# Testing setting key
def testing_key_setting():
    rk1 = client.get("/generate")
    rpk1 = rk1.json()
    key1 = rpk1["key"]

    rpset = client.get(
        f"/set", headers={"X-Admin-Passcode": "KhawarGhafoor931TaylorAvenue"}
    )

    if rpset.status_code == 200:
        print("Key set")
    else:
        print("Key not set")

    rp_verify = client.get(f"/verify", headers={"Authorization": f"Bearer {key1}"})
    rp_json = rp_verify.json()
    print(rp_json)
    match = rp_verify.json()["match"]
    print("Key match:", match)


# Testing setting key with 2 generations and verifying
def testing_key_setting_with_2_generations():
    rk2 = client.get("/generate")

    rpk2 = rk2.json()
    key2 = rpk2["key"]
    rk3 = client.get("/generate")
    rpk3 = rk3.json()
    key3 = rpk3["key"]
    rpset = client.get(
        f"/set", headers={"X-Admin-Passcode": "KhawarGhafoor931TaylorAvenue"}
    )
    if rpset.status_code == 200:
        print("Key set")
    else:
        print("Key not set")

    print("Verifying key")

    rp_verify = client.get(f"/verify", headers={"Authorization": f"Bearer {key2}"})

    print(rp_verify)
    rp_json = rp_verify.json()
    print(rp_json)
    match = rp_verify.json()["match"]
    print("Key match:", match, "(should be false)")

    rp_verify = client.get(f"/verify", headers={"Authorization": f"Bearer {key3}"})
    rp_json = rp_verify.json()
    print(rp_json)
    match = rp_verify.json()["match"]
    print("Key match:", match, "(should be true)")


def testing_custom_key_setting(key):
    rpset = client.post(
        f"/setKey",
        headers={"X-Admin-Passcode": "KhawarGhafoor931TaylorAvenue"},
        json_data={"Key": key},
    )
    print("Status code:", rpset.status_code)
    rp_get_key = client.get(
        f"/getKey", headers={"X-Admin-Passcode": "KhawarGhafoor931TaylorAvenue"}
    )
    print("Status code:", rp_get_key.status_code)
    server_key = rp_get_key.json()["key"]
    print("Server key:", server_key)
    if server_key == key:
        print("Key match")
    else:
        print("Key mismatch")


# testing_key_generation()
# testing_key_setting()
# testing_key_setting_with_2_generations()


def get_and_set_key():
    rk = client.get("/generate")
    rpk = rk.json()
    key = rpk["key"]
    rpset = client.get(
        f"/set", headers={"X-Admin-Passcode": "KhawarGhafoor931TaylorAvenue"}
    )
    if rpset.status_code == 200:
        print("Key set")
        print("Key:", key)
    else:
        print("Key not set")
        return
    verify = client.get(f"/verify", headers={"Authorization": f"Bearer {key}"})
    if verify.status_code == 200 and verify.json()["match"]:
        print("Key match")
        return key
    else:
        print("Unable to verify key")
        return None


def send_one_order_with_key_reset(order):
    key = get_and_set_key()
    if key is None:
        print("Key not set")
        return

    order["order_id"] = rd.randint(1000, 9999)
    # pprint(order)

    response = client.post(
        "/order",
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        json_data=order,
    )

    # print(response)
    print(response.status_code)
    print(response.text)


def testing_key_getter():
    key = client.get(
        f"/getKey", headers={"X-Admin-Passcode": "KhawarGhafoor931TaylorAvenue"}
    )
    if key.status_code == 200:
        print("Key retrieved")
        print(key.json())

    else:
        print("Key not retrieved")


def testing_reprint():
    print("Testing reprint")
    rp = client.get(
        "/reprint/latest", headers={"X-Admin-Passcode": "KhawarGhafoor931TaylorAvenue"}
    )
    if rp.status_code == 200:
        print("Reprint successful")
        print(rp.text)
    else:
        print("Reprint failed")


def send_one_order_with_key_set(order):
    key = client.get(
        f"/getKey", headers={"X-Admin-Passcode": "KhawarGhafoor931TaylorAvenue"}
    )
    key = key.json()["key"]
    print(key)
    order["order_id"] = rd.randint(1000, 9999)
    # pprint(order)

    response = client.post(
        "/order",
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
        json_data=order,
    )

    # print(response)
    print(response.status_code)
    print(response.text)


while True:
    # Add a large ascii art banner with the name of the env prod or dev
    # =============================================================================
    if client.env == "dev":
        print(dev_art)
    else:
        print(prod_art)

    print("Tests : ")
    print("1. Send one order with key reset")
    print("2. Send one order with key already set")
    print("3. Test key getter")
    print("4. Test key generation")
    print("5. Test key setting")
    print("6. Test key setting with 2 generations")
    print("7. Test custom key setting")
    print("8. Test reprint")
    type = input("Enter : ")

    match type:
        case "1":
            sure = input("Are you sure? [y/n]")
            if sure == "y":
                send_one_order_with_key_reset(order_requests["basic_pickup_order"])
            else:
                print("Cancelled")
        case "2":
            send_one_order_with_key_set(order_requests["delivery_with_address"])
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
        case "8":
            testing_reprint()
        case _:
            print("Invalid test")
    print("\n\n\n")
