#!/usr/bin/env bash
# Usage: ./scripts/get-token.sh [orders-agent|orders-readonly] [scopes]
# Prints an access token from the local Keycloak demo realm.
set -euo pipefail
CLIENT_ID="${1:-orders-readonly}"
SCOPE="${2:-orders:read}"
SECRET="${CLIENT_ID}-secret"   # demo realm convention, local only
curl -s -X POST "http://localhost:8180/realms/orders/protocol/openid-connect/token" \
  -d grant_type=client_credentials -d client_id="$CLIENT_ID" -d client_secret="$SECRET" \
  -d scope="$SCOPE" | sed -E 's/.*"access_token":"([^"]+)".*/\1/'
