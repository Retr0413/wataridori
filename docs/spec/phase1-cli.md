# Phase 1 specification: CLI MVP

This document defines the Phase 1 manifest and CLI contract. Later server and
UI features reuse these core operations.

## Operating model

- The user runs Wataridori from a manifest Git checkout.
- Git manifests are desired state.
- Cloud Run Admin API v2 is actual state.
- ADC provides GCP credentials.
- Promotion creates a local Git commit.
- SQLite records local auxiliary history.

```text
manifest-repository/
├── wataridori.yaml
└── environments/
    ├── dev/
    │   └── my-app.yaml
    └── prod/
        └── my-app.yaml
```

## 1. Manifest schema

### 1.1 Repository configuration

```yaml
version: 1
environments:
  dev:
    policy: auto
    branch: develop
    gcp:
      project: my-app-dev
      region: asia-northeast1
    services: environments/dev
  prod:
    policy: manual
    promoteFrom: dev
    gcp:
      project: my-app-prod
      region: asia-northeast1
    services: environments/prod
    imageCopy:
      to: asia-northeast1-docker.pkg.dev/my-app-prod/images
```

Validation:

- `version` must be `1`
- `policy` is `auto` or `manual`
- an automatic environment declares `branch`
- `services` stays inside the repository root
- an all-automatic configuration emits a warning
- `promoteFrom` must name another environment
- promotion relationships must not contain cycles

### 1.2 Service manifest

```yaml
name: my-app
cloudRunName: my-app-prod
image: asia-northeast1-docker.pkg.dev/my-app-prod/images/my-app@sha256:...
env:
  - name: LOG_LEVEL
    value: info
  - name: JWT_SECRET
    secret: my-app-jwt-secret
    version: latest
resources:
  cpu: "1"
  memory: 512Mi
scaling:
  min: 0
  max: 10
serviceAccount: my-app@my-app-prod.iam.gserviceaccount.com
concurrency: 80
port: 8080
```

`name` is the cross-environment logical identity. `cloudRunName` is the
physical Cloud Run service name and defaults to `name`.

Rules:

- `name` is required and unique within the environment
- `image` is required and must contain a valid digest
- mutable tag-only references are rejected
- environment entries use exactly one of `value` or `secret`
- ports, concurrency, scaling, CPU, and memory must be valid
- service paths may not escape the configured service directory

Apply is declarative for supported fields. Environment variables omitted from
the manifest are removed. Selected unsupported or platform-managed Cloud Run
fields are preserved to avoid destructive updates.

## 2. Commands

Every command supports:

- `--repo`: manifest repository root
- `--json`: structured output
- `--db`: history database path

### 2.1 Apply

```sh
wataridori apply --env dev
wataridori apply --env prod --service my-app
wataridori apply --env prod --dry-run
```

Algorithm:

1. load and validate repository configuration
2. select the environment and optional service
3. read the current Cloud Run service
4. calculate create, update, or no-op
5. stop here for dry run
6. update Cloud Run using the digest-pinned image
7. wait for a ready revision until timeout
8. record one history entry per service

The default readiness timeout is five minutes and may be overridden.

### 2.2 Promote

```sh
wataridori promote --to prod
wataridori promote --from dev --to prod --service my-app
wataridori promote --to prod --yes
```

Algorithm:

1. resolve source from `--from` or target `promoteFrom`
2. match services by logical name
3. read source and target digest references
4. plan target changes and any registry copy
5. request confirmation unless `--yes`
6. copy source content to the target repository when configured
7. update only the target manifest image
8. create one Git commit for all changed manifests
9. record promotion history

Promotion rejects:

- an automatic target environment
- the same source and target
- a missing source service or digest
- a dirty tracked target manifest
- unrelated tracked changes that would make the commit unsafe

No-op promotion does not create a commit.

Phase 1 does not push or create a PR. The caller pushes the commit. PR-based
promotion is planned for `v0.1.0`.

### 2.3 Rollback

```sh
wataridori rollback --env prod
wataridori rollback --env prod --service my-app
wataridori rollback --env prod --service my-app --revision my-app-00042
```

Algorithm:

1. list revisions for selected services
2. identify the current serving revision
3. select the previous ready revision, or validate the explicit revision
4. display current and target image/revision
5. request confirmation
6. route 100% of service traffic to the target revision
7. record rollback history

An explicit revision requires a single service. A failed or non-ready revision
is never selected automatically.

Rollback does not edit Git. Status may therefore report drift afterward.

### 2.4 Status

```sh
wataridori status
wataridori status --env prod
wataridori status --check
```

For every manifest service:

- read the serving Cloud Run revision
- normalize image references
- compare desired and actual digest
- include revision, traffic, readiness, URLs, and state

States:

- `in sync`
- `drift`
- `not deployed`

`--check` exits 0 when all services are in sync and 2 when drift or a missing
deployment exists.

### 2.5 History

```sh
wataridori history
wataridori history --env prod --limit 20
```

Entries are newest first and include:

- timestamp
- actor
- action
- environment
- service
- digest
- structured detail

### 2.6 Inventory

```sh
wataridori inventory list
wataridori inventory list --env prod
```

Inventory lists configured Cloud Run locations and classifies managed,
unmanaged, missing, in-sync, and drifted services. It is read-only.

### 2.7 Serve

```sh
wataridori serve \
  --repo /path/to/manifests \
  --addr 127.0.0.1:8080
```

Serve exposes the Connect API and embedded Web UI. `--reconcile` enables the
controller against the current local checkout.

The current server has no application-level authentication. It must not be
exposed directly to an untrusted network.

## 3. History schema

The SQLite schema is private implementation detail, but its logical record is:

```text
id, timestamp, actor, action, environment, service, digest, detail
```

Opening the store is idempotent. The default path can be overridden with
`--db` or `WATARIDORI_DB`.

SQLite is local process history, not a durable hosted audit database.

## 4. Acceptance criteria

Phase 1 is accepted when:

1. valid and invalid manifests are covered by tests
2. apply creates and updates a digest-pinned Cloud Run service
3. promotion copies the exact digest and creates the expected commit
4. split-registry promotion preserves the target path and digest
5. rollback restores a previous ready revision
6. status detects in-sync, drifted, and missing services
7. history records every successful mutation
8. JSON output is machine readable
9. confirmation and dry-run paths perform no mutation
10. the end-to-end procedure in
    [cloudrun-cli-verification.md](../cloudrun-cli-verification.md) succeeds
    against a release candidate
