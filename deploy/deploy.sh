#!/usr/bin/env bash
# deploy.sh — blue-green deploy of online on the STAGING box (lightsail-stag).
#
# Usage:   ./deploy.sh <image-tag>          e.g. ./deploy.sh staging-1a2b3c4
#          DRY_RUN=1 ./deploy.sh <tag>      print every change instead of making it
#
# Modelled on main/deploy/deploy.sh, but the switch is host nginx, not Traefik:
#   1. active slot from .state → the other one is deployed
#   2. <SLOT>_TAG=<tag> in .env.stag, pull + start the inactive slot
#   3. its /health, then /api/v1/version must match the tag
#   4. point the nginx upstream at it (old file kept as .bak), nginx -t, reload
#   5. check the version through nginx (https://$PUBLIC_HOST via 127.0.0.1)
#   6. any failure → restore the upstream + tag, stop the new slot, exit 1
#   7. success → stop the old slot, write .state
#
# Version check: staging-<sha> tags must report a commit starting with <sha>;
# vX.Y.Z / X.Y.Z / prod-vX.Y.Z must report version X.Y.Z. Floating tags
# (staging-latest, branch-*) are refused unless EXPECT_COMMIT or
# EXPECT_VERSION says what to expect.
#
# It REFUSES to run on prod: hostname in PROD_HOST_DENYLIST, the prod layout
# (/opt/ingest), a prod public host, or APP_ENV (from Doppler) not staging.
# No key operations (/set, /setKey, retire) during a deploy: two processes
# share the SQLite file and each caches the active keys.
#
# One-time setup on the box: deploy/README.md.
set -euo pipefail

TAG="${1:?Usage: ./deploy.sh <image-tag>  (e.g. staging-1a2b3c4)}"

DEPLOY_DIR="${DEPLOY_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.staging.yml}"
ENV_FILE="${ENV_FILE:-.env.stag}"
STATE_FILE="${STATE_FILE:-.state}"
UPSTREAM_FILE="${UPSTREAM_FILE:-/etc/nginx/conf.d/quicc-online-upstream.conf}"
UPSTREAM_NAME="${UPSTREAM_NAME:-quicc_online}"
PUBLIC_HOST="${PUBLIC_HOST:-staging.quiccpos.com}"
NGINX_RELOAD="${NGINX_RELOAD:-systemctl reload nginx}"
PROD_HOST_DENYLIST="${PROD_HOST_DENYLIST:-lightsail-api ingest prod}"
PROD_PUBLIC_HOSTS="${PROD_PUBLIC_HOSTS:-ingest.quiccpos.com}"
PROD_MARKER_DIR="${PROD_MARKER_DIR:-/opt/ingest}"
HEALTH_RETRIES="${HEALTH_RETRIES:-30}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-2}"
SKIP_PULL="${SKIP_PULL:-0}"      # 1 = image already loaded on the box
DRY_RUN="${DRY_RUN:-0}"
if [[ $EUID -eq 0 ]]; then SUDO="${SUDO-}"; else SUDO="${SUDO-sudo}"; fi

declare -A SLOT_PORT=([blue]=13231 [green]=13232)

say()  { echo "▶  $*"; }
ok()   { echo "✓  $*"; }
die()  { echo "✗  $*" >&2; exit 1; }

# run: every command that changes something goes through here.
run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    echo "DRY: $*"
  else
    "$@"
  fi
}

cd "$DEPLOY_DIR"

# ── 0. Refuse prod ──────────────────────────────────────────
host=$(hostname 2>/dev/null || echo unknown)
for bad in $PROD_HOST_DENYLIST; do
  if [[ "$host" == *"$bad"* ]]; then die "REFUSING: hostname '$host' matches the prod denylist ('$bad')"; fi
done
for bad in $PROD_PUBLIC_HOSTS; do
  if [[ "$PUBLIC_HOST" == "$bad" ]]; then die "REFUSING: PUBLIC_HOST '$PUBLIC_HOST' is the prod host"; fi
done
if [[ -e "$PROD_MARKER_DIR" ]]; then die "REFUSING: $PROD_MARKER_DIR exists — this looks like the prod online box"; fi
[[ -f "$ENV_FILE" ]] || die "$ENV_FILE not found in $DEPLOY_DIR (copy .env.stag.example)"
[[ -f "$COMPOSE_FILE" ]] || die "$COMPOSE_FILE not found in $DEPLOY_DIR"
[[ -f "$UPSTREAM_FILE" ]] || die "$UPSTREAM_FILE missing — do the one-time nginx setup first (deploy/README.md)"

