#!/usr/bin/env bash
# Turns on in-app sign-in, password reset and the one-tap demo logins on the HEKATE server.
# Run as root on the server:  sudo bash /opt/hekate/enable_demo.sh
set -euo pipefail
set -a; . /etc/hekate/keycloak.env; . /etc/hekate/api.env; set +a

KC=/opt/hekate/keycloak/bin/kcadm.sh
CFG=$(mktemp)
trap 'rm -f "$CFG"' EXIT
URL="https://${KC_HOSTNAME#https://}"
URL="${URL%/}"

echo "1/4 Keycloak: signing in as the admin"
"$KC" config credentials --config "$CFG" --server "$URL" --realm master --user "$KC_BOOTSTRAP_ADMIN_USERNAME" --password "$KC_BOOTSTRAP_ADMIN_PASSWORD" >/dev/null

echo "2/4 Keycloak: allowing in-app sign-in (hekate-mobile) and Forgot password"
CID=$("$KC" get clients --config "$CFG" -r hekate -q clientId=hekate-mobile --fields id --format csv --noquotes)
"$KC" update "clients/$CID" --config "$CFG" -r hekate -s directAccessGrantsEnabled=true
"$KC" update realms/hekate --config "$CFG" -s resetPasswordAllowed=true

echo "3/4 Creating the demo accounts and sample data"
OUT=$(KC_URL="$URL" KC_ADMIN="$KC_BOOTSTRAP_ADMIN_USERNAME" KC_ADMIN_PASSWORD="$KC_BOOTSTRAP_ADMIN_PASSWORD" python3 "$(dirname "$0")/create_demo_logins.py")
echo "$OUT" | grep -v '^HEKATE_DEMO_LOGINS='
LINE=$(echo "$OUT" | grep '^HEKATE_DEMO_LOGINS=')

echo "4/4 Switching the demo buttons on and restarting the API"
sed -i '/^HEKATE_DEMO_LOGINS=/d' /etc/hekate/api.env
echo "$LINE" >> /etc/hekate/api.env
systemctl restart hekate-api
sleep 4
curl -s http://127.0.0.1:18080/v1/public/demo-logins | python3 -c 'import json,sys; print("Demo buttons now served:", ", ".join(d["label"] for d in json.load(sys.stdin)["data"]))'
