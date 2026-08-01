# Web UI

`wataridori serve` embeds and serves this React application. The browser calls
`DeploymentService` through the generated Connect client.

## Views

- **Pipeline:** services as rows and environments in promotion order as columns
- **Timeline:** Cloud Run revisions, independent of who deployed them
- **Inventory:** managed and unmanaged Cloud Run services
- **Activity:** operations recorded by Wataridori
- **Service drawer:** digest, revision, traffic, readiness, Console link, and
  rollback plan
- **Promotion dialog:** digest and registry-copy plan before execution

Timeline and Activity have different sources. Timeline reads Cloud Run;
Activity reads Wataridori's history store.

## Structure

- `src/gen`: generated protobuf messages and service descriptors; do not edit
- `src/api`: Connect transport and client
- `src/views`: page-level views
- `src/components`: reusable dialogs, drawer, status, and toast components
- `src/lib`: pure presentation and board-model helpers
- `src/theme.css`: visual tokens and component styles
- `e2e`: Playwright tests and the Go fake backend
- `dist`: committed Vite build embedded by Go

## Build and test

```sh
npm --prefix web ci
npm --prefix web run typecheck
npm --prefix web run build
npm --prefix web run test:e2e
```

The Playwright fake backend uses the real Go Connect handler and protobuf
serialization. It does not require GCP credentials.

Screenshot tests regenerate `docs/screenshots/*.png`. Review those binary
changes before committing.

## Develop

```sh
go run ./cmd/wataridori serve --repo /path/to/manifests
npm --prefix web run dev
```

Vite listens on port 5173 and proxies RPC requests to port 8080.

## Stack

React, Vite, TanStack Query, Connect for ECMAScript, and Protobuf-ES.