if [[ -z "${DOPPLER_TOKEN:-}" && -f .doppler-stag-token ]]; then
  DOPPLER_TOKEN=$(cat .doppler-stag-token)
  export DOPPLER_TOKEN
fi
app_env=$(doppler run -- printenv APP_ENV 2>/dev/null || true)
case "$app_env" in
  staging|stg|stag) ;;
  *) die "REFUSING: Doppler APP_ENV is '${app_env:-<unset>}', not staging" ;;
esac
COMPOSE=(doppler run -- docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE")

# ── 1. Slots + expected version ────────────────────────────
ACTIVE=$(cat "$STATE_FILE" 2>/dev/null || true)
case "$ACTIVE" in
  blue)  INACTIVE=green ;;
  green) INACTIVE=blue ;;
  "")    INACTIVE=blue ;;   # first blue-green deploy
  *)     die "$STATE_FILE holds '$ACTIVE' (expected blue|green)" ;;
esac
NEW_PORT=${SLOT_PORT[$INACTIVE]}

EXPECT_COMMIT="${EXPECT_COMMIT:-}"
EXPECT_VERSION="${EXPECT_VERSION:-}"
if [[ -z "$EXPECT_COMMIT" && -z "$EXPECT_VERSION" ]]; then
  if [[ "$TAG" =~ ^staging-([0-9a-f]{7,40})$ ]]; then
    EXPECT_COMMIT=${BASH_REMATCH[1]}
  elif [[ "$TAG" =~ ^(prod-)?v?([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
    EXPECT_VERSION=${BASH_REMATCH[2]}
  else
    die "can't tell what '$TAG' should report — deploy a pinned tag (staging-<sha7>) or set EXPECT_COMMIT / EXPECT_VERSION"
  fi
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Host      : $host (APP_ENV=$app_env)"
say "Active    : ${ACTIVE:-<none: first blue-green deploy>}"
say "Deploying : $INACTIVE (127.0.0.1:$NEW_PORT) → $TAG"
say "Expecting : ${EXPECT_COMMIT:+commit $EXPECT_COMMIT*}${EXPECT_VERSION:+version $EXPECT_VERSION}"
if [[ "$DRY_RUN" == "1" ]]; then say "DRY RUN — nothing is changed"; fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# json_field <name> — read a top-level string field from JSON on stdin.
json_field() { sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -n1; }

# version_matches <json> — does the reported version/commit match the tag?
version_matches() {
  local body=$1 commit version
  commit=$(json_field commit <<<"$body")
  version=$(json_field version <<<"$body")
  if [[ -n "$EXPECT_COMMIT" ]]; then
    [[ -n "$commit" && "$commit" == "$EXPECT_COMMIT"* ]] || return 1
  fi
  if [[ -n "$EXPECT_VERSION" ]]; then
    [[ "$version" == "$EXPECT_VERSION" ]] || return 1
  fi
}

# ── 2. Tag + start the inactive slot ───────────────────────
SLOT_VAR="${INACTIVE^^}_TAG"
OLD_TAG_LINE=$(grep -E "^${SLOT_VAR}=" "$ENV_FILE" || true)

set_tag() { # set_tag <line>
  if [[ "$DRY_RUN" == "1" ]]; then echo "DRY: set '$1' in $ENV_FILE"; return; fi
  if grep -qE "^${SLOT_VAR}=" "$ENV_FILE"; then
    sed -i "s|^${SLOT_VAR}=.*|$1|" "$ENV_FILE"
  else
    echo "$1" >>"$ENV_FILE"
  fi
}

UPSTREAM_SWITCHED=0
UPSTREAM_BACKUP="${UPSTREAM_FILE}.bak"

rollback() {
  local why=$1
  echo "✗  $why — rolling back" >&2
  if [[ $UPSTREAM_SWITCHED -eq 1 ]]; then
    run $SUDO cp -f "$UPSTREAM_BACKUP" "$UPSTREAM_FILE"
    if $SUDO nginx -t >/dev/null 2>&1; then
      run $SUDO $NGINX_RELOAD || true
      echo "   nginx upstream restored" >&2
    else
      echo "   !! nginx -t fails even after the restore — check $UPSTREAM_FILE by hand" >&2
    fi
  fi
  run "${COMPOSE[@]}" --profile "$INACTIVE" stop "online-$INACTIVE" || true
  if [[ -n "$OLD_TAG_LINE" ]]; then set_tag "$OLD_TAG_LINE" || true; fi
  echo "   still live: ${ACTIVE:-the previous setup}" >&2
  exit 1
}

set_tag "${SLOT_VAR}=${TAG}"
ok "${SLOT_VAR}=${TAG} in ${ENV_FILE}"

if [[ "$SKIP_PULL" != "1" ]]; then
  run "${COMPOSE[@]}" --profile "$INACTIVE" pull "online-$INACTIVE" || rollback "pull of $TAG failed"
  ok "pulled $TAG"
fi
run "${COMPOSE[@]}" --profile "$INACTIVE" up -d --no-deps --force-recreate "online-$INACTIVE" \
  || rollback "could not start online-$INACTIVE"
ok "started online-$INACTIVE"

# ── 3. Health + version on the slot ────────────────────────
say "waiting for 127.0.0.1:$NEW_PORT/health ..."
healthy=0
for i in $(seq 1 "$HEALTH_RETRIES"); do
  if curl -fsS -m 3 "http://127.0.0.1:$NEW_PORT/health" >/dev/null 2>&1; then
    healthy=1; ok "health passed (attempt $i)"; break
  fi
  echo "   attempt $i/$HEALTH_RETRIES — not ready yet"
  sleep "$HEALTH_INTERVAL"
done
[[ $healthy -eq 1 ]] || rollback "online-$INACTIVE never became healthy"

body=$(curl -fsS -m 3 "http://127.0.0.1:$NEW_PORT/api/v1/version" 2>/dev/null || true)
version_matches "$body" || rollback "online-$INACTIVE reports '${body:-nothing}', not $TAG"
ok "online-$INACTIVE reports $body"

# ── 4. Switch the nginx upstream ───────────────────────────
NEW_UPSTREAM="upstream ${UPSTREAM_NAME} { server 127.0.0.1:${NEW_PORT}; }  # written by online/deploy/deploy.sh — slot ${INACTIVE}, ${TAG}"
TMP_UPSTREAM="$(dirname "$UPSTREAM_FILE")/.$(basename "$UPSTREAM_FILE").new"   # not *.conf: nginx won't include it
run $SUDO cp -f "$UPSTREAM_FILE" "$UPSTREAM_BACKUP"
if [[ "$DRY_RUN" == "1" ]]; then
  echo "DRY: write $UPSTREAM_FILE: $NEW_UPSTREAM"
else
  printf '%s\n' "$NEW_UPSTREAM" | $SUDO tee "$TMP_UPSTREAM" >/dev/null
  $SUDO mv -f "$TMP_UPSTREAM" "$UPSTREAM_FILE"   # atomic rename
fi
UPSTREAM_SWITCHED=1
$SUDO nginx -t >/dev/null 2>&1 || rollback "nginx -t failed with the new upstream"
run $SUDO $NGINX_RELOAD || rollback "nginx reload failed"
ok "nginx → online-$INACTIVE (127.0.0.1:$NEW_PORT)"

# ── 5. Check through nginx ─────────────────────────────────
through=0
for i in $(seq 1 10); do
  body=$(curl -fsS -m 5 --resolve "${PUBLIC_HOST}:443:127.0.0.1" "https://${PUBLIC_HOST}/api/v1/version" 2>/dev/null || true)
  if version_matches "$body"; then through=1; break; fi
  sleep 1
done
[[ $through -eq 1 ]] || rollback "https://${PUBLIC_HOST} does not serve $TAG (got '${body:-nothing}')"
ok "https://${PUBLIC_HOST} serves $TAG"

# ── 6. Stop the old slot, save state ───────────────────────
if [[ -n "$ACTIVE" ]]; then
  run "${COMPOSE[@]}" --profile "$ACTIVE" stop "online-$ACTIVE"
  ok "stopped online-$ACTIVE"
else
  say "first blue-green deploy: stop the old single-slot online container by hand (deploy/README.md)"
fi
if [[ "$DRY_RUN" == "1" ]]; then
  echo "DRY: write $INACTIVE to $STATE_FILE"
else
  echo "$INACTIVE" >"$STATE_FILE"
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [[ "$DRY_RUN" == "1" ]]; then echo "✅  Dry run complete — nothing was changed"; else echo "✅  Deploy complete"; fi
echo "    Active slot : $INACTIVE (127.0.0.1:$NEW_PORT)"
echo "    Image tag   : $TAG"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
