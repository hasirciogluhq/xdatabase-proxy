---
title: Log messages
parent: Reference
nav_order: 2
---

# Log messages

Value of the `msg` field, grouped by area. DEBUG messages appear only with `DEBUG=true`.

## Lifecycle

| Level | msg | Meaning |
| --- | --- | --- |
| INFO | starting xdatabase-proxy | Fields `version`, `discovery`, `tls` |
| INFO | listening | Listener open, `/ready` is 200. Field `port` |
| INFO | stopped | Shutdown finished |
| WARN | TLS is disabled, client connections will not be encrypted | `TLS_ENABLED=false` for postgresql, redis or clickhouse |
| ERROR | health server | The health port could not be served (for example already in use) |
| ERROR | fatal | Startup failed; the process exits 1. Field `error` |

Configuration errors are printed as `configuration error: …` on standard error before logging starts.

## Connections

| Level | msg | Meaning |
| --- | --- | --- |
| DEBUG | session started | Backend dialled, bytes flowing. Fields `via`, `deployment`, `pooled`, `backend` |
| DEBUG | session started (tls terminated) | Same, for an SNI route with `SNI_TLS_MODE=terminate` |
| WARN | resolve failed | No route, route not allowed, or bad destination port. Field `error` |
| ERROR | dial failed | Backend unreachable within `DIAL_TIMEOUT_SECONDS`. Fields `backend`, `error` |
| DEBUG | startup failed | PostgreSQL startup malformed, too slow, CancelRequest, direct TLS without TLS, too many negotiation packets |
| DEBUG | login read failed | Redis or ClickHouse first message not routable |
| DEBUG | tls handshake failed | Client TLS handshake with the proxy failed (often an untrusted certificate) |
| DEBUG | sni route unavailable | Server name has no usable route; field `host`. The proxy then tries username routing or closes |
| DEBUG | plaintext connection but protocol is routed by SNI only | Plaintext client on mongodb or tls |
| ERROR | forward startup failed / forward login failed / forward client hello failed | Backend closed while the first message was being sent |

## Server

| Level | msg | Meaning |
| --- | --- | --- |
| WARN | connection limit reached, rejecting | `MAX_CONNECTIONS` reached. Field `remote` |
| WARN | accept failed, retrying | Temporary accept error, for example out of file descriptors. Field `delay` |
| WARN | shutdown timeout, closing remaining connections | Sessions still open after 25 s of drain |

## Discovery and TLS

| Level | msg | Meaning |
| --- | --- | --- |
| WARN | several services match one route, using the first by namespace/name; … | Ambiguous route; fields `chosen`, `candidates`. Logged once per candidate set, on standard error in text form. Main after 2.1.2 |
| INFO | generating self-signed certificate | Field `renewal` is true when an expiring own certificate is replaced |
| WARN | storing certificate failed, reloading | Another replica won the race, or the file or Secret is not writable |
| WARN | certificate refresh failed | The 6 hourly reload failed; the current certificate stays in use |
