---
title: TLS
nav_order: 7
---

# TLS

TLS is on by default. The proxy answers the PostgreSQL SSLRequest and upgrades the connection. The connection from the proxy to the backend is plain TCP, so keep it inside a trusted network or use a mesh or tunnel for that hop.

## Modes

| Mode | Certificate lives in | Use for |
| --- | --- | --- |
| memory | Process memory, created at start | Development. Every restart makes a new certificate |
| file | TLS_CERT_FILE and TLS_KEY_FILE | VMs, containers, mounted cert-manager output |
| kubernetes | Secret TLS_SECRET_NAME in NAMESPACE | Clusters. Replicas share one certificate |

## Self-signed certificates

When no certificate exists and TLS_AUTO_GENERATE is true, the proxy creates an ECDSA P-256 certificate valid for one year, organization xdatabase-proxy, DNS name localhost, with a random serial number. In kubernetes mode it creates the Secret. If several replicas start at once, the first writer wins and the others reload the stored certificate.

Clients using sslmode=require encrypt the traffic but do not verify the server. For sslmode=verify-full supply your own certificate whose names match the host clients use.

## Your own certificate

Provide the files or the Secret. The proxy never overwrites a certificate it did not create. With TLS_AUTO_GENERATE=false a missing certificate stops startup.

With cert-manager, create a Certificate whose secretName equals TLS_SECRET_NAME. The proxy then only needs get on that Secret.

## Renewal and rotation

- The certificate is re-read every 6 hours and swapped without dropping connections. New connections use the new certificate.
- The proxy's own self-signed certificate is re-issued when fewer than TLS_RENEWAL_THRESHOLD_DAYS remain.
- A certificate from file or cert-manager is only reloaded.

## Settings

Minimum protocol is TLS 1.2. Cipher suites follow the Go defaults.

## Disabling TLS

TLS_ENABLED=false makes the proxy answer N to every SSLRequest. Clients with sslmode=require will refuse to connect. Only do this when TLS is handled elsewhere.
