---
title: Redis
parent: Databases
nav_order: 2
---

# Redis

`DATABASE_TYPE=redis`. Usually with `PROXY_START_PORT=6379`.

Works with Redis 6 or newer (ACL users) and should work with protocol-compatible servers such as Valkey, KeyDB and Dragonfly (not tested). Default routing: username and sni.

## Connecting by username

The first command must carry the credentials, with the deployment in the user name:

```
AUTH default.cache1 secret
HELLO 3 AUTH default.cache1.pool secret
```

```bash
redis-cli -h proxy.example.com --user default.cache1 --pass secret
redis-cli -h proxy.example.com -3 --user default.cache1 --pass secret          # RESP3, uses HELLO 3 AUTH
redis-cli -h proxy.example.com --tls --insecure --user default.cache1 --pass secret
```

```
redis://default.cache1:secret@proxy.example.com:6379/0
rediss://default.cache1:secret@proxy.example.com:6379/0
```

The proxy replaces `default.cache1` with `default` and forwards the command, then passes everything through, including pipelined commands sent right after AUTH. With a server that only has `requirepass`, use the user `default`.

Most client libraries (redis-py, go-redis, ioredis, node-redis, Lettuce, Jedis, StackExchange.Redis) send AUTH or HELLO AUTH first when you set both user name and password. Check that your client does not send `PING`, `CLIENT SETNAME` or `SELECT` before authenticating.

### Rejected first commands

The proxy answers `-ERR unsupported first message: …` and disconnects when the first command is:

- `AUTH <password>` without a user name;
- `HELLO` without `AUTH`;
- any other command (`PING`, `SELECT`, `CLIENT …`);
- an inline command (plain text instead of a RESP array);
- larger than 32 arguments or 4096 bytes in total.

Use SNI routing for clients that cannot send a user name.

## Connecting by server name

```bash
redis-cli -h cache1.redis.example.com --tls --sni cache1.redis.example.com --pass secret
```

With TLS and a server name that has a route, the connection is routed by host name before any command is read. The login is not inspected or rewritten, so password-only AUTH, ACL users without suffix and any first command work.

- `SNI_TLS_MODE=passthrough` (default): the encrypted stream goes to the backend, which must run TLS (`tls-port`). The client sees the backend's certificate.
- `SNI_TLS_MODE=terminate`: the proxy completes TLS with its own certificate and talks plain TCP to the backend.

If the server name has no route, or the Service forbids sni, the proxy falls back to username routing: it terminates TLS itself (when it has a certificate) and reads AUTH.

## TLS for username routing

Username routing always terminates TLS at the proxy, because the user is inside the encrypted stream. The proxy uses its certificate (auto self-signed by default, so clients need `--insecure` or your own certificate). Plaintext connections are routed by username too. With `TLS_ENABLED=false`, TLS clients can only use SNI passthrough routes.

## Errors

| Situation | Client sees |
| --- | --- |
| Unroutable first command | `-ERR unsupported first message: …` |
| No deployment in the user, unknown deployment, route not allowed | `-ERR no backend for this user; expected user.deployment[.pool]` |
| Backend unreachable | `-ERR backend unavailable` |
| SNI route but backend unreachable | connection closed |

## Limits

- Redis Cluster and Sentinel are not supported through the proxy: they return internal node addresses in `MOVED`, `ASK` and `SENTINEL get-master-addr-by-name` replies, which clients would then connect to directly. Route to a standalone primary, or to a cluster-aware proxy (for example Envoy or a managed endpoint) as the backend.
- `AUTH` sent later in the session is passed through unchanged, so re-authentication must use the plain user name.
