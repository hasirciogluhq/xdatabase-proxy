# Security Policy

## Supported versions

Security fixes go to the latest released minor version. Please upgrade before reporting.

## Reporting a vulnerability

Do not open a public issue. Use GitHub's private reporting:
**Security → Report a vulnerability** on https://github.com/hasirciogluhq/xdatabase-proxy/security/advisories/new

Include affected version, configuration, steps to reproduce and impact. You will get an acknowledgement within a few days. Fixes are released as soon as they are ready and credited in the changelog unless you prefer otherwise.

## Scope

In scope: the proxy binary, container image, Kubernetes manifests and CI workflows in this repository.
Out of scope: vulnerabilities in PostgreSQL itself, in your database deployments, or in a self-signed certificate being unverifiable by design.

## Hardening notes

See [docs/security.md](docs/security.md) for the threat model, built-in protections and what you must handle yourself (for example the unencrypted proxy-to-backend hop).
