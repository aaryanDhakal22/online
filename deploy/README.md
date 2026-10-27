# online/deploy — staging box (lightsail-stag)

Blue-green deploys and the nightly SQLite backup for online on the **staging**
box. Prod online (v1, `lightsail-api`) is untouched; it adopts this at the
online v2 cutover. `deploy.sh` refuses to run on prod.

| File | What |
|---|---|
| `docker-compose.staging.yml` | `online-blue` (127.0.0.1:13231) / `online-green` (127.0.0.1:13232), one shared SQLite volume `quicc-online-data` |
| `.env.stag.example` | copy to `.env.stag`: `BLUE_TAG` / `GREEN_TAG` (deploy.sh rewrites them) |
| `deploy.sh <tag>` | the blue-green deploy (`DRY_RUN=1` prints the changes instead) |
| `backup.sh` | SQLite backup + integrity check, keeps 14 |
| `systemd/quicc-online-backup.{service,timer}` | runs `backup.sh` daily at 03:30 |

The online SQLite holds the **live vendor API key**. Never rotate, retire,
overwrite or delete it without the owner — see "Vendor key" below.

## Deploy

```bash
cd /opt/online/deploy          # wherever this folder lives on the box
./deploy.sh staging-1a2b3c4    # an immutable CI tag; staging-latest is refused
```

Steps: active slot from `.state` → set `<SLOT>_TAG`, pull + start the other
slot → `/health`, then `/api/v1/version` must match the tag (commit prefix for
`staging-<sha>`, version for `vX.Y.Z`) → rewrite the nginx upstream file
(previous kept as `.bak`) → `nginx -t` + `systemctl reload nginx` → the
version again through `https://staging.quiccpos.com` (resolved to 127.0.0.1)
→ stop the old slot, write `.state`. Any failure restores the upstream and the
tag, stops the new slot and exits 1; the old slot never stopped serving.

Knobs (env): `SKIP_PULL=1` (image `docker load`ed on the box), `EXPECT_COMMIT`
/ `EXPECT_VERSION` (to deploy a floating tag), `PUBLIC_HOST`,
`UPSTREAM_FILE`, `NGINX_RELOAD`, `PROD_HOST_DENYLIST`. Secrets come from
Doppler `ingest/stg` (`.doppler-stag-token` next to the script, or the
directory's `doppler setup`); APP_ENV there must be `staging`.

`DRY_RUN=1` still runs the read-only probes (Doppler, curl, `nginx -t`). On the
box the slot isn't started in a dry run, so it stops at the health check —
it's for previewing the plan; `tests/test_deploy.py` drives the full logic
against fakes.

**No key operations during a deploy** (`/set`, `/setKey`, retire): for a few
seconds two processes share the SQLite file and each caches the active keys.

## One-time setup (before the first blue-green deploy)

Facts still to verify on the box (the survey couldn't SSH in): where the
current single-slot online runs (container name, compose dir — the root
Taskfile guesses `~/online`), where its SQLite lives, and the nginx site file
(`/etc/nginx/sites-enabled/…`, certbot-managed, today `proxy_pass` to
`127.0.0.1:1323`).

1. Copy this folder to the box (e.g. `/opt/online/deploy`), `cp .env.stag.example .env.stag`,
   put the Doppler `ingest/stg` token in `.doppler-stag-token` (0600).
2. Upstream file (points at the **current** online first, so nothing moves yet):
   ```bash
   echo 'upstream quicc_online { server 127.0.0.1:1323; }' | sudo tee /etc/nginx/conf.d/quicc-online-upstream.conf
   ```
   In the site's `location /api/` (and any other proxied location) change
   `proxy_pass http://127.0.0.1:1323;` to `proxy_pass http://quicc_online;`,
   then `sudo nginx -t && sudo systemctl reload nginx`. (Ubuntu's `nginx.conf`
   includes `conf.d/*.conf` at http level.)
3. Seed the shared volume with the current database, so the live staging key
   carries over (otherwise a fresh DB seeds `DEFAULT_API_KEY`):
   ```bash
   docker volume create quicc-online-data
   docker stop <current-online-container>       # a few seconds of 502s
   docker run --rm -v quicc-online-data:/data -v <current-db-dir>:/old alpine:3.20 \
     sh -c 'cp /old/app.db /data/app.db && chown 65532:65532 /data/app.db'
   docker start <current-online-container>
   ```
   (The old process may have written to `app.db-wal`; stop it before the copy,
   as above, so the WAL is checkpointed into `app.db`.)
4. `./deploy.sh staging-<sha7>` — no `.state` yet, so it deploys **blue**,
   switches nginx to 13231 and leaves the old container alone. Then stop and
   remove the old container (and its compose) by hand, and retire the
   `/health` cron (the 45 s self-probe replaced it — `STAGING.md`).

## Backup

```bash
sudo cp systemd/quicc-online-backup.{service,timer} /etc/systemd/system/
# adjust ExecStart= in the .service if deploy/ isn't at /opt/online/deploy
sudo systemctl daemon-reload && sudo systemctl enable --now quicc-online-backup.timer
sudo systemctl start quicc-online-backup.service && journalctl -u quicc-online-backup -n 5
```

`backup.sh` finds the DB in the `quicc-online-data` volume (or `ONLINE_DB=…`),
copies it with sqlite3's `.backup` (safe while online writes; host `sqlite3`,
or a throwaway `alpine` container if it's missing), runs
`PRAGMA integrity_check` on the copy, and keeps the newest 14 in
`/var/backups/quicc-online/online-<UTC>.db` (dir 0700, files 0600 — they hold
the key plaintext). It warns if the copy has no active key.

### Restore

```bash
cd /opt/online/deploy
slot=$(cat .state)                                   # the live slot
pick=/var/backups/quicc-online/online-<UTC>.db
sqlite3 "$pick" 'PRAGMA integrity_check;'            # must print ok
sqlite3 "$pick" "SELECT id, status, created_at, last_used_at FROM api_keys WHERE status='active';"
DOPPLER_TOKEN=$(cat .doppler-stag-token) doppler run -- \
  docker compose -f docker-compose.staging.yml --env-file .env.stag --profile "$slot" stop "online-$slot"
mp=$(docker volume inspect -f '{{ .Mountpoint }}' quicc-online-data)
sudo cp -a "$mp/app.db" "$mp/app.db.before-restore"
sudo rm -f "$mp/app.db-wal" "$mp/app.db-shm"
sudo install -o 65532 -g 65532 -m 600 "$pick" "$mp/app.db"
DOPPLER_TOKEN=$(cat .doppler-stag-token) doppler run -- \
  docker compose -f docker-compose.staging.yml --env-file .env.stag --profile "$slot" up -d "online-$slot"
curl -s -H "Authorization: Bearer <vendor key>" https://staging.quiccpos.com/api/v1/verify   # {"match":true}
```

## Vendor key

- The key the vendor sends is the only thing standing between the vendor and
  every online order. **Never rotate, retire, overwrite or delete it without
  the owner.** No script or test may call `/set`, `/setKey`, `/generate` or
  retire on prod.
- Invariant (since `002_key_guard.sql`): the table always keeps at least one
  active key. DB triggers refuse deleting, deactivating or overwriting the
  only active key (`last_active_key`), whoever writes the file.
- Rotation is two-step: `/set` or `/setKey` **adds** an active key (the old one
  keeps working) → the vendor switches → `POST /api/v1/keys/{id}/retire` on the
  old key, which answers 409 `new_key_unconfirmed` until the new key has been
  used, and 409 `last_active_key` for the only active key.
  `GET /api/v1/keys` lists id / status / created_at / last_used_at.
