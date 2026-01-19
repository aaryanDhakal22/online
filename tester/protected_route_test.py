import time

import requests

url = "http://localhost:1323/api/v1"
health_check = requests.get(url + "/healthz")
if health_check.status_code == 200:
    print("Server is up and running")
else:
    print("Server is down")

new_key = requests.get(url + "/generate")
use_key = requests.get(url + "/use")
json_data = use_key.json()
key = json_data["key"]
print(key)

verify_key = requests.get(url + "/verify", headers={"Authorization": "Bearer " + key})
print(verify_key.status_code)
