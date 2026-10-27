# online/ tests

| File | What it is | When to use |
|---|---|---|
| `test_api.py` | pytest, asserts on responses | CI; default. Run via `task test`. |
| `prod_test.py` | one-shot probe (reads `/getKey` only) | Not collected by pytest; `client.py` refuses `APP_ENV=prod`. |
| `test_deploy.py` | runs `deploy/deploy.sh` in `DRY_RUN` against fake `docker`/`curl`/`nginx` | Offline; no stack needed. |

The old interactive harness (`test.py` / `main.py`, `task test-script`) was
removed: it rotated keys with the pre-guard `/set` semantics.

## Running

Both suites read config from environment variables; Doppler is expected to
inject them. From `online/`:

```bash
task test          # pytest suite
task test-quick    # everything except @pytest.mark.slow
```

Tests assume the API is reachable at `${DEV_URL}:${DEV_BACKEND_PORT}` (dev)
or `${PROD_URL}` (prod). The dev stack must already be up — `task devup`
in another terminal first.

## Required env vars

The fixtures in `conftest.py` read these. Doppler wires them.

| Var | Notes |
|---|---|
| `APP_ENV` | `dev` or `prod`. Other values → load_env raises. |
| `DEV_URL` | e.g. `http://localhost`. |
| `DEV_BACKEND_PORT` | defaults to `1323` if unset. |
| `PROD_URL` | full URL incl. scheme/host (no port). |
| `ADMIN_PASSCODE` | plaintext; admin-gated tests `pytest.skip` if missing. |
| `DEFAULT_API_KEY` | seeded into `api_keys` on first boot of online/. |

## Notes for new tests

- **Never touch the target's real vendor key.** `/set` and `/setKey` only
  ADD keys; a test that adds one takes the `restore_active_key` fixture, which
  afterwards authenticates with the original key and RETIRES every key the test
  added (`POST /keys/{id}/retire`). Never retire or overwrite the original.
- **Pytest fixtures over module-level state.** `conftest.py` exposes
  `client`, `admin_passcode`, `active_key` and `restore_active_key`. Compose new tests around
  those instead of recreating an `ApiClient` at import time.
- **Don't assume SQS is reachable.** In dev with `SQS_QUEUE_URL` empty,
  online/ uses a noop publisher — `/order` returns 202 but nothing reaches
  AWS. Pytest tests should only assert the HTTP contract; for end-to-end
  with main/'s consumer, run `task test-e2e` from the repo root.
- **Treat order bodies as opaque.** online/ hashes the body and forwards
  the bytes verbatim. Tests should send arbitrary JSON and assert the
  202 + response shape, not the message that ends up on SQS.
- **Synthetic data, not PII.** `data.py` carries large fixtures with
  realistic-looking customer info — fine for legacy interactive use but
  prefer minimal synthetic blobs for new pytest cases (`{"i": 1}` is fine).
