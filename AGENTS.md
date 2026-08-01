# Wataridori

Wataridori is an open-source GitOps continuous delivery tool built specifically
for Google Cloud Run. Its core operation is promoting an Artifact Registry image
from development to production by immutable digest.

## Current phase

The CLI MVP, Connect RPC API, controller foundation, and embedded React Web UI
are implemented. The project is preparing its first public release.

The main remaining work is remote Git synchronization, authentication and
authorization, durable shared audit storage, approval gates, notifications, and
release hardening.

## Required reading

- [docs/requirements.md](docs/requirements.md): authoritative MVP, v1.0, and
  non-goal scope
- [docs/architecture.md](docs/architecture.md): architecture, technology, and
  repository structure
- [docs/roadmap.md](docs/roadmap.md): implementation status and release plan
- [docs/system-flow.md](docs/system-flow.md): runtime and deployment flows

## Design decisions

Update the design documents before changing any of these decisions:

- promotion uses image digests, never mutable tags
- Git is the desired-state source; the Cloud Run API is the actual-state source
- databases store history and approvals, not deployment truth
- server, controller, CLI, and embedded Web UI ship as one Go binary
- `proto/` is the single source of truth for the Connect RPC API
- CI image builds and runtimes other than Cloud Run are out of scope

## Stack

- Backend and CLI: Go, Cobra, Connect RPC, Cloud Run Admin API v2,
  go-containerregistry, go-git, SQLite
- Frontend: TypeScript, React, Vite, TanStack Query, Connect
- License: Apache-2.0
