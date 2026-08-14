#!/bin/bash
set -e

# Prerequisites:
# 1. Backend, Postgres, Redis, and Docker daemon running locally
# 2. JWT_SECRET set in the backend environment
# 3. This script will exit non-zero if the environment stays BUILDING forever or if any orphans remain.

echo "=========================================="
echo " Starting Tier 0-2 E2E & Orphan Test "
echo "=========================================="

echo "[1/6] Registering unique user..."
USER_ID="e2e-$(date +%s)"
USER_EMAIL="${USER_ID}@example.com"
USER_PASS="VeryLongPassword123!"

curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d "{\"email\": \"${USER_EMAIL}\", \"password\": \"${USER_PASS}\", \"username\": \"${USER_ID}\"}" > /dev/null

echo "[2/6] Verifying user via DB & logging in..."
docker exec frontend-postgres-1 psql -U postgres -d api_sandbox -c "UPDATE users SET is_email_verified = true WHERE email = '${USER_EMAIL}';" > /dev/null

curl -s -c cookies.txt -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"email\": \"${USER_EMAIL}\", \"password\": \"${USER_PASS}\"}" > /dev/null

echo "[3/6] Creating Environment (with Postgres sidecar)..."
ENV_JSON=$(curl -s -b cookies.txt -X POST http://localhost:8080/api/environments \
  -H "Content-Type: application/json" \
  -d '{"name": "E2E Orphan Test", "gitUrl": "https://github.com/AyushCN/api-sandbox-links-example", "databaseUrl": "postgres"}')
  
ENV_ID=$(echo $ENV_JSON | jq -r '.id')
echo "      -> Environment ID: $ENV_ID"

if [ "$ENV_ID" == "null" ] || [ -z "$ENV_ID" ]; then
    echo "      -> ❌ Failed to create environment: $ENV_JSON"
    exit 1
fi

echo "[4/6] Waiting for build to finish (timeout 120s)..."
STATUS="BUILDING"
for i in {1..60}; do
  STATUS=$(curl -s -b cookies.txt -X GET http://localhost:8080/api/environments/$ENV_ID | jq -r '.status')
  if [ "$STATUS" == "RUNNING" ] || [ "$STATUS" == "FAILED" ]; then
    echo "      -> Status reached: $STATUS"
    break
  fi
  sleep 2
done

if [ "$STATUS" == "BUILDING" ]; then
    echo "      -> ❌ Timeout: Environment stuck in BUILDING!"
    exit 1
fi

echo "[5/6] Fetching Docker Logs..."
curl -s -b cookies.txt -X GET http://localhost:8080/api/environments/$ENV_ID/docker-logs | head -n 5

echo "[6/6] Deleting Environment..."
DELETE_RESP=$(curl -s -w "\n%{http_code}" -b cookies.txt -X DELETE http://localhost:8080/api/environments/$ENV_ID)
HTTP_CODE=$(echo "$DELETE_RESP" | tail -n1)
if [ "$HTTP_CODE" != "200" ]; then
    echo "      -> ❌ Delete failed with code: $HTTP_CODE"
    exit 1
fi
echo "      -> Deleted successfully."

echo "=========================================="
echo " Verifying Orphan Teardown "
echo "=========================================="

# Wait a second for async daemon cleanup
sleep 2

ORPHANS=$(docker ps -a -q -f name=api-sandbox-env-$ENV_ID -f name=api-sandbox-db-$ENV_ID)
if [ -n "$ORPHANS" ]; then
  echo "❌ FAIL: Orphan containers found!"
  echo "$ORPHANS"
  exit 1
fi

echo "✅ SUCCESS: No orphan containers found."

# Verify workspace is gone
WORKSPACE_DIR="../backend/workspaces/$ENV_ID"
if [ -d "$WORKSPACE_DIR" ]; then
  echo "❌ FAIL: Workspace directory was not deleted! ($WORKSPACE_DIR)"
  exit 1
fi

echo "✅ SUCCESS: Workspace directory cleanly deleted."
echo "=========================================="
echo " Tier 0-2 E2E COMPLETE "
echo "=========================================="
exit 0
