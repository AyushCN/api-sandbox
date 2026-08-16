#!/bin/bash
set -e

# Default URL
APP_URL=${APP_URL:-"http://localhost:8080"}

if [ -z "$SESSION_COOKIE" ]; then
    echo "Error: SESSION_COOKIE is required."
    echo "Usage: SESSION_COOKIE=your_jwt_cookie ./scripts/e2e_session.sh"
    exit 1
fi

echo "Creating Environment..."
ENV_JSON=$(curl -s -X POST "$APP_URL/api/environments" \
  -H "Cookie: token=$SESSION_COOKIE" \
  -H "Content-Type: application/json" \
  -d '{"name": "Session E2E Test", "gitUrl": "https://github.com/expressjs/express", "projectId": "96aa4613-6f23-4914-afb3-2a7baedd8e01"}')
  
ENV_ID=$(echo "$ENV_JSON" | jq -r '.id')
echo "Environment ID: $ENV_ID"

if [ "$ENV_ID" == "null" ] || [ -z "$ENV_ID" ]; then
    echo "Failed to create environment: $ENV_JSON"
    exit 1
fi

echo "Disabling TCP Health Check via Settings API..."
curl -s -X PUT "$APP_URL/api/environments/$ENV_ID/settings" \
  -H "Cookie: token=$SESSION_COOKIE" \
  -H "Content-Type: application/json" \
  -d '{"healthCheckType": "none"}'

echo "Waiting for build to finish..."
STATUS="BUILDING"
for i in {1..60}; do
  STATUS=$(curl -s -X GET "$APP_URL/api/environments/$ENV_ID" -H "Cookie: token=$SESSION_COOKIE" | jq -r '.status')
  if [ "$STATUS" == "RUNNING" ] || [ "$STATUS" == "FAILED" ]; then
    echo "Status reached: $STATUS"
    break
  fi
  sleep 2
done

if [ "$STATUS" != "RUNNING" ]; then
    echo "Error: Environment failed to run. Status: $STATUS"
    curl -s -X GET "$APP_URL/api/environments/$ENV_ID/docker-logs" -H "Cookie: token=$SESSION_COOKIE" | head -n 20
    exit 1
fi

echo "Testing file edit (touch)..."
TOUCH_RES=$(curl -s -X POST "$APP_URL/api/environments/$ENV_ID/touch" -H "Cookie: token=$SESSION_COOKIE")
if echo "$TOUCH_RES" | grep -q "error"; then
    echo "Failed to touch file: $TOUCH_RES"
    exit 1
else
    echo "Touch successful."
fi

if [ -z "$SKIP_PUSH" ]; then
    echo "Testing git push..."
    PUSH_RES=$(curl -s -X POST "$APP_URL/api/environments/$ENV_ID/push" -H "Cookie: token=$SESSION_COOKIE")
    if echo "$PUSH_RES" | grep -q "error"; then
        echo "Push returned an error (expected if no GitHub token or read-only repo). Use SKIP_PUSH=1 to ignore."
        echo "Push response: $PUSH_RES"
    else
        echo "Push successful."
    fi
else
    echo "Skipping git push due to SKIP_PUSH."
fi

echo "Deleting Environment..."
DELETE_RES=$(curl -s -X DELETE "$APP_URL/api/environments/$ENV_ID" -H "Cookie: token=$SESSION_COOKIE")
if echo "$DELETE_RES" | grep -q "error"; then
    echo "Failed to delete environment: $DELETE_RES"
    exit 1
fi

echo "Checking orphan containers..."
ORPHANS=$(docker ps -a -q -f name=api-sandbox-env-$ENV_ID -f name=api-sandbox-db-$ENV_ID)
if [ -z "$ORPHANS" ]; then
  echo "Success: No orphan containers found!"
else
  echo "Error: Orphans found: $ORPHANS"
  exit 1
fi

echo "E2E Session Test completed successfully."
exit 0
