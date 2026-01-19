import requests

url = "http://localhost:1323/api/v1"

health_check = requests.get(url + "/healthz")
if health_check.status_code == 200:
    print("Server is up and running")
else:
    print("Server is down")

# Generate a key
new_key = requests.get(url + "/generate")
json_data = new_key.json()
key = json_data["key"]


# Use the key
PASSWORD = "KhawarGhafoor931TaylorAvenue"
use_key = requests.get(
    url + "/use",
    headers={"Authorization": "Bearer " + PASSWORD},
    params={"password": PASSWORD},
)
json_data = use_key.json()
print(use_key.status_code)
