"""Pytest suite for online/. Targets a running dev stack — start it with
`task devup` (under Doppler) before running these.

Tests are split into two markers:
- not slow / default: liveness, key lifecycle, opaque relay, auth.
- slow: bulk order send, repeated rotations.

The opaque-relay test (test_order_relay_accepts_opaque) only asserts the
HTTP contract — it tolerates the dev-mode noop publisher (no SQS) and
doesn't try to verify message arrival downstream. For end-to-end, run the
project-level `task test-e2e` from repo root, which exercises main/'s
consumer too.
"""

import secrets

import pytest


# ---------------------------------------------------------------------------
# Liveness
# ---------------------------------------------------------------------------


def test_healthz(client):
    resp = client.get("/healthz")
    assert resp.status_code == 200
    assert resp.text.strip() == "OK"


# ---------------------------------------------------------------------------
# Key lifecycle
# ---------------------------------------------------------------------------


def test_generate_returns_id_and_key(client):
    resp = client.get("/generate")
    assert resp.status_code == 200
    body = resp.json()
    assert body.get("id"), f"missing id in {body}"
    assert body.get("key"), f"missing key in {body}"


def _verifies(client, key):
    v = client.get("/verify", headers={"Authorization": f"Bearer {key}"})
    assert v.status_code == 200, v.text
    return v.json()["match"] is True


def test_generate_then_set_adds_active(client, admin_passcode, restore_active_key):
    g = client.get("/generate")
    assert g.status_code == 200
    new_key = g.json()["key"]

    s = client.get("/set", headers={"X-Admin-Passcode": admin_passcode})
    assert s.status_code == 200, s.text

    assert _verifies(client, new_key)
    # Two-step rotation: the previous key keeps working until it is retired.
    assert _verifies(client, restore_active_key)


def test_set_requires_admin(client):
    resp = client.get("/set", headers={"X-Admin-Passcode": "wrong-pass"})
    assert resp.status_code == 401


def test_set_key_break_glass(client, admin_passcode, restore_active_key):
    pasted = "manual-break-glass-" + secrets.token_urlsafe(12)
    r = client.post(
        "/setKey",
        headers={"X-Admin-Passcode": admin_passcode},
        json_data={"key": pasted},
    )
    assert r.status_code == 200, r.text
    assert r.json()["key"] == pasted

    assert _verifies(client, pasted)
    assert _verifies(client, restore_active_key)


def test_set_key_rejects_empty(client, admin_passcode):
    r = client.post(
        "/setKey",
        headers={"X-Admin-Passcode": admin_passcode},
        json_data={"key": ""},
    )
    assert r.status_code == 400


def test_get_key_requires_admin(client):
    resp = client.get("/getKey", headers={"X-Admin-Passcode": "nope"})
    assert resp.status_code == 401


# ---------------------------------------------------------------------------
# Key guard: list + retire (two-step rotation)
# ---------------------------------------------------------------------------


def _keys(client, admin_passcode):
    r = client.get("/keys", headers={"X-Admin-Passcode": admin_passcode})
    assert r.status_code == 200, r.text
    return r.json()["keys"]


def _single_active_id(client, admin_passcode):
    active = [k for k in _keys(client, admin_passcode) if k["status"] == "active"]
    if len(active) != 1:
        pytest.skip(f"needs exactly one active key on the target, found {len(active)}")
    return active[0]["id"]


def test_keys_routes_require_admin(client):
    assert client.get("/keys", headers={"X-Admin-Passcode": "nope"}).status_code == 401
    r = client.post("/keys/whatever/retire", headers={"X-Admin-Passcode": "nope"})
    assert r.status_code == 401


def test_keys_list_has_no_plaintext(client, admin_passcode, active_key):
    keys = _keys(client, admin_passcode)
    assert keys, "expected at least one key"
    for k in keys:
        assert set(k) == {"id", "status", "created_at", "last_used_at"}, k
    assert active_key not in str(keys)


def test_retire_unknown_key_404(client, admin_passcode):
    r = client.post("/keys/no-such-key/retire", headers={"X-Admin-Passcode": admin_passcode})
    assert r.status_code == 404, r.text
    assert r.json()["error"] == "not_found"


