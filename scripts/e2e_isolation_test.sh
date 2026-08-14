#!/bin/bash
set -e

# Test 7.2: Network Isolation
# This script creates two separate Docker bridge networks (simulating Org A and Org B),
# spawns a container in each, and proves that cross-tenant routing is impossible.

echo "Running Network Isolation Test..."

docker network create test-org-a-net > /dev/null
docker network create test-org-b-net > /dev/null

echo "Spawning Env A in Org A..."
docker run -d --name env-a --network test-org-a-net alpine sleep infinity > /dev/null

echo "Spawning Env B in Org B..."
docker run -d --name env-b --network test-org-b-net alpine sleep infinity > /dev/null

ENV_B_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' env-b)
echo "Env B IP Address: $ENV_B_IP"

echo "Attempting to ping Env B from Env A..."
set +e
docker exec env-a ping -c 1 -W 2 $ENV_B_IP
PING_RESULT=$?
set -e

if [ $PING_RESULT -ne 0 ]; then
  echo "✅ Ping failed as expected (100% packet loss). Isolation enforced."
else
  echo "❌ CRITICAL: Env A successfully pinged Env B!"
  exit 1
fi

echo "Attempting to reach host gateway (port 5432) from Env A..."
# We assume default bridge gateway or docker0 isn't bound, but we can test reaching 127.0.0.1 on the host
set +e
docker exec env-a wget -q --timeout=2 http://172.17.0.1:5432
WGET_RESULT=$?
set -e

if [ $WGET_RESULT -ne 0 ]; then
  echo "✅ Host connection failed as expected. Isolation enforced."
else
  echo "❌ CRITICAL: Env A reached host loopback/gateway!"
  exit 1
fi

echo "Cleaning up..."
docker rm -f env-a env-b > /dev/null
docker network rm test-org-a-net test-org-b-net > /dev/null

echo "All isolation tests passed."
