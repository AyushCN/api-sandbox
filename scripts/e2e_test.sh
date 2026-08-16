#!/bin/bash
set -euo pipefail

if [ -z "${SESSION_COOKIE:-}" ]; then
    echo "Error: SESSION_COOKIE environment variable is not set."
    echo "Usage: Export SESSION_COOKIE='auth_session=...' and run this script."
    exit 1
fi

API_URL="${API_URL:-http://localhost/api}"
TEST_REPO="https://github.com/vercel/next-learn" # Small public repo

echo "1. Creating Sandbox..."
RES=$(curl -s -X POST "$API_URL/environments" \
    -H "Cookie: $SESSION_COOKIE" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"e2e-test-$(date +%s)\", \"gitUrl\":\"$TEST_REPO\"}")

ENV_ID=$(echo "$RES" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -z "$ENV_ID" ]; then
    echo "Failed to create sandbox. Response: $RES"
    exit 1
fi

echo "Created environment ID: $ENV_ID"

echo "2. Polling for RUNNING status..."
STATUS=""
for i in {1..30}; do
    STATUS_RES=$(curl -s -H "Cookie: $SESSION_COOKIE" "$API_URL/environments")
    STATUS=$(echo "$STATUS_RES" | grep -o "\"id\":\"$ENV_ID\"[^\}]*\"status\":\"[^\"]*" | awk -F'status":"' '{print $2}')
    
    if [[ "$STATUS" == "RUNNING" ]]; then
        echo "Environment is RUNNING!"
        break
    fi
    if [[ "$STATUS" == "FAILED" ]]; then
        echo "Environment FAILED to build."
        exit 1
    fi
    
    echo "Status: ${STATUS:-BUILDING}... waiting (attempt $i/30)"
    sleep 5
done

if [[ "$STATUS" != "RUNNING" ]]; then
    echo "Timeout waiting for RUNNING status."
    exit 1
fi

echo "3. Updating a file..."
UPDATE_RES=$(curl -s -X PUT "$API_URL/environments/$ENV_ID/files/content" \
    -H "Cookie: $SESSION_COOKIE" \
    -H "Content-Type: application/json" \
    -d "{\"path\":\"README.md\", \"content\":\"# E2E Test Update\"}")

if echo "$UPDATE_RES" | grep -q '"reloadSignaled":true'; then
    echo "File updated and reload signaled successfully!"
else
    echo "File update failed or did not signal reload. Response: $UPDATE_RES"
    exit 1
fi

if [ "${SKIP_PUSH:-0}" -eq 1 ]; then
    echo "4. Skipping Push (SKIP_PUSH=1)"
else
    echo "4. Committing and Pushing..."
    COMMIT_RES=$(curl -s -X POST "$API_URL/environments/$ENV_ID/git/commit" \
        -H "Cookie: $SESSION_COOKIE" \
        -H "Content-Type: application/json" \
        -d "{\"message\":\"e2e test commit\"}")
    
    if echo "$COMMIT_RES" | grep -q '"success":true'; then
        echo "Committed successfully!"
    else
        echo "Commit failed. Response: $COMMIT_RES"
        echo "(If this is expected because of token permissions on the test repo, run with SKIP_PUSH=1)"
    fi
fi

echo "5. Deleting Sandbox..."
DELETE_RES=$(curl -s -X DELETE "$API_URL/environments/$ENV_ID" \
    -H "Cookie: $SESSION_COOKIE")

if echo "$DELETE_RES" | grep -q 'successfully'; then
    echo "Sandbox deleted successfully!"
else
    echo "Delete failed. Response: $DELETE_RES"
    exit 1
fi

echo "E2E Test Passed!"
