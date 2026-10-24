#!/usr/bin/env bash
# End-to-end tests against real databases, all inside Docker.
#
# A throw-away Docker network holds: the database, the proxy (the freshly built
# linux binary mounted into a tiny container) and a client container that runs the
# database's own CLI. Nothing is installed on the host except Docker and Go.
#
#   scripts/e2e.sh                        # postgres, redis, clickhouse
#   E2E_ONLY="postgres redis" scripts/e2e.sh
#
# Image overrides (useful offline or on a slow connection):
#   E2E_PROXY_IMAGE  any image with a shell-less static exec capability (default busybox:1.37)
#   E2E_PG_IMAGE     PostgreSQL server and psql, version 17 or newer for direct TLS (default postgres:17-alpine)
#   E2E_REDIS_IMAGE  Redis server and redis-cli (default redis:7-alpine)
#   E2E_CH_IMAGE     ClickHouse server and clickhouse-client (default clickhouse/clickhouse-server:latest-alpine)
#   E2E_MONGO_IMAGE  MongoDB server and mongosh (default mongo:8)
set -uo pipefail
cd "$(dirname "$0")/.."

PROXY_IMAGE=${E2E_PROXY_IMAGE:-busybox:1.37}
PG_IMAGE=${E2E_PG_IMAGE:-postgres:17-alpine}
REDIS_IMAGE=${E2E_REDIS_IMAGE:-redis:7-alpine}
CH_IMAGE=${E2E_CH_IMAGE:-clickhouse/clickhouse-server:latest-alpine}
MONGO_IMAGE=${E2E_MONGO_IMAGE:-mongo:8}
ONLY=${E2E_ONLY:-"postgres redis clickhouse mongodb"}
ALIASES=() # extra "docker run" args for the next proxy (network aliases used as SNI host names)

NET="xdp-e2e-$$"
WORK=$(mktemp -d)
FAILED=0

