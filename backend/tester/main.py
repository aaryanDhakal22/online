import requests

url = "http://localhost:1323/api/v1"

smaller_payload = {
    "id": "123456789",
    "name": "Aaryan",
}
larger_payload = {
    "order_id": "123456789",
    "product_id": "123456789",
    "customer_id": "123456789",
    "amount": 100,
    "currency": "USD",
    "status": "PENDING",
}

health_check = requests.get(url + "/healthz")
if health_check.status_code == 200:
    print("Server is up and running")
else:
    print("Server is down")

# Generate three times
new_key_1 = requests.get(url + "/generate")
new_key_2 = requests.get(url + "/generate")
new_key_3 = requests.get(url + "/generate")

# Use the key
use_key_1 = requests.get(url + "/use")
json_data = use_key_1.json()
key_1 = json_data["key"]
print(key_1)

# Create a new order with the key as bearer token
new_order_1 = requests.post(
    url + "/orders",
    headers={"Authorization": "Bearer " + key_1},
    json=smaller_payload,
)

# Confirm the order was created

print(new_order_1.text)
print(new_order_1.status_code)

# Regenerate the key and use it to create a new orders
new_key_4 = requests.get(url + "/generate")
use_key_2 = requests.get(url + "/use")
json_data = use_key_2.json()
key_2 = json_data["key"]
print(key_2)
new_order_2 = requests.post(
    url + "/orders",
    headers={"Authorization": "Bearer " + key_2},
    json=smaller_payload,
)
# Confirm the order was created

print(new_order_2.text)
print(new_order_2.status_code)

# Regenerate the key and use it to create a new orders
new_key_5 = requests.get(url + "/generate")
use_key_3 = requests.get(url + "/use")
json_data = use_key_3.json()
key_3 = json_data["key"]
print(key_2)
new_order_3 = requests.post(
    url + "/orders",
    # Intentionally using the wrong key to test the 401 error
    headers={"Authorization": "Bearer " + key_2},
    json=larger_payload,
)
# Confirm the order was not created

print(new_order_3.text)
print("This should be a 401 error: ", new_order_3.status_code)


# Verify with outdated key and then the new key by sending it in the header
verify_key_1 = requests.get(
    url + "/verify",
    headers={"Authorization": "Bearer " + key_1},
)
print(verify_key_1.status_code)

verify_key_2 = requests.get(
    url + "/verify",
    headers={"Authorization": "Bearer " + key_2},
)
print(verify_key_2.status_code)

verify_key_3 = requests.get(
    url + "/verify",
    headers={"Authorization": "Bearer " + key_3},
)
print(verify_key_3.status_code)
