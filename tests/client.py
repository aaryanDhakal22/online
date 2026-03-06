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
    
        app_env = os.getenv("APP_ENV")
        if app_env is None:
            print("APP_ENV not set")
            print("Setting app_env to dev")
            app_env = "dev"
        app_env = app_env.lower()
        print(f"app_env: {app_env}")
        if app_env == "dev" :
            self.base_url = os.getenv("DEV_URL")
            self.backend_port = os.getenv("DEV_BACKEND_PORT")
            if self.backend_port is None:
                self.backend_port = "1323"
            self.backend_url = f"{self.base_url}:{self.backend_port}"
        elif app_env == "prod":
            self.base_url = os.getenv("PROD_URL")
            self.backend_port = None
            self.backend_url = self.base_url
        else:
            print("Invalid app env")

    def get(self,route,headers=None):
        if headers is None:
            headers = {}
        url = f"{self.backend_url}/{self.endpoint}{route}"
        print(f"GET {url}","->",headers)
        response = requests.get(url,headers=headers)
        print(f"GET {url}","->",response.status_code)
        return response

    def post(self,route,headers=None,json_data=None):
        if headers is None:
            headers = {}
        url = f"{self.backend_url}/{self.endpoint}{route}"
        response = requests.post(url,headers=headers,json=json_data)
        # print(f"POST {url}","->",response.status_code)
        return response
