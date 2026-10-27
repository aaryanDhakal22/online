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

# prod_test.py is a one-shot probe, not a pytest suite: it matches pytest's
# `*_test.py` pattern and calls load_env() at import.
collect_ignore = ["prod_test.py"]


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


def _active_ids(client, hdr):
    r = client.get("/keys", headers=hdr)
    assert r.status_code == 200, f"/keys returned {r.status_code}: {r.text}"
    return {k["id"] for k in r.json()["keys"] if k["status"] == "active"}


@pytest.fixture
def restore_active_key(client, admin_passcode):
    """Key tests ADD active keys (/set, /setKey never deactivate). Afterwards,
    put online back to the keys that were active before — ONLINE_API_KEY for
    the e2e suite + console smokes must keep working — by RETIRING the keys
    the test added (two-step rotation), never by overwriting.

    Retiring needs the remaining key to have been used since the test key was
    added (409 new_key_unconfirmed otherwise), so the teardown first
    authenticates with the original key. If a test somehow left the original
    key inactive, it is added back first and kept."""
    hdr = {"X-Admin-Passcode": admin_passcode}
    resp = client.get("/getKey", headers=hdr)
    assert resp.status_code == 200, resp.text
    original = resp.json()["key"]
    keep = _active_ids(client, hdr)
    yield original

    v = client.get("/verify", headers={"Authorization": f"Bearer {original}"})
    if not (v.status_code == 200 and v.json()["match"]):
        r = client.post("/setKey", headers=hdr, json_data={"key": original})
        assert r.status_code == 200, f"failed to re-add the original vendor key: {r.text}"
        keep.add(r.json()["id"])
        v = client.get("/verify", headers={"Authorization": f"Bearer {original}"})
    assert v.status_code == 200 and v.json()["match"], "original vendor key does not verify"

    for key_id in sorted(_active_ids(client, hdr) - keep):
        r = client.post(f"/keys/{key_id}/retire", headers=hdr)
        assert r.status_code == 200, f"failed to retire test key {key_id}: {r.status_code} {r.text}"