cleanup() {
  docker ps -aq --filter "label=xdp-e2e=$$" | xargs -r docker rm -f >/dev/null 2>&1
  docker network rm "$NET" >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

arch=$(docker version --format '{{.Server.Arch}}')
echo "building linux/$arch proxy binary"
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$WORK/xdatabase-proxy" ./cmd/proxy || exit 1
docker network create "$NET" >/dev/null || exit 1

# run_bg NAME IMAGE [docker run args...] -- [command...]
run_bg() {
  local name=$1 image=$2; shift 2
  docker run -d --name "$name" --network "$NET" --label "xdp-e2e=$$" "$@" >/dev/null || return 1
}

# start_proxy NAME ENV... : proxy container running the freshly built binary
start_proxy() {
  local name=$1; shift
  local envs=()
  for e in "$@"; do envs+=(-e "$e"); done
  docker run -d --name "$name" --network "$NET" --label "xdp-e2e=$$" -v "$WORK/xdatabase-proxy:/xdp:ro" \
    "${ALIASES[@]}" "${envs[@]}" "$PROXY_IMAGE" /xdp >/dev/null || return 1
  for _ in $(seq 50); do
    docker logs "$name" 2>&1 | grep -q '"msg":"listening"' && return 0
    sleep 0.2
  done
  echo "proxy $name did not start:"; docker logs "$name" 2>&1 | tail -5
  return 1
}

# wait_for DESCRIPTION [CONTAINER] CMD... : retry a command for up to 60 s.
# With a CONTAINER name (when the second argument names a running container) its log is printed on timeout.
wait_for() {
  local what=$1 ctr=""; shift
  if docker inspect "$1" >/dev/null 2>&1; then ctr=$1; shift; fi
  for _ in $(seq 120); do "$@" >/dev/null 2>&1 && return 0; sleep 0.5; done
  echo "timeout waiting for $what"
  if [ -n "$ctr" ]; then
    echo "--- last command output:"; "$@" 2>&1 | tail -5
    echo "--- container state: $(docker inspect -f '{{.State.Status}} exit={{.State.ExitCode}}' "$ctr")"
    echo "--- container log:"; docker logs "$ctr" 2>&1 | tail -15
  fi
  return 1
}

# client IMAGE ARGS... : run a CLI in a container on the test network
client() { local image=$1; shift; docker run --rm --network "$NET" "$image" "$@"; }

# check NAME EXPECTED CMD... : the command output must contain EXPECTED
check() {
  local name=$1 want=$2; shift 2
  local out
  if out=$("$@" 2>&1) && grep -qF -- "$want" <<<"$out"; then
    echo "  PASS  $name"
  else
    echo "  FAIL  $name (wanted: $want)"; sed 's/^/        /' <<<"$out" | head -8
    FAILED=1
  fi
}

# check_fails NAME EXPECTED CMD... : the command must fail and print EXPECTED
check_fails() {
  local name=$1 want=$2; shift 2
  local out
  if out=$("$@" 2>&1); then
    echo "  FAIL  $name (command succeeded)"; FAILED=1
  elif grep -qF -- "$want" <<<"$out"; then
    echo "  PASS  $name"
  else
    echo "  FAIL  $name (wanted: $want)"; sed 's/^/        /' <<<"$out" | head -8; FAILED=1
  fi
}

test_postgres() {
  echo "== postgresql ($PG_IMAGE)"
  run_bg pg "$PG_IMAGE" -e POSTGRES_PASSWORD=secret "$PG_IMAGE" || return 1
  wait_for "postgres" docker exec pg pg_isready -U postgres || return 1
  start_proxy proxy-pg DATABASE_TYPE=postgresql STATIC_BACKENDS=e2e=pg:5432,e2e.pool=pg:5432 || return 1
  local psql=(client "$PG_IMAGE" psql -tA -c)
  local base="host=proxy-pg user=postgres.e2e dbname=postgres"
  # PGPASSWORD must reach the client container
  psql() { docker run --rm --network "$NET" -e PGPASSWORD=secret "$PG_IMAGE" psql -tA "$@"; }
  check "login routes and user suffix is stripped (TLS)" "postgres" psql "$base sslmode=require" -c "select current_user"
  check "plaintext connection" "postgres" psql "$base sslmode=disable" -c "select current_user"
  check "pooled route (.pool)" "postgres" psql "host=proxy-pg user=postgres.e2e.pool dbname=postgres sslmode=require" -c "select current_user"
  check "database defaults to postgres when equal to login" "postgres" psql "host=proxy-pg user=postgres.e2e sslmode=disable" -c "select current_database()"
  # \conninfo: psql 17 prints "SSL connection (protocol: ...)", psql 18 prints a table row "SSL Connection|true"
  tls_in_use() { psql "$base sslmode=require" -c '\conninfo' | grep -iE 'ssl connection( \(|\|true)'; }
  check "TLS is really negotiated with the client" "onnection" tls_in_use
  check "direct TLS (sslnegotiation=direct)" "postgres" psql "$base sslmode=require sslnegotiation=direct" -c "select current_user"
  check_fails "unknown deployment is rejected" "no backend for this user" psql "host=proxy-pg user=postgres.nope dbname=postgres sslmode=disable" -c "select 1"
  check_fails "login without deployment is rejected" "no backend for this user" psql "host=proxy-pg user=postgres dbname=postgres sslmode=disable" -c "select 1"
  check "large result passes through" "100000" psql "$base sslmode=require" -c "select count(*) from generate_series(1,100000)"
}

test_redis() {
  echo "== redis ($REDIS_IMAGE)"
  run_bg redis "$REDIS_IMAGE" "$REDIS_IMAGE" redis-server --requirepass secret || return 1
  wait_for "redis" docker exec redis redis-cli -a secret --no-auth-warning ping || return 1
  start_proxy proxy-redis DATABASE_TYPE=redis STATIC_BACKENDS=cache1=redis:6379,cache1.pool=redis:6379 PROXY_START_PORT=6379 || return 1
  local rc=(client "$REDIS_IMAGE" redis-cli -h proxy-redis --no-auth-warning --user default.cache1 --pass secret)
  check "AUTH user.deployment is routed and rewritten" "default" "${rc[@]}" acl whoami
  check "SET through proxy" "OK" "${rc[@]}" set e2e-key e2e-value
  check "GET through proxy" "e2e-value" "${rc[@]}" get e2e-key
  check "pooled route" "default" client "$REDIS_IMAGE" redis-cli -h proxy-redis --no-auth-warning --user default.cache1.pool --pass secret acl whoami
  check "TLS to the proxy (proxy terminates)" "default" client "$REDIS_IMAGE" redis-cli -h proxy-redis --tls --insecure --no-auth-warning --user default.cache1 --pass secret acl whoami
  check "HELLO 3 AUTH (RESP3 clients)" "default" client "$REDIS_IMAGE" redis-cli -h proxy-redis -3 --no-auth-warning --user default.cache1 --pass secret acl whoami
  check_fails "unknown deployment is rejected" "no backend for this user" client "$REDIS_IMAGE" redis-cli -h proxy-redis --no-auth-warning --user default.nope --pass secret ping
  check "pipelined commands after AUTH" "2" client "$REDIS_IMAGE" sh -c "printf 'incr e2e-n\nincr e2e-n\n' | redis-cli -h proxy-redis --no-auth-warning --user default.cache1 --pass secret | tail -1"
}

test_mongodb() {
  echo "== mongodb ($MONGO_IMAGE)"
  # mongod with a throw-away certificate made on the host (the image may not ship openssl);
  # the proxy only passes TLS through, routed by host name (SNI)
  openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=mongo \
    -keyout "$WORK/mongo.key" -out "$WORK/mongo.crt" >/dev/null 2>&1 || { echo "openssl failed"; return 1; }
  cat "$WORK/mongo.key" "$WORK/mongo.crt" > "$WORK/mongo.pem" && chmod 644 "$WORK/mongo.pem" "$WORK/mongo.crt"
  # mongod 8 refuses TLS without a chain of trust: the self-signed certificate is its own CA,
  # and clients do not have to present a certificate.
  run_bg mongo "$MONGO_IMAGE" -v "$WORK/mongo.pem:/tls/mongo.pem:ro" -v "$WORK/mongo.crt:/tls/ca.pem:ro" "$MONGO_IMAGE" \
    mongod --tlsMode requireTLS --tlsCertificateKeyFile /tls/mongo.pem --tlsCAFile /tls/ca.pem \
    --tlsAllowConnectionsWithoutCertificates --bind_ip_all || return 1
  wait_for "mongod" mongo docker exec mongo mongosh --quiet --tls --tlsAllowInvalidCertificates --tlsAllowInvalidHostnames \
    --eval "db.runCommand({ping:1}).ok" || return 1
  ALIASES=(--network-alias m1.mongo.example.com --network-alias m1.pool.mongo.example.com --network-alias nope.mongo.example.com)
  start_proxy proxy-mongo DATABASE_TYPE=mongodb STATIC_BACKENDS=m1=mongo:27017,m1.pool=mongo:27017 PROXY_START_PORT=27017 || return 1
  ALIASES=()
  local ms=(client "$MONGO_IMAGE" mongosh --quiet --tls --tlsAllowInvalidCertificates --tlsAllowInvalidHostnames --eval)
  check "ping through SNI route" "1" "${ms[@]}" "db.runCommand({ping:1}).ok" --host m1.mongo.example.com
  check "pooled route (.pool host)" "1" "${ms[@]}" "db.runCommand({ping:1}).ok" --host m1.pool.mongo.example.com
  check "insert and read back" "e2e" "${ms[@]}" "db.t.insertOne({v:'e2e'}); db.t.findOne().v" --host m1.mongo.example.com
  check_fails "unknown deployment is refused" "Mongo" "${ms[@]}" "db.runCommand({ping:1}).ok" --host nope.mongo.example.com --serverSelectionTimeoutMS 2000
}

test_clickhouse() {
  echo "== clickhouse ($CH_IMAGE)"
  run_bg ch "$CH_IMAGE" -e CLICKHOUSE_PASSWORD=secret --ulimit nofile=262144:262144 "$CH_IMAGE" || return 1
  wait_for "clickhouse" docker exec ch clickhouse-client --password secret -q "select 1" || return 1
  start_proxy proxy-ch DATABASE_TYPE=clickhouse STATIC_BACKENDS=ch1=ch:9000,ch1.pool=ch:9000 PROXY_START_PORT=9000 || return 1
  local ch=(client "$CH_IMAGE" clickhouse-client --host proxy-ch --password secret)
  check "Hello user.deployment is routed and rewritten" "default" "${ch[@]}" --user default.ch1 -q "select currentUser()"
  check "query result" "42" "${ch[@]}" --user default.ch1 -q "select 6*7"
  check "pooled route" "default" "${ch[@]}" --user default.ch1.pool -q "select currentUser()"
  check "large result passes through" "1000000" "${ch[@]}" --user default.ch1 -q "select count() from numbers(1000000)"
  check_fails "unknown deployment is rejected" "Connection reset" "${ch[@]}" --user default.nope -q "select 1"
}

for t in $ONLY; do
  case $t in
    postgres|postgresql) test_postgres || FAILED=1 ;;
    redis) test_redis || FAILED=1 ;;
    clickhouse) test_clickhouse || FAILED=1 ;;
    mongodb|mongo) test_mongodb || FAILED=1 ;;
    *) echo "unknown test set: $t"; FAILED=1 ;;
  esac
done

if [ "$FAILED" -eq 0 ]; then echo "all e2e checks passed"; else echo "e2e FAILED"; fi
exit "$FAILED"
