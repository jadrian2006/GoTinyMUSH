#!/bin/bash
# Quick REST API helper for CrystalMUSH
API="http://localhost:8443/api/v1"

# Login and get token
TOKEN=$(curl -s "$API/auth/login" -X POST \
  -H "Content-Type: application/json" \
  -d '{"name":"otter","password":"crystal"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

if [ -z "$TOKEN" ]; then
  echo "Login failed"
  exit 1
fi

# Execute the requested endpoint
ENDPOINT="${1:-objects/123}"
curl -s "$API/$ENDPOINT" -H "Authorization: Bearer $TOKEN"
