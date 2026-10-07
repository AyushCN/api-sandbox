#!/usr/bin/env bash
set -euo pipefail

if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon is unavailable" >&2
  exit 2
fi

root=$(mktemp -d)
a_name="api-sandbox-isolation-a-$$"
b_name="api-sandbox-isolation-b-$$"
network="api-sandbox-isolation-net-$$"
cleanup() {
  docker rm -f "$a_name" "$b_name" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$root"
}
trap cleanup EXIT
mkdir -p "$root/a" "$root/b"
printf 'environment-a\n' > "$root/a/A-private.txt"
printf 'environment-b\n' > "$root/b/B-private.txt"
docker network create "$network" >/dev/null

for spec in "a:$a_name" "b:$b_name"; do
  env_dir=${spec%%:*}
  name=${spec#*:}
  docker run -d --name "$name" --network "$network" --memory 512m --memory-swap 512m --cpus 1 --pids-limit 256 \
    --security-opt no-new-privileges:true --cap-drop ALL \
    -v "$root/$env_dir:/app:rw" alpine:3.20 sleep 300 >/dev/null
done

# Assert Docker's recorded mount source and effective resource configuration.
for spec in "a:$a_name" "b:$b_name"; do
  env_dir=${spec%%:*}
  name=${spec#*:}
  source=$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/app"}}{{.Source}}{{end}}{{end}}' "$name")
  test "$source" = "$root/$env_dir"
  test "$(docker inspect -f '{{.HostConfig.Memory}}' "$name")" = 536870912
  test "$(docker inspect -f '{{.HostConfig.MemorySwap}}' "$name")" = 536870912
  test "$(docker inspect -f '{{.HostConfig.PidsLimit}}' "$name")" = 256
  test "$(docker inspect -f '{{.HostConfig.NanoCpus}}' "$name")" = 1000000000
  test "$(docker inspect -f '{{.Config.User}}' "$name")" = ""
  test "$(docker exec "$name" id -u)" = 0
  test "$(docker inspect -f '{{.HostConfig.CapDrop}}' "$name")" = '[ALL]'
  test "$(docker inspect -f '{{.HostConfig.SecurityOpt}}' "$name")" = '[no-new-privileges:true]'
  test "$(docker inspect -f '{{.HostConfig.Privileged}}' "$name")" = false
  test "$(docker inspect -f '{{.HostConfig.PidMode}}' "$name")" = ""
  test "$(docker inspect -f '{{.HostConfig.IpcMode}}' "$name")" != host
  test "$(docker inspect -f '{{.HostConfig.Devices}}' "$name")" = '[]'
  test "$(docker inspect -f '{{.HostConfig.NetworkMode}}' "$name")" = "$network"
  test "$(docker inspect -f '{{range $networkName, $networkInfo := .NetworkSettings.Networks}}{{$networkName}}{{end}}' "$name")" = "$network"
  test "$(docker exec "$name" cat "/app/$( [ "$env_dir" = a ] && echo A-private.txt || echo B-private.txt)")" = "environment-$env_dir"
  if docker exec "$name" test -e "/app/$( [ "$env_dir" = a ] && echo B-private.txt || echo A-private.txt)"; then
    echo "$name can read the other environment's file" >&2
    exit 1
  fi
  if docker exec "$name" test -e /workspaces; then
    echo "$name unexpectedly has a /workspaces mount" >&2
    exit 1
  fi
  docker exec "$name" sh -c 'printf "root: "; ls -A / | tr "\n" " "; printf "\n/app: "; ls -A /app | tr "\n" " "; printf "\n"'
  docker inspect -f '{{.Name}} mounts={{json .Mounts}} memory={{.HostConfig.Memory}} pids={{.HostConfig.PidsLimit}} nanoCPUs={{.HostConfig.NanoCpus}}' "$name"
done

echo "Docker workspace isolation and resource assertions passed"
docker stop "$a_name" "$b_name" >/dev/null
docker rm "$a_name" "$b_name" >/dev/null
if docker inspect "$a_name" "$b_name" >/dev/null 2>&1; then
  echo "Docker cleanup left test containers behind" >&2
  exit 1
fi
echo "Docker stop/remove cleanup assertions passed"
