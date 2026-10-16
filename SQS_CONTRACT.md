# SQS Contract — `online/` → `main/`

This is the wire-level description of every SQS message `online/` publishes.
It exists for the `main/` maintainer: if you're updating the consumer in
`main/internal/infra/sqs/`, this is the source of truth for what arrives.

## TL;DR

| Field | Value | Where it comes from |
|---|---|---|
| `MessageBody` (string) | The vendor's POST body, **byte-for-byte** | `online/` reads `c.Request().Body` and forwards it without parsing |
| `MessageAttributes["source"]` | `"online"` (literal, `DataType=String`) | set explicitly in `online/backend/internal/infra/message/message.go` so the consumer routes deterministically rather than relying on its `online`-default fallback |
| `MessageAttributes["tenant_id"]` | UUID string (from `SCOPE_TENANT_ID`) | **Optional** — sent only when the env var is set. Ignored by main for the `online` source. |
| `MessageAttributes["location_id"]` | UUID string (from `SCOPE_LOCATION_ID`) | **Optional** — sent only when set. Ignored by main for the `online` source. |
| `MessageAttributes["traceparent"]` | W3C trace context | injected by `otelaws` middleware |
| `MessageAttributes["tracestate"]` | W3C tracestate (when set) | injected by `otelaws` |

> **Routing:** main routes each `online` order from the vendor body, NOT from
> these SQS attributes (see main `app/tenancy.ResolveStore`):
> 1. the body's numeric **`store_id`** (e.g. `3282`) → the tenant whose
>    `tenants.store_id` matches (set in main's admin);
> 2. if `store_id` is absent/`0` or unmapped → fuzzy **`store_name`** match
>    (first 8 chars, case-insensitive, must hit exactly one tenant name);
> 3. otherwise the order is never persisted and redrives to the **DLQ**.
>
> main accepts `online` messages with or without the scope attributes, so the
> `SCOPE_TENANT_ID` / `SCOPE_LOCATION_ID` env vars are optional and can be left
> unset. **Rollout order:** deploy main before an online build that stops
> sending them (older mains required both attributes). There is no placeholder
> tenant in main.

The queue is **standard (non-FIFO)** — same shape main/'s `elasticmq.conf`
already declares. There are no `MessageGroupId` or `MessageDeduplicationId`
fields, and SQS provides no built-in dedup; idempotency is the consumer's
responsibility (see § 5).

## 0. Where the producer publishes from

`online/` resolves the SQS endpoint from two env vars:

| Env var | Required | Notes |
|---|---|---|
| `SQS_QUEUE_URL` | yes | Full queue URL. e.g. `http://elasticmq:9324/000000000000/dev-ingest` in dev, `https://sqs.<region>.amazonaws.com/<acct>/online-ingest` in prod. |
| `SQS_ENDPOINT` | optional | AWS SDK base-endpoint override. Set in dev/staging to point the SQS client at a local ElasticMQ; leave empty in prod so the SDK targets real AWS. |

**Per-environment topology:**

- **Dev**: `online/`'s docker-compose includes its own ElasticMQ container
  (`docker/elasticmq.conf`, host port 9324). Both env vars point at it.
  For an end-to-end dev flow online → main, both stacks need to share a
  single ElasticMQ — either tear down one stack's ElasticMQ and have the
  other publish/consume on the surviving instance, or run only the
  consumer/producer side and stub the other.
- **Staging**: same shape as dev — ElasticMQ deployed alongside.
- **Prod**: `SQS_QUEUE_URL` is the AWS queue URL, `SQS_ENDPOINT` is empty,
  the SDK uses default AWS endpoints.

The wire format does not change between envs. Only the destination does.

## 1. `MessageBody` — opaque bytes

`online/` no longer parses, validates, or transforms the order body.
Whatever the vendor POSTs to `POST /api/v1/order` is what lands in
`MessageBody`.

- **Encoding:** UTF-8 string. SQS requires `MessageBody` to be a string,
  so the bytes are passed through `string(body)` without re-encoding.
  Non-UTF-8 bytes are theoretically possible but the vendor sends JSON
  (which is UTF-8), so this is academic.
- **Schema:** vendor-defined JSON. `online/` makes **no guarantees** about
  the shape — historically it has matched the `OrderRequest` struct that
  used to live at `online/backend/internal/app/order/dto.go` (now removed
  to keep customer fields out of the source tree). The vendor's spec is
  the authoritative reference.
- **PII implication:** the body carries customer name, phone, email,
  address (delivery orders), and partial card metadata. `online/` never
  logs or persists the body — `main/` should treat it the same.

### What `main/` should do with the body

`main/` already does the right thing today — see `contract/main.md` §3
(SQS ingest contract) for the consumer-side handling and §10 for the
canonical `OrderEvent` / `Snapshot` shape produced by the `online`
caster from this body. There is nothing `online/` adds to the body that
isn't documented in those two places.

## 2. Trace propagation — `MessageAttributes`

`online/`'s SQS client is wired with
`go.opentelemetry.io/contrib/instrumentation/.../otelaws`, which:

- Starts a producer span (`SQS.SendMessage`) on every send.
- Writes the W3C `traceparent` (and `tracestate` when present) into
  `MessageAttributes`.

On the producer side the span tree is now:

```
POST /api/v1/order              (echo OTEL middleware)
└── order.relay                 (orderApp.OrderService.Relay)
    └── SQS.SendMessage         (otelaws)
```

`main/`'s consumer already calls `otelaws.AppendMiddlewares` (see
`main/backend/internal/infra/sqs/client.go`), so it extracts the same
attributes automatically and chains the consumer span underneath
`SQS.SendMessage` via the propagated traceparent. **No custom plumbing
is required on the `main/` side.**

