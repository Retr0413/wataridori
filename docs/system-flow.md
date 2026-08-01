# System flows

Wataridori reconciles a Git-declared image digest with the revision serving in
Cloud Run.

## Runtime model

```mermaid
flowchart TB
    subgraph Repo["Manifest repository"]
        Config["wataridori.yaml"]
        Dev["environments/dev/*.yaml"]
        Prod["environments/prod/*.yaml"]
    end

    subgraph W["Wataridori binary"]
        CLI["CLI"]
        API["Connect RPC"]
        UI["Embedded Web UI"]
        Ctrl["Controller"]
        Core["Core use cases"]
    end

    UI --> API
    CLI --> Core
    API --> Core
    Ctrl --> Core
    Core --> Config
    Core --> Dev
    Core --> Prod
    Core --> AR["Artifact Registry"]
    Core --> Run["Cloud Run"]
    Core --> DB["History store"]
```

## Status

```mermaid
sequenceDiagram
    actor User
    participant W as Wataridori
    participant Git as Git manifests
    participant Run as Cloud Run API

    User->>W: status
    W->>Git: load desired image
    W->>Run: read serving revision and traffic
    W->>W: compare image digests
    W-->>User: in sync / drift / not deployed
```

Status is read-only. `--check` exits with code 2 when drift exists, making it
suitable for automation.

## Apply

Apply changes Cloud Run, not Git.

```mermaid
sequenceDiagram
    actor User
    participant W as Wataridori
    participant Git as Git manifests
    participant Run as Cloud Run API
    participant Store as History store

    User->>W: apply --env prod
    W->>Git: load and validate desired services
    W->>Run: read current services
    W-->>User: create / update / no-op plan
    User->>W: confirm
    W->>Run: create or update service
    W->>Run: wait for ready revision
    W->>Store: record result
```

`--dry-run` stops after planning. Apply preserves selected Cloud Run fields that
are not controlled by the Wataridori manifest, avoiding accidental deletion of
platform-managed configuration.

## Promotion

Promotion changes Git, not Cloud Run.

```mermaid
sequenceDiagram
    actor User
    participant W as Wataridori
    participant Source as Source manifest
    participant AR as Artifact Registry
    participant Target as Target manifest
    participant Git as Git
    participant Store as History store

    User->>W: promote --from dev --to prod
    W->>Source: read source digest
    W->>Target: read current target digest
    W-->>User: digest change and image-copy plan
    User->>W: confirm
    opt separate target repository
        W->>AR: copy source content by digest
    end
    W->>Target: write target path with source digest
    W->>Git: create commit
    W->>Store: record promotion
```

The user pushes the commit and applies the target environment. PR-based
promotion is planned for `v0.1.0`.

## Rollback

Rollback is an explicit recovery operation against Cloud Run.

```mermaid
sequenceDiagram
    actor User
    participant W as Wataridori
    participant Run as Cloud Run API
    participant Store as History store

    User->>W: rollback --env prod
    W->>Run: list revisions
    W->>W: select a previous ready revision
    W-->>User: current to target plan
    User->>W: confirm
    W->>Run: route 100% traffic to target
    W->>Store: record rollback
```

Rollback may intentionally create drift because Git can still point to the
newer image. Operators must then decide whether to revert Git or re-apply the
desired revision.

## Inventory

Inventory lists Cloud Run services from configured projects and regions, then
joins them with manifests.

```text
Cloud Run + manifest -> managed and in sync
Cloud Run + no manifest -> unmanaged
manifest + no Cloud Run service -> not deployed
both + different digest -> drift
```

Inventory is read-only and does not adopt unmanaged services.

## Timeline and activity

- **Timeline** reads Cloud Run revisions. It shows deployment facts even when
  another tool performed the deployment.
- **Activity** reads Wataridori's history store. It records Wataridori
  operations and actor metadata.

The views are complementary and must not be presented as the same data source.

## Controller

```mermaid
flowchart TD
    Trigger["Startup / interval / trigger"] --> Refresh["Refresh Git checkout"]
    Refresh --> Load["Load manifests"]
    Load --> Select["Select policy:auto environments"]
    Select --> Apply["Apply each environment"]
    Apply --> Wait["Wait for next trigger"]
```

The controller loop and Git synchronization primitives exist, but the hosted
`serve` command does not yet wire a remote repository clone/fetch into each
cycle. Today it reloads a local checkout.

## Web request flow

```mermaid
sequenceDiagram
    actor Browser
    participant UI as Embedded React UI
    participant RPC as Connect handler
    participant Core as Core
    participant GCP as Cloud Run / Registry

    Browser->>UI: load assets
    Browser->>RPC: typed Connect request
    RPC->>Core: use-case request
    Core->>GCP: read or mutate
    GCP-->>Core: state
    Core-->>RPC: structured result
    RPC-->>Browser: protobuf JSON or binary
```

The current server is stateless at the RPC layer: it re-derives execution plans
instead of storing browser sessions. Authentication and hosted authorization
remain v1.0 work.
