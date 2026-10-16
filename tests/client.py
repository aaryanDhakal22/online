import os

import requests


class ApiClient:
    """Tiny HTTP wrapper for hitting online/'s api/v1. Reads its base URL
    from APP_ENV / DEV_URL / PROD_URL — Doppler is expected to inject these
    via `doppler run --` (see online/Taskfile.yml). No dotenv loading; if
    you want to run tests bare, export the vars yourself.
    """

    def __init__(self, endpoint: str):
        self.endpoint = endpoint
        self.base_url = ""
        self.backend_port = ""
        self.backend_url = ""
        self.env = ""

    def load_env(self):
        # ONLINE_BASE_URL (injected by the console for the target env) wins over
        # the APP_ENV-derived defaults so the same suite can point at dev or stg.
        override = os.getenv("ONLINE_BASE_URL")
        if override:
            self.backend_url = override.rstrip("/")
            self.base_url = self.backend_url
            self.env = (os.getenv("TARGET_ENV") or os.getenv("APP_ENV") or "dev").lower()
            return
        app_env = (os.getenv("APP_ENV") or "dev").lower()
        if app_env == "dev":
            self.base_url = os.getenv("DEV_URL", "http://localhost")
            # Host port: DEV_HOST_PORT is the Doppler-owned published port
            # (1324 for online — :1323 on the host is main). SERVER_PORT is the
            # in-container port, so it's deliberately NOT used here.
            self.backend_port = os.getenv("DEV_BACKEND_PORT") or os.getenv("DEV_HOST_PORT") or "1324"
            self.backend_url = f"{self.base_url}:{self.backend_port}"
        elif app_env == "prod":
            # Never point an automated suite at prod (key-rotation tests would
            # rotate the live vendor key). Use the manual harness if you must.
            raise RuntimeError("refusing to run the online test suite against prod")
        else:
            raise RuntimeError(f"unknown APP_ENV: {app_env!r}")
        self.env = app_env

    def get(self, route, headers=None):
        if headers is None:
            headers = {}
        url = f"{self.backend_url}/{self.endpoint}{route}"
        return requests.get(url, headers=headers)

    def post(self, route, headers=None, json_data=None, data=None):
        if headers is None:
            headers = {}
        url = f"{self.backend_url}/{self.endpoint}{route}"
        return requests.post(url, headers=headers, json=json_data, data=data)
