# Requirements

This document defines Wataridori's product scope. Non-goals are requirements:
features listed as out of scope must not be added implicitly.

## Priority summary

| Priority | Capability |
|---|---|
| MVP | Environment policies, digest promotion, rollback, status, history, CLI |
| v1.0 | Operational Web UI, detailed service state, authentication, approvals, notifications |
| Future | Progressive delivery and metric-driven automatic rollback |
| Out of scope | Image builds, non-Cloud Run runtimes, full log and metric views |

## MVP

### Environment update policies

Every environment chooses one update policy:

- `auto`: an eligible Git change may be reconciled automatically
- `manual`: only an explicit promotion or apply may update the environment

Policies belong to environments. If every environment is automatic, there is no
meaningful promotion boundary.

### Digest-based promotion

Promotion copies the source environment's image digest into the target
environment's manifest.

- Tags such as `latest` and `v1.2` are rejected as deployment references.
- The promoted artifact is bit-for-bit identical to the verified source image.
- Promotion creates a Git commit; PR-based promotion is planned.
- If environments use separate Artifact Registry repositories, Wataridori may
  copy the image by digest while preserving the target repository path.

### Rollback

One command or confirmed UI operation routes 100% of traffic to a previous
ready Cloud Run revision. Users may select a revision explicitly for a single
service.

### Current-state comparison

Status shows which digest and revision serve in each environment and compares
that state with Git. Services are classified as in sync, drifted, or not
deployed.

### History

Wataridori records when, who, action, environment, service, digest, and
operation detail for apply, promotion, and rollback.

The MVP implementation uses local SQLite. Shared durable history is required
before a hosted multi-user deployment can provide a complete audit trail.

### CLI

The CLI exposes:

```sh
wataridori apply --env dev
wataridori promote --from dev --to prod
wataridori rollback --env prod
wataridori status
wataridori inventory list
wataridori history --env prod
wataridori serve
```

The CLI currently invokes the core use cases directly. Moving remote CLI
operations through the Connect API is v1.0 work.

## v1.0

### Operational Web UI

The embedded UI must provide:

- environments and services in promotion order
- desired and actual digests
- revision, traffic, readiness, and drift
- promotion and rollback plans before execution
- Cloud Run revision timeline
- Wataridori activity history
- inventory including unmanaged services
- Cloud Console deep links

Most read and execution views are implemented. Authentication, authorization,
approvals, and shared history remain.

### Authentication and authorization

- identify the human or workload principal for every request
- distinguish read access, write access, and production operations
- reject untrusted identity headers
- retain an explicit local-development mode

### Durable audit storage

A hosted deployment must use a shared store that survives Cloud Run instance
replacement and remains consistent across instances. The store remains
auxiliary; Git and Cloud Run remain the sources of deployment truth.

### Approval gates

- approval may be required per environment
- the exact reviewed plan must be fingerprinted
- a changed plan requires new approval
- approver and executor identities are recorded

### Notifications

Slack and generic webhooks cover planned, started, succeeded, failed, rolled
back, drifted, approval-requested, approved, and rejected events.

## Future

### Progressive delivery

- Cloud Run traffic steps such as 10%, 50%, and 100%
- canary and blue-green operation
- revision-tag preview URLs with zero production traffic

### Automatic rollback

HTTP smoke tests or Cloud Monitoring signals may stop a rollout and restore the
previous revision.

## Non-goals

- building container images
- GKE, Compute Engine, ECS, Lambda, or other runtimes
- full Cloud Logging or Cloud Monitoring interfaces
- a complex multi-tenant control plane before v1.0
- a plugin platform
