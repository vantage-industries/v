#!/bin/sh
# Vendored from security-hub/backend's own deploy/nginx/generate-cert.sh, unmodified --
# defaults (CERT_DIR=/etc/nginx/ssl) already match this recipe's nginx vhost. Run once at
# boot by security-hub-cert.service, before nginx starts.
set -eu

CERT_DIR="${CERT_DIR:-/etc/nginx/ssl}"
CERT="$CERT_DIR/securityhub.crt"
KEY="$CERT_DIR/securityhub.key"
DAYS="${DAYS:-825}"

detect_sans() {
    printf 'DNS:securityhub.lan,DNS:vantageos,DNS:%s,DNS:localhost' "$(hostname)"
    ip -4 -o addr show 2>/dev/null \
        | awk '{split($4,a,"/"); print a[1]}' \
        | sort -u \
        | while read -r addr; do printf ',IP:%s' "$addr"; done
    printf ',IP:127.0.0.1\n'
}

SANS="${SANS:-$(detect_sans)}"

if [ -f "$CERT" ] && [ "${FORCE:-0}" != "1" ]; then
    echo "$CERT already exists; re-run with FORCE=1 to replace it." >&2
    openssl x509 -in "$CERT" -noout -enddate -ext subjectAltName
    exit 0
fi

mkdir -p "$CERT_DIR"

# ECDSA P-256: smaller and faster than RSA on a Pi, and universally supported by
# anything that can speak TLS 1.2.
openssl req -x509 -nodes \
    -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
    -days "$DAYS" \
    -subj "/CN=securityhub.lan/O=VantageOS SecurityHub" \
    -addext "subjectAltName=$SANS" \
    -addext "keyUsage=critical,digitalSignature,keyEncipherment" \
    -addext "extendedKeyUsage=serverAuth" \
    -addext "basicConstraints=critical,CA:FALSE" \
    -keyout "$KEY" -out "$CERT"

chmod 600 "$KEY"
chmod 644 "$CERT"

echo "Wrote $CERT and $KEY"
openssl x509 -in "$CERT" -noout -subject -enddate -ext subjectAltName
