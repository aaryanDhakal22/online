#!/bin/sh
set -ex

MODE="${1:-init}"

: "${DOMAIN:?DOMAIN is required}"
: "${LE_EMAIL:?LE_EMAIL is required}"
: "${CF_API_TOKEN:?CF_API_TOKEN is required}"

LE_DIR="/etc/letsencrypt"
LIVE_DIR="$LE_DIR/live/${DOMAIN}"
CREDS_FILE="/run/secrets/cloudflare.ini"

write_cloudflare_ini() {
    mkdir -p /run/secrets

    cat > "${CREDS_FILE}" <<EOF
dns_cloudflare_api_token = ${CF_API_TOKEN}
EOF

    chmod 600 "${CREDS_FILE}"
}

have_cert() {
  [ -f "$LIVE_DIR/fullchain.pem" ] && [ -f "$LIVE_DIR/privkey.pem" ]
}

issue_cert_if_missing() {
    if have_cert; then
        echo "Certificate already exists for ${DOMAIN}"
        return 0
    fi

    echo "No certificate found for ${DOMAIN}. Issuing one..."
    write_cloudflare_ini

    certbot certonly \
        --non-interactive \
        --agree-tos \
        --email "${LE_EMAIL}" \
        --dns-cloudflare \
        --dns-cloudflare-credentials "${CREDS_FILE}" \
        --dns-cloudflare-propagation-seconds 30 \
        -d "${DOMAIN}" \
        -d "*.${DOMAIN}"

    echo "Certificate issued for ${DOMAIN}"
}

renew_once(){
    write_cloudflare_ini

    certbot renew \
        --non-interactive \
        --agree-tos \
        --dns-cloudflare \
        --dns-cloudflare-credentials "${CREDS_FILE}" \
        --dns-cloudflare-propagation-seconds 30 
}

case "${MODE}" in
    init)
        echo "== certbot-init.sh =="
        issue_cert_if_missing
        ;;

    renew)
        echo "== certbot-init.sh renew loop=="
        issue_cert_if_missing

        while :; do
            echo "Running renew..."
            renew_once || true
            echo "Renew complete."
            sleep 12h
        done
        ;;
    *)
        echo "Unknown mode: ${MODE}. Use: init or renew" >&2
        exit 1
        ;;
esac