def test_retire_last_active_key_409(client, admin_passcode):
    only = _single_active_id(client, admin_passcode)
    r = client.post(f"/keys/{only}/retire", headers={"X-Admin-Passcode": admin_passcode})
    assert r.status_code == 409, r.text
    assert r.json()["error"] == "last_active_key"


def test_retire_old_key_before_new_used_409(client, admin_passcode, restore_active_key):
    old_id = _single_active_id(client, admin_passcode)
    new_key = "unconfirmed-" + secrets.token_urlsafe(12)
    r = client.post("/setKey", headers={"X-Admin-Passcode": admin_passcode}, json_data={"key": new_key})
    assert r.status_code == 200, r.text

    # Nobody has used the new key yet: the old (live) key must stay.
    r = client.post(f"/keys/{old_id}/retire", headers={"X-Admin-Passcode": admin_passcode})
    assert r.status_code == 409, r.text
    assert r.json()["error"] == "new_key_unconfirmed"
    assert _verifies(client, restore_active_key)


def test_rotation_retire_after_use(client, admin_passcode, restore_active_key):
    """Add a key, use the one that stays, retire the other (here the test key
    is the one retired, so the target's real key is never at risk)."""
    hdr = {"X-Admin-Passcode": admin_passcode}
    extra = "retire-me-" + secrets.token_urlsafe(12)
    r = client.post("/setKey", headers=hdr, json_data={"key": extra})
    assert r.status_code == 200, r.text
    extra_id = r.json()["id"]
    assert _verifies(client, extra)

    # The original key hasn't been used since `extra` was added.
    r = client.post(f"/keys/{extra_id}/retire", headers=hdr)
    assert r.status_code == 409, r.text
    assert r.json()["error"] == "new_key_unconfirmed"

    assert _verifies(client, restore_active_key)
    r = client.post(f"/keys/{extra_id}/retire", headers=hdr)
    assert r.status_code == 200, r.text
    assert not _verifies(client, extra)
    assert _verifies(client, restore_active_key)

    r = client.post(f"/keys/{extra_id}/retire", headers=hdr)
    assert r.status_code == 409 and r.json()["error"] == "not_active", r.text


# ---------------------------------------------------------------------------
# Auth on /order
# ---------------------------------------------------------------------------


def test_order_requires_bearer(client):
    resp = client.post("/order", headers={"Content-Type": "application/json"}, json_data={})
    assert resp.status_code == 401


def test_order_rejects_wrong_key(client):
    resp = client.post(
        "/order",
        headers={"Authorization": "Bearer not-the-key", "Content-Type": "application/json"},
        json_data={"hello": "world"},
    )
    assert resp.status_code == 401


# ---------------------------------------------------------------------------
# Opaque relay
# ---------------------------------------------------------------------------


def test_order_relay_accepts_opaque(client, active_key):
    """Send an arbitrary JSON body — online/ should hash + forward without
    inspecting fields. We only assert the HTTP contract."""
    body = {"completely": "arbitrary", "nested": {"shape": [1, 2, 3]}}
    resp = client.post(
        "/order",
        headers={
            "Authorization": f"Bearer {active_key}",
            "Content-Type": "application/json",
        },
        json_data=body,
    )
    assert resp.status_code == 202, resp.text
    payload = resp.json()
    assert payload.get("status") == "relayed"
    assert "ext_order_id" in payload
    assert payload.get("body_bytes") == len(resp.request.body or b"")


def test_order_rejects_empty_body(client, active_key):
    resp = client.post(
        "/order",
        headers={
            "Authorization": f"Bearer {active_key}",
            "Content-Type": "application/json",
        },
        data=b"",
    )
    assert resp.status_code == 400


# ---------------------------------------------------------------------------
# Bulk / slow
# ---------------------------------------------------------------------------


@pytest.mark.slow
def test_bulk_relay(client, active_key):
    for i in range(20):
        resp = client.post(
            "/order",
            headers={
                "Authorization": f"Bearer {active_key}",
                "Content-Type": "application/json",
            },
            json_data={"i": i, "nonce": secrets.token_hex(4)},
        )
        assert resp.status_code == 202, f"iteration {i}: {resp.text}"
