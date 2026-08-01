# Contributing to Wataridori

Thank you for helping improve Wataridori.

Wataridori is intentionally focused on digest-based delivery for Google Cloud
Run. Before starting a large change, read:

- [requirements](docs/requirements.md)
- [architecture](docs/architecture.md)
- [roadmap](docs/roadmap.md)

Open an issue before implementing a new product capability. This avoids work
that conflicts with the project's scope or an existing design.

## Security reports

Do not open a public issue for a suspected vulnerability, exposed credential,
authorization bypass, or unsafe deployment behavior. Follow
[SECURITY.md](SECURITY.md).

## Development setup

Requirements:

- Go 1.25 or later
- Node.js 20 or later
- npm
- Buf
- Chromium for Playwright tests

Install tools and dependencies:

```sh
make tools
npx playwright install chromium
```

Run the standard checks:

```sh
go test ./...
npm --prefix web run typecheck
npm --prefix web run build
npm --prefix web run test:e2e
make gen-check
```

Run the linter when available:

```sh
make lint
```

## Generated files

Generated protobuf code is committed:

- `gen/`
- `web/src/gen/`

After changing a `.proto` file:

```sh
make gen
```

The built Web UI in `web/dist/` is also committed because it is embedded in the
Go binary. After changing Web source:

```sh
make web-build
```

Screenshot tests update `docs/screenshots/*.png`. Review image changes before
including them in a pull request.

## Pull requests

Keep pull requests focused and explain:

- what changed
- why the change is needed
- user and operator impact
- tests performed
- documentation or migration impact

Use protobuf messages and generated clients for API changes. Do not introduce a
second handwritten API contract.

Do not commit credentials, access tokens, generated cloud credentials,
Terraform state, local databases, or private manifest repositories.

## Design expectations

- Deploy image digests, never mutable tags.
- Preserve Git as desired state and Cloud Run as actual state.
- Keep database state auxiliary.
- Keep Cloud Run as the only supported runtime.
- Do not add container image builds to Wataridori.
- Prefer Cloud Console deep links over rebuilding logging and monitoring.

## License

By submitting a contribution, you agree that it is licensed under the Apache
License 2.0 as described in [LICENSE](LICENSE).
