#!/usr/bin/env bash
# gen-certs.sh — generate a self-signed CA and per-service mTLS certificates.
#
# Output: certs/ directory (gitignored — never commit private keys).
# Usage:  bash scripts/gen-certs.sh   or   make certs
#
# Each leaf cert includes SANs for:
#   DNS:<service-name>  — matches the Kubernetes Service hostname
#   DNS:localhost       — enables local / docker-compose testing
set -euo pipefail

SERVICES=(light-service plant-service dashboard-service)
DAYS_CA=3650   # ~10 years for the dev CA
DAYS_LEAF=365  # rotate leaves annually

mkdir -p certs

# ── CA ────────────────────────────────────────────────────────────────────────
echo "Generating CA..."
openssl genrsa -out certs/ca.key 4096 2>/dev/null
openssl req -new -x509 \
  -key  certs/ca.key \
  -out  certs/ca.crt \
  -days "$DAYS_CA" \
  -subj "/CN=plant-monitor-ca" \
  2>/dev/null
echo "  ca.crt / ca.key"

# ── Per-service leaf certificates ─────────────────────────────────────────────
for SERVICE in "${SERVICES[@]}"; do
  echo "Generating $SERVICE..."
  openssl genrsa -out "certs/$SERVICE.key" 2048 2>/dev/null

  openssl req -new \
    -key  "certs/$SERVICE.key" \
    -out  "certs/$SERVICE.csr" \
    -subj "/CN=$SERVICE" \
    2>/dev/null

  openssl x509 -req \
    -in            "certs/$SERVICE.csr" \
    -CA            certs/ca.crt \
    -CAkey         certs/ca.key \
    -CAcreateserial \
    -out           "certs/$SERVICE.crt" \
    -days          "$DAYS_LEAF" \
    -extfile       <(printf "subjectAltName=DNS:%s,DNS:localhost" "$SERVICE") \
    2>/dev/null

  rm "certs/$SERVICE.csr"
  echo "  $SERVICE.crt / $SERVICE.key"
done

echo ""
echo "Done. Certificates written to certs/"
echo "Next: make k8s-certs   (to create the Kubernetes TLS secret)"
