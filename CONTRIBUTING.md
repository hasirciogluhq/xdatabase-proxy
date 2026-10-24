# Contributing

Thanks for helping. Please read the [Code of Conduct](CODE_OF_CONDUCT.md) first.

## Quick start

```bash
git clone https://github.com/hasirciogluhq/xdatabase-proxy
cd xdatabase-proxy
go test -race ./...
scripts/dev-run.sh        # needs a PostgreSQL on localhost:5432
```

The Go version is set in `go.mod`. Project layout and rules are in [AGENTS.md](AGENTS.md) and [docs/development.md](docs/development.md).

## Workflow

1. Open an issue for larger changes so we can agree on the approach.
2. Branch from `development` (`feature/...` or `fix/...`).
3. Keep the change small and simple. Prefer the standard library and well-maintained libraries over hand-written protocol or Kubernetes code.
4. Add or update tests. Run `gofmt -l .`, `go vet ./...` and `go test -race ./...`.
5. Update `docs/`, add an entry under **Unreleased** in `CHANGELOG.md`, and update `AGENTS.md` if layout, commands or variables changed.
6. Open a pull request against `development`. CI must pass.

## Commit messages

Short imperative subject line, for example `Fix startup deadline handling`. Explain why in the body when it is not obvious.

## Reporting bugs and requesting features

Use the issue templates. For security problems do **not** open a public issue, see [SECURITY.md](SECURITY.md).

## License

By contributing you agree that your contributions are licensed under the [MIT License](LICENSE).
