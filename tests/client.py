import dotenv
import os
import requests

dotenv.load_dotenv()

class ApiClient():
    def __init__(self,endpoint ):
        self.base_url = ""
        self.backend_port = ""
        self.backend_url = ""
        self.endpoint = endpoint

    def load_env(self):
        self.base_url = os.getenv("BASE_URL")
        self.backend_port = os.getenv("BACKEND_PORT")
        self.backend_url = f"http://localhost:{self.backend_port}"
        if self.base_url is None:
            print("BASE_URL is not set")
            print("Using default value: http://localhost")
            self.base_url = "http://localhost"
        if self.backend_port is None:
            print("BACKEND_PORT is not set")
            print("Using default value: 1323")
            self.backend_port = "1323"
        self.backend_url = f"{self.base_url}:{self.backend_port}"

    def get(self,route,headers=None):
        if headers is None:
            headers = {}
        url = f"{self.backend_url}/{self.endpoint}{route}"
        response = requests.get(url,headers=headers)
        # print(f"GET {url}","->",response.status_code)
        return response

    def post(self,route,headers=None,json_data=None):
        if headers is None:
            headers = {}
        url = f"{self.backend_url}/{self.endpoint}{route}"
        response = requests.post(url,headers=headers,json=json_data)
        # print(f"POST {url}","->",response.status_code)
        return response
