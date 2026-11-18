---
title: Operations
nav_order: 11
---

# Operations

## Health endpoints

| Path | Meaning |
| --- | --- |
| /health | The process is alive |
| /ready | The proxy finished startup and accepts connections. Returns 503 during shutdown |

The image also has a built-in check: `/xdatabase-proxy -healthcheck` calls /health and exits 0 or 1. It is the Docker HEALTHCHECK.

## Logs

JSON on standard output through slog. Fields include remote, via (username or sni), deployment, pooled, backend and error. Set DEBUG=true to log session starts, startup failures, failed TLS handshakes and unusable SNI routes. Startup parameters are never logged.

## Upgrades

- Deployment: rolling update with maxUnavailable 0. Existing sessions continue on old pods until their grace period ends (25 seconds), then are closed. Clients reconnect.
- DaemonSet: one node at a time.
- A new version only needs the new image. Configuration is environment only.

## Sizing

A session costs two sockets and two small copy goroutines. The manifests request 100m CPU and 64Mi memory and limit memory to 512Mi. Raise MAX_CONNECTIONS and the file descriptor limit together for very large fleets.

## Troubleshooting

| Symptom | Likely cause | Check |
| --- | --- | --- |
| no backend for this user | Login has no deployment part, no matching Service or table entry, or the routing label does not allow the strategy | Log line resolve failed, labels on the Service, login format |
| Wrong backend reached | Several Services share a route (often a pooler without xdatabase-proxy-pooled) | Warning several services match one route lists the candidates |
| resolve failed: matches no port | xdatabase-proxy-destination-port names a port the Service does not have | Fix the label or the Service ports |
| SNI client closed at once | No route for the host name, route does not allow sni, or client sent no SNI (connected by IP) | DEBUG=true, log line sni route unavailable |
| backend unavailable | Service DNS fails or backend down | Log line dial failed shows the address. Test it from the proxy pod |
| Pod not ready | Cannot reach the API server, or TLS Secret cannot be created | Pod log. RBAC for services and the Secret |
| sslmode=require refused | TLS_ENABLED=false | Configuration |
| verify-full fails | Self-signed certificate | Provide your own, see TLS |
| Service added but not routable | Missing enabled or database-type label | Labels must match exactly |
| Services outside one namespace ignored | WATCH_NAMESPACE is set | Unset it and use the ClusterRole |
| Connections dropped at start-up | MAX_CONNECTIONS reached | Log line connection limit reached |
| Cancel does nothing | CancelRequest is not routed | See Connection routing |
