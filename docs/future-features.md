# Future features

Wataridori borrows selected operational ideas from systems such as PipeCD while
remaining a small Cloud Run delivery tool.

## Selection criteria

Every feature should:

- preserve Git as the desired-state source
- preserve digest-based promotion
- remain specific to Cloud Run
- fit the single-binary distribution model
- avoid building images
- link to Cloud Console instead of rebuilding log and metric products

## Implemented

- structured apply, promotion, and rollback plans
- embedded environment pipeline
- Cloud Run inventory
- drift, readiness, revision, and traffic visibility
- Cloud Console deep links
- Cloud Run revision timeline
- Wataridori activity history

## v0.1 candidates

### PR-based promotion

```sh
wataridori promote --to prod --pr
```

The command should:

1. create a promotion branch
2. update the target manifest
3. commit and push the change
4. create a GitHub pull request
5. include the structured plan in the PR body

The existing local-commit behavior remains the default.

### Reproducible real-GCP acceptance environment

The release checklist must cover:

1. deploy development
2. update the development digest
3. promote to production
4. copy between registries when configured
5. apply production
6. verify the endpoint
7. rollback
8. confirm drift and history
9. clean up resources

## v1.0

### Remote Git reconciliation

The hosted controller needs an authenticated checkout, branch selection,
clone/fetch, safe fast-forward behavior, webhook verification, and recovery
from dirty or locally-ahead state.

### Authentication and authorization

IAP or OIDC should establish the request principal. Authorization should
distinguish read, mutation, and production operations.

### Shared audit storage

Hosted activity and approval records must survive Cloud Run instance
replacement and be shared by every instance. SQLite remains useful locally.

### Approval gates

Approval applies to a fingerprinted plan. Any changed digest, service, or
target environment invalidates the prior approval.

### Notifications

Start with a stable generic webhook event model, then add Slack presentation.
Delivery retries must be bounded and must not change the deployment result.

### Import existing services

Inventory may generate a reviewable manifest for an unmanaged service.
Unsupported Cloud Run fields must be reported, and adoption must produce a Git
commit or PR before Wataridori manages the service.

### Managed image update

Users may choose an Artifact Registry digest for a managed service, but the
standard path updates Git first. Direct Cloud Run mutation belongs only in an
explicit break-glass workflow.

## After v1.0

### Progressive rollout

```yaml
rollout:
  steps:
    - percent: 10
      wait: 10m
    - percent: 50
      wait: 10m
    - percent: 100
```

The first version may require manual step advancement and abort.

### Automatic rollback

Possible signals:

- revision readiness timeout
- HTTP smoke-test failure
- Cloud Monitoring error-rate threshold

Automatic decisions require durable state, auditability, notifications, and a
manual override.

### Event-driven development updates

Artifact Registry or CI events may update a development manifest by digest.
Wataridori still does not build the image, and production remains gated by
promotion and approval.

### Multi-stage promotion chains

The current `promoteFrom` model already supports a simple
`dev -> staging -> prod` chain. Future work should avoid turning that chain into
an unnecessarily general workflow engine.

## Explicitly excluded through v1.0

- runtimes other than Cloud Run
- CI image builds
- a full logging or monitoring UI
- a multi-tenant control plane
- a plugin system
