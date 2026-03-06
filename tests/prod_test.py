import os

from client import ApiClient

client = ApiClient("api/v1")
client.load_env()

def prod_test():
     print("Running in prod mode")
    # Assume key is set in prod
    # Get the key 

def one_test():
    print("Sending one test order")
    # Assume key is set in prod
    password = os.getenv("PASSWORD")
    key = client.get(f"/key", headers={
        "X-Admin-Passcode": password
        }
    )
    key = key.json()["key"]
    print(key)