If you ever need to read the traceparent manually:

```go
if attr, ok := msg.MessageAttributes["traceparent"]; ok && attr.StringValue != nil {
    tp := *attr.StringValue
}
```

…but you should not need to — the SDK middleware does this for you and
attaches the parent context to the span the consumer creates.

## 3. Headers / metadata not present

`online/` does **not** forward:

- The vendor's `Authorization: Bearer <key>` header (auth is checked at
  ingress and discarded).
- The vendor's `User-Agent` or any other request header.
- A `Content-Type` (it's always treated as bytes regardless).
- An `online/`-side timestamp other than the AWS-assigned `SentTimestamp`.
- A vendor `order_id` outside of whatever's in the body itself.

If `main/` needs any of these, they have to come from the body itself or
be wired through as a new `MessageAttribute` (which would require a
change in `online/backend/internal/infra/message/message.go`).

## 4. Failure modes worth knowing about

| Failure | Where it happens | What `main/` sees |
|---|---|---|
| Vendor POST has empty body | online/ handler returns 400 | Nothing — never reaches SQS |
| Vendor POST body too large | Echo `BodyLimit(5 MiB)` rejects | Nothing — 413 to vendor |
| SQS publish fails (network, IAM, throttling, ElasticMQ down) | online/ returns 502 to vendor; `OrdersRejected{reason=sqs_error}` increments | Nothing — vendor retries |
| Vendor retry within visibility window | SQS treats as a brand new message | **Two messages with identical body** |
| Vendor retry after long delay | Same — new message | **Two messages with identical body** |

The bottom two rows are the load-bearing ones now that we're on a
standard (non-FIFO) queue: SQS has no built-in dedup, so a vendor retry
produces a duplicate. **`main/` must be idempotent on `sha256(body)` (or
on the vendor `order_id` after parsing the body)**, since `online/` no
longer carries that key on the wire.

## 5. Idempotency on the consumer side

**main/ uses Option B today** — it parses the vendor `order_id`, stores it
as `orders.source_event_id`, and lets Postgres reject duplicates via:

```sql
UNIQUE (tenant_id, source, source_event_id)
```

Scoping by `tenant_id` means two tenants could legitimately share the same
vendor `order_id` without colliding. The aggregate detects the duplicate
unique-constraint violation and surfaces `order.ErrDuplicate`; the consumer
acks (deletes the SQS message) and moves on.

`online/` does not guard against vendor retries — `main/` is the only place
duplicates get filtered.

Reference of the alternative:

- **Option A — body hash.** Compute `sha256(body)` in main/'s consumer and
  use it as a UNIQUE column. Stable across vendor retries (same body → same
  hash). Not used today; the vendor order_id is more useful for support
  conversations.

## 6. Producer-side observability (informational)

`online/` emits the following alongside each publish; none of it is on
the wire, but if you want to mirror anything in `main/`'s dashboards
the names are stable:

- **Span**: `order.relay` (custom, see §2). Attributes: `order.id`
  (sha256 hex of body), `order.body.bytes`, `order.relay.latency_ms`.
- **Counters**: `orders.received`, `orders.relayed`,
  `orders.rejected{reason}`.
- **Histograms**: `orders.relay.latency_ms`, `orders.body.bytes`.

Meter scope is `quiccpos/online`. All instruments are defined in
`online/backend/internal/observability/meters.go` if you want the
authoritative list.

## 7. Code pointers (online side)

If you need to change the contract, these are the touch points in
`online/`:

- `online/backend/internal/transport/handlers/orders.go:CreateOrder` —
  reads the body, hashes it for span attrs (not for SQS), hands off to
  the service.
- `online/backend/internal/app/order/service.go:Relay` — opens the
  `order.relay` span, bumps `OrdersReceived`/`BodyBytes`/`OrdersRelayed`
  /`OrderRelayLatency`/`OrdersRejected`, calls the publisher.
- `online/backend/internal/infra/message/message.go:Publish` — the actual
  `SendMessage` call. **This is where you'd add a `MessageAttribute`
  if `main/` ever needed something out-of-body.**
- `online/backend/internal/infra/config/config.go:SQSConfig` — defines
  the `SQS_QUEUE_URL` and `SQS_ENDPOINT` knobs documented in § 0.

---

If you change anything described above, update **this file** and the
package doc in `online/backend/internal/infra/message/message.go`. The
two should agree.
