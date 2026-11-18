---
title: Service labels
parent: Reference
nav_order: 1
---

# Service labels

Labels on Kubernetes Services read by the proxy. The informer only lists Services with `xdatabase-proxy-enabled=true` and `xdatabase-proxy-database-type=<DATABASE_TYPE>`.

| Label | Values | Default | Effect |
| --- | --- | --- | --- |
| xdatabase-proxy-enabled | `"true"` | – | Required. Opts the Service in |
| xdatabase-proxy-database-type | `postgresql`, `redis`, `clickhouse`, `mongodb`, `tls` | – | Required. Must equal the proxy's `DATABASE_TYPE` (PostgreSQL-compatible products use `postgresql`) |
| xdatabase-proxy-deployment-id | any label value | – | Required. Route key. Lower case for SNI routing |
| xdatabase-proxy-pooled | `"true"` | direct | `"true"` makes it the pooled endpoint; any other value is direct |
| xdatabase-proxy-destination-port | port number or port name | first port | Which Service port to dial. No match is an error (main after 2.1.2; 2.1.2 falls back to the first port) |
| xdatabase-proxy-routing | `username`, `sni`, `all` | per type: postgresql `username`; redis, clickhouse both; mongodb, tls `sni` | Strategies that may reach this Service. Unknown values use the default |

Complete example:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: cache1
  namespace: data
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-database-type: redis
    xdatabase-proxy-deployment-id: cache1
    xdatabase-proxy-pooled: "false"
    xdatabase-proxy-destination-port: redis
    xdatabase-proxy-routing: all
spec:
  selector: { app: cache1 }
  ports:
    - { name: redis, port: 6379 }
```
