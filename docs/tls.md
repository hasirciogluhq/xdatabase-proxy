---
title: TLS
nav_order: 9
---

# TLS

TLS is on by default. The proxy terminates client TLS for PostgreSQL and for username routing of Redis and ClickHouse. SNI routes pass TLS through to the backend by default and need no proxy certificate; with `SNI_TLS_MODE=terminate` they use the certificate described here too.

When the proxy terminates TLS, the hop to the backend is plain TCP. Keep it inside a trusted network, or use a service mesh or tunnel for that hop. SNI passthrough keeps TLS end to end.

## Certificate sources

| TLS_MODE | Certificate lives in | Use for |
| --- | --- | --- |
| memory | Process memory, generated at start | Development, single instance. Every restart creates a new certificate |
| file | `TLS_CERT_FILE` and `TLS_KEY_FILE` | VMs, containers, mounted cert-manager or Vault output |
| kubernetes | Secret `TLS_SECRET_NAME` in `NAMESPACE` | Clusters. All replicas share one certificate |

`TLS_MODE` is chosen automatically when unset: file if `TLS_CERT_FILE` is set, kubernetes if `TLS_SECRET_NAME` is set, else memory.

## Self-signed certificates

When no certificate exists and `TLS_AUTO_GENERATE=true`, the proxy creates one:

- ECDSA P-256, valid for one year, random 127-bit serial;
- subject organization `xdatabase-proxy`, DNS name `localhost`;
- usable for server authentication only.

In file mode it writes the files (certificate 0644, key 0600), so the location must be writable. In kubernetes mode it creates the Secret. If several replicas start at once, the first writer wins and the others reload the stored certificate.

Clients that encrypt without verifying (PostgreSQL `sslmode=require`, `redis-cli --insecure`, `clickhouse-client --accept-invalid-certificate`, `tlsAllowInvalidCertificates`) work with it. Clients that verify need your own certificate.

## Your own certificate

Provide the files or the Secret. The proxy never overwrites a certificate it did not create (it checks the organization and that the certificate is self-issued). With `TLS_AUTO_GENERATE=false` a missing or unreadable certificate stops startup.

In file mode with auto generation on, a missing or unreadable pair is replaced by a freshly generated self-signed pair written to the same paths. Set `TLS_AUTO_GENERATE=false` when you provide files, so a mounting mistake fails loudly instead of silently serving a self-signed certificate.

Name the host clients use. For SNI routing that is usually a wildcard, and a pooled route needs a second wildcard:

```
*.db.example.com
*.pool.db.example.com
```

### cert-manager

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: xdatabase-proxy
  namespace: xdatabase-proxy
spec:
  secretName: xdatabase-proxy-tls
  dnsNames:
    - db.example.com
    - "*.db.example.com"
    - "*.pool.db.example.com"
  issuerRef:
    name: letsencrypt
    kind: ClusterIssuer
```

Set `TLS_SECRET_NAME=xdatabase-proxy-tls` and `TLS_AUTO_GENERATE=false`. The proxy then only needs `get` on that Secret, and you can drop `create` and `update` from the Role.

## Renewal and rotation

- The certificate is re-read every 6 hours and swapped in memory. Existing sessions keep their TLS session; new connections use the new certificate. No restart is needed.
- The proxy's own self-signed certificate is re-issued when fewer than `TLS_RENEWAL_THRESHOLD_DAYS` remain (only when auto generation is on).
- Certificates from files or cert-manager are only reloaded, never re-issued.
- A failed reload logs `certificate refresh failed` and keeps the current certificate.

## Protocol settings

- Minimum TLS 1.2, Go default cipher suites and curves.
- One certificate for every server name.
- PostgreSQL direct TLS advertises ALPN `postgresql`, as PostgreSQL 17 requires.
- Client certificates are not requested. Certificate authentication against the backend therefore needs SNI passthrough.

## Disabling TLS

`TLS_ENABLED=false`:

- PostgreSQL answers `N` to every SSLRequest and closes direct TLS connections; `sslmode=require` clients refuse to connect.
- Redis and ClickHouse accept plaintext; TLS clients only work through SNI passthrough routes.
- MongoDB and tls are unaffected in passthrough mode; terminate mode requires TLS.
- The proxy logs `TLS is disabled, client connections will not be encrypted` for postgresql, redis and clickhouse.

Only do this when TLS is handled in front of the proxy.
