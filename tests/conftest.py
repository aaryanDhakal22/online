"""Shared fixtures for online/ pytest suite.

Env contract (Doppler injects these via `task test`):
- APP_ENV               dev | prod
- DEV_URL               base URL of the dev box (e.g. http://localhost)
- DEV_BACKEND_PORT      default 1323
- PROD_URL              base URL in prod
- ADMIN_PASSCODE        plaintext that hashes against ADMIN_PASS_HASH
- DEFAULT_API_KEY       seeded into the api_keys table on first boot

Most tests assume the api server has just been started with a known
DEFAULT_API_KEY. The `active_key` fixture walks /getKey to fetch it; tests
then use that for /order requests.
"""

import os

import pytest

from client import ApiClient

# Interactive / prod-targeting scripts, not pytest suites: prod_test.py matches
# pytest's `*_test.py` pattern and calls load_env() at import (and targets prod);
# test.py is the legacy interactive harness (`task test-script`).
collect_ignore = ["prod_test.py", "test.py"]


@pytest.fixture(scope="session")
def client():
    c = ApiClient("api/v1")
    c.load_env()
    return c


@pytest.fixture(scope="session")
def admin_passcode():
    pc = os.getenv("ADMIN_PASSCODE") or os.getenv("ADMIN_PASS")
    if not pc:
        pytest.skip("ADMIN_PASSCODE not set; skipping admin-gated tests")
    return pc


@pytest.fixture
def active_key(client):
    """The plaintext of the currently-active vendor key: read via the admin
    /getKey when ADMIN_PASSCODE is available, else DEFAULT_API_KEY (the key
    online seeds as Active on first boot — what dev Doppler provides)."""
    pc = os.getenv("ADMIN_PASSCODE") or os.getenv("ADMIN_PASS")
    if pc:
        resp = client.get("/getKey", headers={"X-Admin-Passcode": pc})
        assert resp.status_code == 200, f"/getKey returned {resp.status_code}: {resp.text}"
        return resp.json()["key"]
    key = os.getenv("DEFAULT_API_KEY")
    if not key:
        pytest.skip("neither ADMIN_PASSCODE nor DEFAULT_API_KEY set")
    return key


@pytest.fixture
def restore_active_key(client, admin_passcode):
    """Key-rotation tests change online's ACTIVE vendor key, which would break
    ONLINE_API_KEY for the e2e suite + console smokes. Snapshot the active
    plaintext first and put it back (break-glass setKey) afterwards."""
    resp = client.get("/getKey", headers={"X-Admin-Passcode": admin_passcode})
    assert resp.status_code == 200, resp.text
    original = resp.json()["key"]
    yield original
    r = client.post("/setKey", headers={"X-Admin-Passcode": admin_passcode}, json_data={"key": original})
    assert r.status_code == 200, f"failed to restore the active vendor key: {r.text}"
