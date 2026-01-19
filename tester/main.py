import requests

url = 'http://localhost:1323/api/v1'

smaller_payload = {
        'id': '123456789',
        'name': 'Aaryan',
        }
larger_payload = {
    'order_id': '123456789',
    'product_id': '123456789',
    'customer_id': '123456789',
    'amount': 100,
    'currency': 'USD',
    'status': 'PENDING'
}

health_check = requests.get(url + '/healthz')
if health_check.status_code == 200:
    print('Server is up and running')
else:
    print('Server is down')

# Generate three times
new_key= requests.get(url + '/generate' )
new_key= requests.get(url + '/generate' )
new_key= requests.get(url + '/generate' )

# Use the key
use_key = requests.get(url + '/use')
json_data = use_key.json()
print(json_data)

# Create a new order with the key as bearer token
new_order_1 = requests.post(url + '/order', headers={'Authorization': 'Bearer ' + new_key.text}, json=smaller_payload)

# Confirm the order was created

print(new_order_1.text)
print(new_order_1.status_code)


