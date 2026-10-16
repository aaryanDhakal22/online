# CLAUDE.md

This file is loaded into Claude Code's context when working in `online/`. For
the external wire contract see `contract/online.md` (and the deeper SQS
detail in `SQS_CONTRACT.md`).

## Project overview

`online/` is the front door — a tiny Echo v5 service that accepts authenticated
`POST /api/v1/order` requests from an external POS vendor and forwards the
**bare vendor JSON, byte-for-byte** to a single AWS SQS queue. It does not
parse, validate, transform, or store the body. It manages its own single API
key (the credential the vendor uses) in a one-row SQLite table.

`online/` has **no order persistence**. A vendor retry produces two
identical SQS messages; idempotency is `main/`'s job (see
`contract/main.md` §3.3).

## Common commands

`task` commands shell in a Doppler token from `.doppler-dev-online` (raw
token file, gitignored). No `doppler login` / `doppler setup` needed.

```bash
task devup             # foreground
task devupd            # detached
task devdown
task devreset          # rebuild images
task dev-del-volumes
task dev-logs -- api
task build             # build binary to backend/tmp/main
task vet
task test-go           # Go unit tests
task test              # Python pytest (dev stack must be up; never targets prod;
                       # key-rotation tests restore the active key)
task test-script       # legacy interactive harness
task release -- patch  # version bump + tag + push (CI builds + pushes GHCR only; no deploy)
```

## What goes on the wire

Every `POST /api/v1/order` produces exactly one SQS message:

| Field | Value | Source |
|---|---|---|
| `MessageBody` | The vendor's body byte-for-byte | `c.Request().Body` |
| `MessageAttributes["source"]` | `"online"` literal | hard-coded in `internal/infra/message/message.go` |
| `MessageAttributes["tenant_id"]` | UUID — **only if `SCOPE_TENANT_ID` is set** | `SCOPE_TENANT_ID` env (optional) |
| `MessageAttributes["location_id"]` | UUID — **only if `SCOPE_LOCATION_ID` is set** | `SCOPE_LOCATION_ID` env (optional) |
| `MessageAttributes["traceparent"]` | W3C trace context | injected by `otelaws` |

`main/` **ignores the scope attributes for the `online` source** — it routes each
order by the vendor body's `store_id` (matched to `tenants.store_id`), falling
back to a fuzzy `store_name` match, and DLQs anything unroutable (see main
`app/tenancy.ResolveStore`). So `SCOPE_*` are optional: `messageAttributes()` in
`internal/infra/message/message.go` only adds each when set. **Deploy main before
an online build that stops sending them** — older mains required both.

## System layout (online side)

```
cmd/server/main.go         — Bootstrap: config → OTEL → DB → key svc → SQS → routes → server
internal/shared/           — Zerolog logger
internal/infra/config/     — env-var loader (Fatal on missing required)
internal/infra/message/    — SQS publisher (otelaws-wrapped, single Publish call)
internal/infra/database/   — SQLite + sqlc for the api_keys table
internal/domain/key/       — Key entity, statuses, repository interface
internal/app/key/          — Key service: bootstrap / generate / set / verify / setKey
internal/app/order/        — One-method order service that calls Publish
internal/transport/        — Echo handlers + middleware (auth, admin, request log, CORS)
internal/observability/    — OTLP exporters, log bridge, custom meters, watchdog
internal/migrations/       — One-shot SQL applied at every startup (`001_setup.sql`)
```

## Key lifecycle (Active / Primed)

- At most one `Active` key and at most one `Primed` key at any time.
- `GET /api/v1/generate` mints a new Primed key and returns the plaintext
  once. The vendor copies it and sends it to the operator.
- `GET /api/v1/set` (admin-gated) promotes the Primed key to Active and
  deactivates the prior Active.
- `POST /api/v1/setKey` (admin-gated) is the break-glass path: insert + activate
  a plaintext directly, skipping Primed.
