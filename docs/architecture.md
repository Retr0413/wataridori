# Architecture

## Overview

```mermaid
flowchart LR
    CLI["CLI"] --> Core["Core use cases"]
    Web["React Web UI"] --> RPC["Connect RPC server"]
    RPC --> Core
    Controller["Controller"] --> Core

    Core --> Git["Git manifests<br/>desired state"]
    Core --> Run["Cloud Run Admin API<br/>actual state"]
    Core --> Registry["Artifact Registry<br/>digest copy"]
    Core --> Store["History store<br/>auxiliary records"]

    WebAssets["Vite build"] --> Binary["Single Go binary"]
    RPC --> Binary
    Controller --> Binary
    CLI --> Binary
```

Detailed command and controller sequences are documented in
[system-flow.md](system-flow.md). Terms are defined in
[concepts-and-glossary.md](concepts-and-glossary.md).

## Design principles

### One binary

The CLI, Connect RPC server, controller foundation, and embedded Web UI ship in
one Go binary. Wataridori can therefore run locally or as a Cloud Run service
without operating a separate frontend deployment or control plane.

### Two sources of truth

- **Desired state:** Git manifests
- **Actual state:** Cloud Run Admin API

The history database is auxiliary. It must never become the authoritative
source for the current image, revision, or traffic allocation.

### Immutable promotion

An environment manifest references an image with `@sha256:...`. Promotion
copies that digest into the target manifest. If the target uses another
Artifact Registry repository, Wataridori copies the content by digest and keeps
the target repository path.

### Plan before mutation

Apply, promotion, and rollback calculate structured plans. CLI confirmation and
the Web UI display those plans before calling the execution operation.

## Components

| Component | Responsibility |
|---|---|
| `cmd/wataridori` | Process entrypoint |
| `internal/cli` | Cobra commands, flags, confirmation, and rendering |
| `internal/core` | Apply, promotion, rollback, status, inventory, history, and timeline use cases |
| `internal/manifest` | YAML types, loading, validation, and digest updates |
| `internal/cloudrun` | Cloud Run Admin API v2 wrapper |
| `internal/registry` | Digest-based registry copy |
| `internal/gitops` | Commits and remote Git synchronization primitives |
| `internal/store` | Local SQLite operation history |
| `internal/server` | Connect RPC handlers and protobuf conversion |
| `internal/controller` | Periodic and triggered reconciliation |
| `proto` | API contract and generated-code input |
| `web` | React UI, generated client, and embedded build output |

Dependencies point toward `internal/core`; UI and CLI rendering do not belong in
the use-case layer.

## API

[`proto/wataridori/v1/wataridori.proto`](../proto/wataridori/v1/wataridori.proto)
is the API's single source of truth. Buf generates:

- Go protobuf and Connect handlers in `gen/`
- TypeScript protobuf and service descriptors in `web/src/gen/`

The browser uses the Connect protocol. JSON and binary protobuf are transport
choices; the protobuf schema defines the contract in both cases.

## Cloud Run integration

Wataridori uses `cloud.google.com/go/run/apiv2`.

- Updating a service creates a Cloud Run revision.
- Status compares the serving revision's digest with Git.
- Rollback changes traffic to a previous ready revision.
- Timeline reads revision history from Cloud Run, independently from the local
  Wataridori activity database.

Full log and metric rendering is intentionally delegated to Google Cloud
Console deep links.

## Authentication boundaries

GCP access uses Application Default Credentials locally and should use a
dedicated service account through Workload Identity on Cloud Run.

Application-level Web and RPC authentication is not implemented yet. The
planned hosted model is IAP or OIDC plus request-level authorization and a
human principal recorded in the audit history.

## Hosted-state limitation

SQLite works for a local single-process workflow. A Cloud Run filesystem is
ephemeral, and multiple instances do not share one SQLite file. Hosted
multi-user operation therefore requires a durable shared history and approval
store before v1.0.

## Technology

| Layer | Technology |
|---|---|
| Backend and CLI | Go, Cobra |
| Cloud Run | Cloud Run Admin API v2 |
| Registry | go-containerregistry |
| Git | go-git |
| API | Connect RPC and protobuf |
| Web | TypeScript, React, Vite, TanStack Query |
| Local history | SQLite |
| Build and release | GitHub Actions, Buf, GoReleaser, ko |

## Distribution

GoReleaser builds Linux and macOS binaries for amd64 and arm64, checksums, and
a distroless GHCR image. No stable release has been published yet.
