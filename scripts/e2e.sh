#!/bin/bash
set -e

echo "Logging in..."
curl -s -c cookies.txt -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "e2e@example.com", "password": "VeryLongPassword123!"}' > /dev/null

echo "Creating Environment..."
ENV_JSON=$(curl -s -b cookies.txt -X POST http://localhost:8080/api/environments \
  -H "Content-Type: application/json" \
  -d '{"name": "E2E Test", "gitUrl": "https://github.com/AyushCN/api-sandbox-links-example"}')
  
ENV_ID=$(echo $ENV_JSON | jq -r '.id')
echo "Environment ID: $ENV_ID"

if [ "$ENV_ID" == "null" ] || [ -z "$ENV_ID" ]; then
    echo "Failed to create environment: $ENV_JSON"
    exit 1
fi

echo "Waiting for build to finish..."
for i in {1..60}; do
  STATUS=$(curl -s -b cookies.txt -X GET http://localhost:8080/api/environments/$ENV_ID | jq -r '.status')
  if [ "$STATUS" == "RUNNING" ] || [ "$STATUS" == "FAILED" ]; then
    echo "Status reached: $STATUS"
    break
  fi
  sleep 2
done

echo "Fetching Docker logs..."
curl -s -b cookies.txt -X GET http://localhost:8080/api/environments/$ENV_ID/docker-logs | head -n 5

echo "Deleting Environment..."
curl -s -b cookies.txt -X DELETE http://localhost:8080/api/environments/$ENV_ID

echo "Checking orphan containers..."
ORPHANS=$(docker ps -a -q -f name=api-sandbox-env-$ENV_ID -f name=api-sandbox-db-$ENV_ID)
if [ -z "$ORPHANS" ]; then
  echo "No orphan containers found!"
else
  echo "Orphans found: $ORPHANS"
fi