- `GET /api/v1/getKey` (admin-gated) returns the current Active plaintext.
- `GET /api/v1/verify` (vendor-auth) returns `{match: bool}` without
  side-effects.

The active key plaintext is cached behind an `atomic.Pointer` warmed at
bootstrap and refreshed on every Set, so the read-mostly `Verify` path
never touches SQLite during normal operation.

## Two auth surfaces, by intent

| Credential | Header | Used by |
|---|---|---|
| Vendor API key | `Authorization: Bearer <key>` | the external POS vendor |
| Admin passcode | `X-Admin-Passcode: <passcode>` | operator (curl, scripts) |

The admin passcode is **argon2id-hashed** in `ADMIN_PASS_HASH`. Plaintext
never lands on disk or in logs. Header values are never logged; only their
lengths.

## Observability

- OTLP/HTTP to `OTEL_EXPORTER_OTLP_ENDPOINT`. Empty → no-op providers.
- Resource: `service.name=quicc-online`, `service.version` from build flag,
  `deployment.environment=$APP_ENV`.
- Custom meters in `internal/observability/meters.go` — see
  `contract/online.md` §7.2 for the names.
- Liveness watchdog (120s threshold, 30s tick) panics + restarts the
  container if a registered goroutine goes silent. The HTTP heartbeat is
  request-pulsed, so `RunSelfProbe` (`observability/liveness.go`, started in
  `cmd/server/main.go`) GETs our own `/health` every 45s through the full
  middleware chain — an idle server stays alive, a wedged one still trips it.
  Probes carry `X-Quicc-Self-Probe`, which the OTEL + request-log middlewares
  skip (no span / log line per probe).
- OTEL export errors go through a rate-limited handler (`observability/errhandler.go`,
  ≤1 line/min to stderr); exporters time out at 5s. The OTLP log hook is in-repo
  (`observability/zlog.go`) — the upstream `otelzerolog` bridge was abandoned.
- Trace ID is written to `X-Trace-Id` response header on every request so
  the vendor can grep their support ticket.

## Environment variables

See `contract/online.md` §5 for the full table. The required ones:

```
APP_ENV
DEFAULT_API_KEY            # seeded as Active on first boot if no key exists
ADMIN_PASS_HASH            # argon2id-encoded
SQS_QUEUE_URL              # full queue URL
```

Optional: `SCOPE_TENANT_ID` / `SCOPE_LOCATION_ID` (legacy scope UUIDs — sent on
SQS messages only when set; main ignores them for this source).

Secrets are Doppler-managed in every environment.

## Deployment

CI (`.github/workflows/docker-publish.yml`) **builds + pushes images to GHCR
only** — staging tags on every branch push, `prod-v1.2.3` + `prod-latest` on a
`v*` semver tag. **It does not deploy or SSH anywhere.** The old CI `deploy`
job (SSH → Lightsail `docker compose pull && up -d`) was removed 2026-07-11;
rolling a built image out to a host is now **manual** (planned: orchestrated
from the console), so a plain push / tag can never touch a running host.

Single-slot deploy with brief 3–5s downtime — vendor retries the 502. There is
**no blue-green** on online by design: the relay is stateless, vendor retries
handle the swap window, and the operational cost of two slots isn't worth it.

## Files to look at for…

| To learn about | Look at |
|---|---|
| SQS wire contract (deep detail) | `SQS_CONTRACT.md` |
| Order ingest handler | `internal/transport/handlers/orders.go` |
| Key lifecycle | `internal/transport/handlers/keys.go` + `internal/app/key/service.go` |
| SQS publisher | `internal/infra/message/message.go` |
| Auth middleware | `internal/transport/middleware/authMiddleware.go`, `adminMiddleware.go` |
| Env vars | `internal/infra/config/config.go` |
| Schema | `internal/migrations/sql/001_setup.sql` |
| Watchdog | `internal/observability/liveness.go` |
